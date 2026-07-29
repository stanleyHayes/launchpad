package scim

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"launchpad/pkg/httpx"
	"launchpad/pkg/security"
)

const (
	maxBodyBytes    = 1 << 20 // 1 MiB
	filterMaxResult = 200     // also the maximum page size (count)
	defaultPageSize = 100
)

// Handler exposes SCIM 2.0 HTTP endpoints.
type Handler struct {
	svc *Service
}

// NewHandler constructs a SCIM Handler.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// userPayload is a lenient decode target for SCIM User bodies (unknown IdP
// attributes are ignored). Active is a pointer so we can tell "omitted" from
// "false" on create.
type userPayload struct {
	Schemas     []string `json:"schemas"`
	ExternalID  string   `json:"externalId"`
	UserName    string   `json:"userName"`
	Name        *Name    `json:"name"`
	DisplayName string   `json:"displayName"`
	Emails      []Email  `json:"emails"`
	Active      *bool    `json:"active"`
}

// HandleGenerateToken issues a provisioning token for the caller's organization.
// The route middleware gates it on integrations.manage.
func (h *Handler) HandleGenerateToken(w http.ResponseWriter, r *http.Request) {
	principal, ok := security.PrincipalFromContext(r.Context())
	if !ok {
		h.writeError(w, r, http.StatusUnauthorized, "Authentication required")

		return
	}

	token, expiresAt, err := h.svc.GenerateToken(r.Context(), principal.OrganizationID, principal.UserID)
	if err != nil {
		slog.ErrorContext(r.Context(), "generate scim token failed", "error", err)
		h.writeError(w, r, http.StatusInternalServerError, "Unable to generate token")

		return
	}

	response := map[string]string{"token": token, "expiresAt": expiresAt.UTC().Format(time.RFC3339)}
	if err := httpx.WriteJSON(w, http.StatusCreated, response); err != nil {
		slog.ErrorContext(r.Context(), "write token response", "error", err)
	}
}

// HandleServiceProviderConfig advertises supported SCIM capabilities.
func (h *Handler) HandleServiceProviderConfig(w http.ResponseWriter, r *http.Request) {
	h.writeSCIM(w, r, http.StatusOK, ServiceProviderConfig{
		Schemas:        []string{SchemaServiceProviderConfig},
		Patch:          Supported{Supported: true},
		Bulk:           BulkConfig{Supported: false, MaxOperations: 0, MaxPayloadSize: 0},
		Filter:         FilterConfig{Supported: true, MaxResults: filterMaxResult},
		ChangePassword: Supported{Supported: false},
		Sort:           Supported{Supported: false},
		ETag:           Supported{Supported: false},
		AuthenticationSchemes: []AuthenticationType{{
			Type:        "oauthbearertoken",
			Name:        "OAuth Bearer Token",
			Description: "Authentication via a per-organization SCIM bearer token.",
			Primary:     true,
		}},
	})
}

// HandleListUsers lists tenant users, optionally filtered by userName, with SCIM
// startIndex/count pagination.
//
//nolint:dupl // parallel to HandleListGroups; distinct resource + envelope types.
func (h *Handler) HandleListUsers(w http.ResponseWriter, r *http.Request) {
	organizationID, ok := organizationFromContext(r.Context())
	if !ok {
		h.writeError(w, r, http.StatusUnauthorized, "Authentication required")

		return
	}

	offset, limit := parsePage(r.URL.Query().Get("startIndex"), r.URL.Query().Get("count"))

	records, total, err := h.svc.ListUsers(
		r.Context(), organizationID, parseUserNameFilter(r.URL.Query().Get("filter")), offset, limit,
	)
	if err != nil {
		h.mapError(w, r, err)

		return
	}

	resources := make([]User, len(records))
	for i, record := range records {
		resources[i] = toUser(record)
	}

	h.writeSCIM(w, r, http.StatusOK, ListResponse{
		Schemas:      []string{SchemaListResponse},
		TotalResults: total,
		StartIndex:   offset + 1,
		ItemsPerPage: len(resources),
		Resources:    resources,
	})
}

