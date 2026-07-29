// Package platform implements platform staff use cases and HTTP handlers.
package platform

import (
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"launchpad/internal/audit"
	"launchpad/internal/organizations"
	"launchpad/pkg/httpx"
	"launchpad/pkg/security"
)

// Handler exposes platform HTTP endpoints.
type Handler struct {
	svc   *Service
	audit *audit.Service
}

// NewHandler constructs a platform Handler.
func NewHandler(svc *Service, auditSvc *audit.Service) *Handler {
	return &Handler{svc: svc, audit: auditSvc}
}

// HandleOverview returns platform-wide metrics.
func (h *Handler) HandleOverview(w http.ResponseWriter, r *http.Request) {
	overview, err := h.svc.Overview(r.Context())
	if err != nil {
		slog.ErrorContext(r.Context(), "platform overview failed", "error", err)
		writeError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to load overview")

		return
	}

	writeJSON(w, r, overview)
}

// HandleLaunchReadiness reports launch-readiness checks. Mounted on the
// platform route group, which already restricts access to platform staff.
func (h *Handler) HandleLaunchReadiness(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, r, h.svc.LaunchReadiness(r.Context()))
}

func (h *Handler) HandleStorageOverview(w http.ResponseWriter, r *http.Request) {
	overview, err := h.svc.StorageOverview(r.Context())
	if err != nil {
		slog.ErrorContext(r.Context(), "platform storage overview failed", "error", err)
		writeError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to load storage overview")
		return
	}
	writeJSON(w, r, overview)
}

// HandleListOrganizations lists all tenant organizations.
func (h *Handler) HandleListOrganizations(w http.ResponseWriter, r *http.Request) {
	items, err := h.svc.ListOrganizations(r.Context(), OrganizationListInput{
		Search:   r.URL.Query().Get("search"),
		Status:   r.URL.Query().Get("status"),
		PlanCode: r.URL.Query().Get("planCode"),
	})
	if err != nil {
		slog.ErrorContext(r.Context(), "list organizations failed", "error", err)
		writeError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to list organizations")

		return
	}

	responses := make([]organizations.OrganizationResponse, 0, len(items))
	for _, item := range items {
		responses = append(responses, item.ToResponse())
	}

	writeJSON(w, r, responses)
}

// HandleCloseOrganization moves a tenant into the terminal closed state.
// Tenant data remains available for authorized export/purge workflows.
func (h *Handler) HandleCloseOrganization(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(w, r)
	if !ok {
		return
	}

	organizationID := chi.URLParam(r, "organizationID")
	org, err := h.svc.SetOrganizationStatus(r.Context(), organizationID, organizations.StatusClosed())
	if err != nil {
		h.recordAuditFailure(r, principal, organizationID, "organization.closed")
		writeOrganizationError(w, r, err)

		return
	}

	if !h.recordAudit(w, r, principal, organizationID, "organization.closed", org.ID) {
		return
	}

	writeJSON(w, r, org.ToResponse())
}

// HandleGetOrganization returns one tenant organization.
func (h *Handler) HandleGetOrganization(w http.ResponseWriter, r *http.Request) {
	org, err := h.svc.GetOrganization(r.Context(), chi.URLParam(r, "organizationID"))
	if err != nil {
		writeOrganizationError(w, r, err)

		return
	}

	writeJSON(w, r, org.ToResponse())
}

// HandleSuspendOrganization suspends a tenant organization.
func (h *Handler) HandleSuspendOrganization(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(w, r)
	if !ok {
		return
	}

	organizationID := chi.URLParam(r, "organizationID")

	org, err := h.svc.SetOrganizationStatus(r.Context(), organizationID, organizations.StatusSuspended())
	if err != nil {
		h.recordAuditFailure(r, principal, organizationID, "organization.suspended")
		writeOrganizationError(w, r, err)

		return
	}

	if !h.recordAudit(w, r, principal, organizationID, "organization.suspended", org.ID) {
		return
	}

	writeJSON(w, r, org.ToResponse())
}

