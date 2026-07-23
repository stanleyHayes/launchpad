package departments

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"launchpad/internal/audit"
	"launchpad/internal/organizations"
	"launchpad/pkg/httpx"
	"launchpad/pkg/security"
)

// Handler exposes department and job-role HTTP endpoints.
type Handler struct {
	svc   *Service
	audit *audit.Service
}

type namedCreateResult struct {
	resourceID string
	payload    any
}

// NewHandler constructs a Handler.
func NewHandler(svc *Service, auditSvc *audit.Service) *Handler {
	return &Handler{svc: svc, audit: auditSvc}
}

// HandleListDepartments lists departments for the current organization.
func (h *Handler) HandleListDepartments(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(w, r)
	if !ok {
		return
	}

	items, err := h.svc.ListDepartments(r.Context(), principal.OrganizationID)
	if err != nil {
		slog.ErrorContext(r.Context(), "list departments failed", "error", err)
		writeError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to list departments")

		return
	}

	writeJSON(w, r, http.StatusOK, items)
}

// HandleCreateDepartment creates a department.
func (h *Handler) HandleCreateDepartment(w http.ResponseWriter, r *http.Request) {
	h.createNamedResource(w, r, "department.created", "department", func(
		ctx context.Context,
		organizationID, name, description string,
	) (namedCreateResult, error) {
		department, err := h.svc.CreateDepartment(ctx, organizationID, CreateDepartmentInput{
			Name:        name,
			Description: description,
		})
		if err != nil {
			return namedCreateResult{}, err
		}

		return namedCreateResult{resourceID: department.ID, payload: department}, nil
	})
}

// HandleListJobRoles lists job roles for the current organization.
func (h *Handler) HandleListJobRoles(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(w, r)
	if !ok {
		return
	}

	items, err := h.svc.ListJobRoles(r.Context(), principal.OrganizationID)
	if err != nil {
		slog.ErrorContext(r.Context(), "list job roles failed", "error", err)
		writeError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to list job roles")

		return
	}

	writeJSON(w, r, http.StatusOK, items)
}

// HandleCreateJobRole creates a job role.
func (h *Handler) HandleCreateJobRole(w http.ResponseWriter, r *http.Request) {
	h.createNamedResource(w, r, "job_role.created", "job_role", func(
		ctx context.Context,
		organizationID, name, description string,
	) (namedCreateResult, error) {
		role, err := h.svc.CreateJobRole(ctx, organizationID, CreateJobRoleInput{
			Name:        name,
			Description: description,
		})
		if err != nil {
			return namedCreateResult{}, err
		}

		return namedCreateResult{resourceID: role.ID, payload: role}, nil
	})
}

func (h *Handler) createNamedResource(
	w http.ResponseWriter,
	r *http.Request,
	auditAction, resourceType string,
	createFn func(context.Context, string, string, string) (namedCreateResult, error),
) {
	principal, ok := requireManager(w, r)
	if !ok {
		return
	}

	var body struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := httpx.DecodeJSON(r, &body); err != nil {
		writeError(w, r, http.StatusBadRequest, "INVALID_JSON", "Request body is invalid")

		return
	}

	created, err := createFn(r.Context(), principal.OrganizationID, body.Name, body.Description)
	if err != nil {
		writeDepartmentError(w, r, err)

		return
	}

	orgID := principal.OrganizationID
	if err := h.audit.Record(
		r.Context(),
		&orgID,
		principal.UserID,
		auditAction,
		resourceType,
		created.resourceID,
		map[string]any{"name": body.Name},
	); err != nil {
		slog.ErrorContext(r.Context(), "audit named resource create failed", "error", err, "action", auditAction)
		writeError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to record audit event")

		return
	}

	writeJSON(w, r, http.StatusCreated, created.payload)
}

func requirePrincipal(w http.ResponseWriter, r *http.Request) (security.Principal, bool) {
	principal, ok := security.PrincipalFromContext(r.Context())
	if !ok {
		writeError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")

		return security.Principal{}, false
	}

	return principal, true
}

func requireManager(w http.ResponseWriter, r *http.Request) (security.Principal, bool) {
	principal, ok := requirePrincipal(w, r)
	if !ok {
		return security.Principal{}, false
	}

	if !organizations.CanManageOrganization(principal.RoleCode) {
		writeError(w, r, http.StatusForbidden, "FORBIDDEN", "Insufficient permissions")

		return security.Principal{}, false
	}

	return principal, true
}

func writeDepartmentError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, ErrInvalidInput):
		writeError(w, r, http.StatusBadRequest, "INVALID_INPUT", err.Error())
	case errors.Is(err, ErrNameTaken), errors.Is(err, ErrRoleNameTaken):
		writeError(w, r, http.StatusConflict, "NAME_TAKEN", err.Error())
	default:
		slog.ErrorContext(r.Context(), "department handler error", "error", err)
		writeError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Unexpected error")
	}
}

func writeJSON(w http.ResponseWriter, r *http.Request, status int, data any) {
	if err := httpx.WriteJSON(w, status, data); err != nil {
		slog.ErrorContext(r.Context(), "write json response", "error", err)
	}
}

func writeError(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	if err := httpx.WriteError(w, status, code, message); err != nil {
		slog.ErrorContext(r.Context(), "write error response", "error", err)
	}
}