// parsePage converts SCIM `startIndex` (1-based) and `count` query values into a
// 0-based offset and a page limit. Missing startIndex -> offset 0; missing count
// -> defaultPageSize; count is clamped to [0, filterMaxResult] (0 = count only).
func parsePage(startIndexRaw, countRaw string) (int, int) {
	offset := 0
	if start, err := strconv.Atoi(strings.TrimSpace(startIndexRaw)); err == nil && start > 1 {
		offset = start - 1
	}

	limit := defaultPageSize

	if raw := strings.TrimSpace(countRaw); raw != "" {
		if count, err := strconv.Atoi(raw); err == nil {
			switch {
			case count < 0:
				limit = 0
			case count > filterMaxResult:
				limit = filterMaxResult
			default:
				limit = count
			}
		}
	}

	return offset, limit
}

// HandleCreateUser provisions a new user.
func (h *Handler) HandleCreateUser(w http.ResponseWriter, r *http.Request) {
	organizationID, ok := organizationFromContext(r.Context())
	if !ok {
		h.writeError(w, r, http.StatusUnauthorized, "Authentication required")

		return
	}

	payload, err := decodeUser(r)
	if err != nil {
		h.writeError(w, r, http.StatusBadRequest, "Request body is invalid")

		return
	}

	record, err := h.svc.CreateUser(r.Context(), organizationID, toCreateInput(payload, true))
	if err != nil {
		h.mapError(w, r, err)

		return
	}

	h.writeSCIM(w, r, http.StatusCreated, toUser(record))
}

// HandleGetUser returns a single user resource.
func (h *Handler) HandleGetUser(w http.ResponseWriter, r *http.Request) {
	organizationID, ok := organizationFromContext(r.Context())
	if !ok {
		h.writeError(w, r, http.StatusUnauthorized, "Authentication required")

		return
	}

	record, err := h.svc.GetUser(r.Context(), organizationID, chi.URLParam(r, "userID"))
	if err != nil {
		h.mapError(w, r, err)

		return
	}

	h.writeSCIM(w, r, http.StatusOK, toUser(record))
}

// HandleReplaceUser applies a full-resource update.
func (h *Handler) HandleReplaceUser(w http.ResponseWriter, r *http.Request) {
	organizationID, ok := organizationFromContext(r.Context())
	if !ok {
		h.writeError(w, r, http.StatusUnauthorized, "Authentication required")

		return
	}

	payload, err := decodeUser(r)
	if err != nil {
		h.writeError(w, r, http.StatusBadRequest, "Request body is invalid")

		return
	}

	record, err := h.svc.ReplaceUser(r.Context(), organizationID, chi.URLParam(r, "userID"), toCreateInput(payload, true))
	if err != nil {
		h.mapError(w, r, err)

		return
	}

	h.writeSCIM(w, r, http.StatusOK, toUser(record))
}

// HandlePatchUser applies an active/inactive change.
func (h *Handler) HandlePatchUser(w http.ResponseWriter, r *http.Request) {
	organizationID, ok := organizationFromContext(r.Context())
	if !ok {
		h.writeError(w, r, http.StatusUnauthorized, "Authentication required")

		return
	}

	var req PatchRequest
	if err := decodeBody(r, &req); err != nil {
		h.writeError(w, r, http.StatusBadRequest, "Request body is invalid")

		return
	}

	userID := chi.URLParam(r, "userID")

	active := parsePatchActive(req)
	if active == nil {
		record, err := h.svc.GetUser(r.Context(), organizationID, userID)
		if err != nil {
			h.mapError(w, r, err)

			return
		}

		h.writeSCIM(w, r, http.StatusOK, toUser(record))

		return
	}

	record, err := h.svc.SetActive(r.Context(), organizationID, userID, *active)
	if err != nil {
		h.mapError(w, r, err)

		return
	}

	h.writeSCIM(w, r, http.StatusOK, toUser(record))
}

// HandleDeleteUser deprovisions and removes a user resource.
func (h *Handler) HandleDeleteUser(w http.ResponseWriter, r *http.Request) {
	organizationID, ok := organizationFromContext(r.Context())
	if !ok {
		h.writeError(w, r, http.StatusUnauthorized, "Authentication required")

		return
	}

	if err := h.svc.DeleteUser(r.Context(), organizationID, chi.URLParam(r, "userID")); err != nil {
		h.mapError(w, r, err)

		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) mapError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, ErrInvalid):
		h.writeError(w, r, http.StatusBadRequest, "Invalid request")
	case errors.Is(err, ErrConflict):
		h.writeError(w, r, http.StatusConflict, "User already exists")
	case errors.Is(err, ErrNotFound):
		h.writeError(w, r, http.StatusNotFound, "Resource not found")
	case errors.Is(err, ErrUnauthorized):
		h.writeError(w, r, http.StatusUnauthorized, "Authentication required")
	default:
		slog.ErrorContext(r.Context(), "scim handler error", "error", err)
		h.writeError(w, r, http.StatusInternalServerError, "Unexpected error")
	}
}