// HandleActivateOrganization activates a tenant organization.
func (h *Handler) HandleActivateOrganization(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(w, r)
	if !ok {
		return
	}

	organizationID := chi.URLParam(r, "organizationID")

	org, err := h.svc.SetOrganizationStatus(r.Context(), organizationID, organizations.StatusActive())
	if err != nil {
		h.recordAuditFailure(r, principal, organizationID, "organization.activated")
		writeOrganizationError(w, r, err)

		return
	}

	if !h.recordAudit(w, r, principal, organizationID, "organization.activated", org.ID) {
		return
	}

	writeJSON(w, r, org.ToResponse())
}

// fieldRoleCode is the audit metadata key for staff role changes.
const fieldRoleCode = "roleCode"

// StaffResponse is the API representation of a platform staff record.
type StaffResponse struct {
	ID               string           `json:"id"`
	UserID           string           `json:"userId"`
	Email            string           `json:"email"`
	DisplayName      string           `json:"displayName"`
	RoleCode         string           `json:"roleCode"`
	Status           string           `json:"status"`
	CreatedAt        time.Time        `json:"createdAt"`
	BreakGlass       *BreakGlassGrant `json:"breakGlass,omitempty"`
	AccessReviewedAt *time.Time       `json:"accessReviewedAt,omitempty"`
	AccessReviewedBy string           `json:"accessReviewedBy,omitempty"`
}

// CreateStaffResponse is returned once by staff creation; TempPassword is
// only present when no mail sender is configured.
type CreateStaffResponse struct {
	Staff        StaffResponse `json:"staff"`
	TempPassword string        `json:"tempPassword,omitempty"`
	Invited      bool          `json:"invited"`
}

func toStaffResponse(staff Staff) StaffResponse {
	return StaffResponse(staff)
}

// HandleListStaff lists all platform staff accounts.
func (h *Handler) HandleListStaff(w http.ResponseWriter, r *http.Request) {
	items, err := h.svc.ListStaff(r.Context())
	if err != nil {
		slog.ErrorContext(r.Context(), "list platform staff failed", "error", err)
		writeError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to list staff")

		return
	}

	responses := make([]StaffResponse, 0, len(items))
	for _, item := range items {
		responses = append(responses, toStaffResponse(item))
	}

	writeJSON(w, r, responses)
}

func (h *Handler) HandleAccessReview(w http.ResponseWriter, r *http.Request) {
	items, err := h.svc.AccessReview(r.Context())
	if err != nil {
		writeStaffError(w, r, err)
		return
	}
	writeJSON(w, r, items)
}

func (h *Handler) HandleAttestAccess(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(w, r)
	if !ok {
		return
	}
	staff, err := h.svc.AttestAccess(r.Context(), principal.UserID, chi.URLParam(r, "staffID"))
	if err != nil {
		writeStaffError(w, r, err)
		return
	}
	if !h.recordStaffAudit(w, r, principal, staff.ID, "staff.access_attested", nil) {
		return
	}
	writeJSON(w, r, toStaffResponse(staff))
}

func (h *Handler) HandleGrantBreakGlass(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(w, r)
	if !ok {
		return
	}
	var body struct {
		Reason          string `json:"reason"`
		DurationMinutes int    `json:"durationMinutes"`
	}
	if err := httpx.DecodeJSON(r, &body); err != nil {
		writeError(w, r, http.StatusBadRequest, "INVALID_JSON", "Request body is invalid")
		return
	}
	staff, err := h.svc.GrantBreakGlass(
		r.Context(), principal.UserID, chi.URLParam(r, "staffID"),
		body.Reason, time.Duration(body.DurationMinutes)*time.Minute,
	)
	if err != nil {
		h.recordStaffAuditFailure(r, principal, chi.URLParam(r, "staffID"), "staff.break_glass_granted")
		writeStaffError(w, r, err)
		return
	}
	if !h.recordStaffAudit(w, r, principal, staff.ID, "staff.break_glass_granted", map[string]any{
		"reason": body.Reason, "expiresAt": staff.BreakGlass.ExpiresAt,
	}) {
		return
	}
	writeJSON(w, r, toStaffResponse(staff))
}

func (h *Handler) HandleRevokeBreakGlass(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(w, r)
	if !ok {
		return
	}
	staff, err := h.svc.RevokeBreakGlass(r.Context(), principal.UserID, chi.URLParam(r, "staffID"))
	if err != nil {
		writeStaffError(w, r, err)
		return
	}
	if !h.recordStaffAudit(w, r, principal, staff.ID, "staff.break_glass_revoked", nil) {
		return
	}
	writeJSON(w, r, toStaffResponse(staff))
}

// HandleCreateStaff creates a platform staff account with a temporary
// password (emailed when a sender is configured, otherwise returned once).
func (h *Handler) HandleCreateStaff(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(w, r)
	if !ok {
		return
	}

	var body struct {
		Email       string `json:"email"`
		DisplayName string `json:"displayName"`
		RoleCode    string `json:"roleCode"`
	}
	if err := httpx.DecodeJSON(r, &body); err != nil {
		writeError(w, r, http.StatusBadRequest, "INVALID_JSON", "Request body is invalid")

		return
	}

	result, err := h.svc.CreateStaff(r.Context(), CreateStaffInput{
		Email:       body.Email,
		DisplayName: body.DisplayName,
		RoleCode:    body.RoleCode,
	})
	if err != nil {
		h.recordStaffAuditFailure(r, principal, "", "staff.created")
		writeStaffError(w, r, err)

		return
	}

	if !h.recordStaffAudit(w, r, principal, result.Staff.ID, "staff.created", map[string]any{
		"email":       result.Staff.Email,
		fieldRoleCode: result.Staff.RoleCode,
	}) {
		return
	}

	writeCreated(w, r, CreateStaffResponse{
		Staff:        toStaffResponse(result.Staff),
		TempPassword: result.TempPassword,
		Invited:      result.Invited,
	})
}

// HandleUpdateStaffRole changes a staff member's role.
func (h *Handler) HandleUpdateStaffRole(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(w, r)
	if !ok {
		return
	}

	var body struct {
		RoleCode string `json:"roleCode"`
	}
	if err := httpx.DecodeJSON(r, &body); err != nil {
		writeError(w, r, http.StatusBadRequest, "INVALID_JSON", "Request body is invalid")

		return
	}

	staffID := chi.URLParam(r, "staffID")

	staff, err := h.svc.UpdateStaffRole(r.Context(), staffID, body.RoleCode)
	if err != nil {
		h.recordStaffAuditFailure(r, principal, staffID, "staff.role_updated")
		writeStaffError(w, r, err)

		return
	}

	if !h.recordStaffAudit(w, r, principal, staff.ID, "staff.role_updated", map[string]any{
		fieldRoleCode: staff.RoleCode,
	}) {
		return
	}

	writeJSON(w, r, toStaffResponse(staff))
}

// HandleDeactivateStaff deactivates a staff account, blocking further logins.
func (h *Handler) HandleDeactivateStaff(w http.ResponseWriter, r *http.Request) {
	h.handleSetStaffStatus(w, r, staffStatusDeactivated, "staff.deactivated")
}

// HandleReactivateStaff reactivates a deactivated staff account.
func (h *Handler) HandleReactivateStaff(w http.ResponseWriter, r *http.Request) {
	h.handleSetStaffStatus(w, r, staffStatusActive, "staff.reactivated")
}

func (h *Handler) handleSetStaffStatus(w http.ResponseWriter, r *http.Request, status, action string) {
	principal, ok := requirePrincipal(w, r)
	if !ok {
		return
	}

	staffID := chi.URLParam(r, "staffID")

	staff, err := h.svc.SetStaffStatus(r.Context(), principal.UserID, staffID, status)
	if err != nil {
		h.recordStaffAuditFailure(r, principal, staffID, action)
		writeStaffError(w, r, err)

		return
	}

	if !h.recordStaffAudit(w, r, principal, staff.ID, action, map[string]any{fieldRoleCode: staff.RoleCode}) {
		return
	}

	writeJSON(w, r, toStaffResponse(staff))
}