func (h *Handler) writeSCIM(w http.ResponseWriter, r *http.Request, status int, v any) {
	w.Header().Set("Content-Type", ContentType)
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.ErrorContext(r.Context(), "write scim response", "error", err)
	}
}

func (h *Handler) writeError(w http.ResponseWriter, r *http.Request, status int, detail string) {
	w.Header().Set("Content-Type", ContentType)
	w.WriteHeader(status)

	body := ErrorResponse{Schemas: []string{SchemaError}, Detail: detail, Status: strconv.Itoa(status)}
	if err := json.NewEncoder(w).Encode(body); err != nil {
		slog.ErrorContext(r.Context(), "write scim error response", "error", err)
	}
}

func toUser(record Record) User {
	return User{
		Schemas:    []string{SchemaUser},
		ID:         record.ID,
		ExternalID: record.ExternalID,
		UserName:   record.UserName,
		Name:       &Name{GivenName: record.GivenName, FamilyName: record.FamilyName, Formatted: ""},
		Active:     record.Active,
		Meta: &Meta{
			ResourceType: resourceTypeUser,
			Created:      record.CreatedAt.UTC().Format(time.RFC3339),
			LastModified: record.UpdatedAt.UTC().Format(time.RFC3339),
			Location:     "Users/" + record.ID,
		},
	}
}

func toCreateInput(payload userPayload, activeDefault bool) CreateUserInput {
	active := activeDefault
	if payload.Active != nil {
		active = *payload.Active
	}

	var given, family string
	if payload.Name != nil {
		given = payload.Name.GivenName
		family = payload.Name.FamilyName
	}

	return CreateUserInput{
		UserName:    payload.UserName,
		ExternalID:  payload.ExternalID,
		Email:       primaryEmail(payload),
		DisplayName: payload.DisplayName,
		GivenName:   given,
		FamilyName:  family,
		Active:      active,
	}
}

func primaryEmail(payload userPayload) string {
	for _, email := range payload.Emails {
		if email.Primary && strings.TrimSpace(email.Value) != "" {
			return strings.ToLower(strings.TrimSpace(email.Value))
		}
	}

	for _, email := range payload.Emails {
		if strings.TrimSpace(email.Value) != "" {
			return strings.ToLower(strings.TrimSpace(email.Value))
		}
	}

	if strings.Contains(payload.UserName, "@") {
		return strings.ToLower(strings.TrimSpace(payload.UserName))
	}

	return ""
}

// parseUserNameFilter extracts the value from a `userName eq "x"` SCIM filter.
func parseUserNameFilter(filter string) string {
	filter = strings.TrimSpace(filter)
	if filter == "" {
		return ""
	}

	_, after, found := strings.Cut(filter, "userName eq ")
	if !found {
		return ""
	}

	return strings.Trim(strings.TrimSpace(after), `"`)
}

func parsePatchActive(req PatchRequest) *bool {
	var result *bool

	for _, op := range req.Operations {
		if !strings.EqualFold(op.Op, "replace") && !strings.EqualFold(op.Op, "add") {
			continue
		}

		if strings.EqualFold(op.Path, "active") {
			var value bool
			if json.Unmarshal(op.Value, &value) == nil {
				result = &value
			}

			continue
		}

		if op.Path == "" {
			var payload struct {
				Active *bool `json:"active"`
			}
			if json.Unmarshal(op.Value, &payload) == nil && payload.Active != nil {
				result = payload.Active
			}
		}
	}

	return result
}

func decodeUser(r *http.Request) (userPayload, error) {
	var payload userPayload
	if err := decodeBody(r, &payload); err != nil {
		return userPayload{}, err
	}

	return payload, nil
}

// decodeBody reads a SCIM body leniently, ignoring unknown IdP-supplied
// attributes (unlike httpx.DecodeJSON, which rejects them).
func decodeBody(r *http.Request, dst any) error {
	defer func() { _ = r.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(r.Body, maxBodyBytes))
	if err != nil {
		return fmt.Errorf("read scim body: %w", err)
	}

	if err := json.Unmarshal(body, dst); err != nil {
		return fmt.Errorf("decode scim body: %w", err)
	}

	return nil
}