// recordStaffAudit writes an audit event for a platform staff administration
// action. Staff accounts are platform-scoped, so the organization is nil.
func (h *Handler) recordStaffAudit(
	w http.ResponseWriter,
	r *http.Request,
	principal security.Principal,
	staffID, action string,
	metadata map[string]any,
) bool {
	if err := h.audit.Record(r.Context(), nil, principal.UserID, action, "platform_staff", staffID, metadata); err != nil {
		slog.ErrorContext(r.Context(), "audit platform staff action failed", "error", err, "action", action)
		writeError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to record audit event")

		return false
	}

	return true
}

// recordStaffAuditFailure best-effort records a rejected staff administration
// action so failed privileged changes still appear in the audit log.
func (h *Handler) recordStaffAuditFailure(
	r *http.Request,
	principal security.Principal,
	staffID, action string,
) {
	err := h.audit.RecordResult(
		r.Context(),
		nil,
		principal.UserID,
		action,
		"platform_staff",
		staffID,
		audit.ResultFailure,
		"action_rejected",
		nil,
	)
	if err != nil {
		slog.WarnContext(r.Context(), "audit failed platform staff action", "error", err, "action", action)
	}
}

func writeStaffError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		writeError(w, r, http.StatusNotFound, "NOT_FOUND", "Staff account not found")
	case errors.Is(err, ErrInvalidInput):
		writeError(w, r, http.StatusBadRequest, "INVALID_INPUT", err.Error())
	case errors.Is(err, ErrProvisioningUnavailable):
		writeError(w, r, http.StatusServiceUnavailable, "PROVISIONING_UNAVAILABLE", err.Error())
	default:
		slog.ErrorContext(r.Context(), "platform staff handler error", "error", err)
		writeError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Unexpected error")
	}
}

func writeCreated(w http.ResponseWriter, r *http.Request, data any) {
	if err := httpx.WriteJSON(w, http.StatusCreated, data); err != nil {
		slog.ErrorContext(r.Context(), "write json response", "error", err)
	}
}

func requirePrincipal(w http.ResponseWriter, r *http.Request) (security.Principal, bool) {
	principal, ok := security.PrincipalFromContext(r.Context())
	if !ok {
		writeError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")

		return security.Principal{}, false
	}

	return principal, true
}

// recordAudit writes an audit event for a platform action against a tenant.
// The target organization is the audited tenant; the actor is the platform staff member.
func (h *Handler) recordAudit(
	w http.ResponseWriter,
	r *http.Request,
	principal security.Principal,
	organizationID, action, resourceID string,
) bool {
	orgID := organizationID
	if err := h.audit.Record(
		r.Context(),
		&orgID,
		principal.UserID,
		action,
		"organization",
		resourceID,
		nil,
	); err != nil {
		slog.ErrorContext(r.Context(), "audit platform action failed", "error", err, "action", action)
		writeError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to record audit event")

		return false
	}

	return true
}

// recordAuditFailure best-effort records a rejected platform action so failed
// privileged changes still appear in the audit log (result=failure). The
// original service error, never an audit write failure, drives the response.
func (h *Handler) recordAuditFailure(
	r *http.Request,
	principal security.Principal,
	organizationID, action string,
) {
	orgID := organizationID

	err := h.audit.RecordResult(
		r.Context(),
		&orgID,
		principal.UserID,
		action,
		"organization",
		organizationID,
		audit.ResultFailure,
		"action_rejected",
		nil,
	)
	if err != nil {
		slog.WarnContext(r.Context(), "audit failed platform action", "error", err, "action", action)
	}
}

func writeOrganizationError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, organizations.ErrNotFound):
		writeError(w, r, http.StatusNotFound, "NOT_FOUND", "Organization not found")
	case errors.Is(err, organizations.ErrInvalidInput):
		writeError(w, r, http.StatusBadRequest, "INVALID_INPUT", err.Error())
	default:
		slog.ErrorContext(r.Context(), "platform organization handler error", "error", err)
		writeError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Unexpected error")
	}
}

func writeJSON(w http.ResponseWriter, r *http.Request, data any) {
	if err := httpx.WriteJSON(w, http.StatusOK, data); err != nil {
		slog.ErrorContext(r.Context(), "write json response", "error", err)
	}
}

func writeError(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	if err := httpx.WriteError(w, status, code, message); err != nil {
		slog.ErrorContext(r.Context(), "write error response", "error", err)
	}
}
