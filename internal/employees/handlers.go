package employees

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"launchpad/internal/audit"
	"launchpad/internal/organizations"
	"launchpad/pkg/httpx"
	"launchpad/pkg/security"
)

// Handler exposes employee HTTP endpoints.
type Handler struct {
	svc      *Service
	audit    *audit.Service
	accounts AccountCreator
	members  MemberAdder
}

// AccountCreator creates login accounts for employees.
type AccountCreator interface {
	CreateUserAccount(ctx context.Context, email, displayName, password string) (userID string, err error)
}

// MemberAdder adds organization memberships.
type MemberAdder interface {
	AddEmployeeMember(ctx context.Context, organizationID, userID string) error
}

var (
	errProvisionAccount = errors.New("provision account failed")
	errProvisionMember  = errors.New("provision member failed")
)

// NewHandler constructs a Handler.
func NewHandler(svc *Service, auditSvc *audit.Service, accounts AccountCreator, members MemberAdder) *Handler {
	return &Handler{svc: svc, audit: auditSvc, accounts: accounts, members: members}
}

// HandleList lists employees for the current organization.
func (h *Handler) HandleList(w http.ResponseWriter, r *http.Request) {
	principal, ok := security.PrincipalFromContext(r.Context())
	if !ok {
		writeError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")

		return
	}

	limit := int64(0)

	if raw := r.URL.Query().Get("limit"); raw != "" {
		parsed, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			writeError(w, r, http.StatusBadRequest, "INVALID_LIMIT", "limit must be an integer")

			return
		}

		limit = parsed
	}

	items, err := h.svc.List(r.Context(), principal.OrganizationID, limit)
	if err != nil {
		slog.ErrorContext(r.Context(), "list employees failed", "error", err)
		writeError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to list employees")

		return
	}

	writeJSON(w, r, http.StatusOK, items)
}

// HandleCreate creates an employee.
func (h *Handler) HandleCreate(w http.ResponseWriter, r *http.Request) {
	principal, ok := security.PrincipalFromContext(r.Context())
	if !ok {
		writeError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")

		return
	}

	if !organizations.CanManageOrganization(principal.RoleCode) {
		writeError(w, r, http.StatusForbidden, "FORBIDDEN", "Insufficient permissions")

		return
	}

	var body struct {
		EmployeeNumber    string `json:"employeeNumber"`
		FirstName         string `json:"firstName"`
		LastName          string `json:"lastName"`
		WorkEmail         string `json:"workEmail"`
		JobRoleID         string `json:"jobRoleId"`
		DepartmentID      string `json:"departmentId"`
		ManagerEmployeeID string `json:"managerEmployeeId"`
		StartDate         string `json:"startDate"`
	}
	if err := httpx.DecodeJSON(r, &body); err != nil {
		writeError(w, r, http.StatusBadRequest, "INVALID_JSON", "Request body is invalid")

		return
	}

	startDate, err := time.Parse("2006-01-02", body.StartDate)
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "INVALID_INPUT", "startDate must be YYYY-MM-DD")

		return
	}

	employee, err := h.svc.Create(r.Context(), principal.OrganizationID, CreateInput{
		EmployeeNumber:    body.EmployeeNumber,
		FirstName:         body.FirstName,
		LastName:          body.LastName,
		WorkEmail:         body.WorkEmail,
		JobRoleID:         body.JobRoleID,
		DepartmentID:      body.DepartmentID,
		ManagerEmployeeID: body.ManagerEmployeeID,
		StartDate:         startDate,
	})
	if err != nil {
		writeEmployeeError(w, r, err)

		return
	}

	orgID := principal.OrganizationID
	if err := h.audit.Record(
		r.Context(),
		&orgID,
		principal.UserID,
		"employee.created",
		"employee",
		employee.ID,
		map[string]any{"workEmail": employee.WorkEmail},
	); err != nil {
		slog.ErrorContext(r.Context(), "audit employee create failed", "error", err)
		writeError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to record audit event")

		return
	}

	writeJSON(w, r, http.StatusCreated, employee)
}

// HandleGet returns one employee.
func (h *Handler) HandleGet(w http.ResponseWriter, r *http.Request) {
	principal, ok := security.PrincipalFromContext(r.Context())
	if !ok {
		writeError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")

		return
	}

	employeeID := chi.URLParam(r, "employeeID")

	employee, err := h.svc.Get(r.Context(), principal.OrganizationID, employeeID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			writeError(w, r, http.StatusNotFound, "NOT_FOUND", "Employee not found")

			return
		}

		slog.ErrorContext(r.Context(), "get employee failed", "error", err)
		writeError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to load employee")

		return
	}

	writeJSON(w, r, http.StatusOK, employee)
}

// HandleProvisionAccess creates login credentials for an invited employee.
func (h *Handler) HandleProvisionAccess(w http.ResponseWriter, r *http.Request) {
	principal, ok := security.PrincipalFromContext(r.Context())
	if !ok {
		writeError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")

		return
	}

	if !organizations.CanManageOrganization(principal.RoleCode) {
		writeError(w, r, http.StatusForbidden, "FORBIDDEN", "Insufficient permissions")

		return
	}

	var body struct {
		Password    string `json:"password"`
		DisplayName string `json:"displayName"`
	}
	if err := httpx.DecodeJSON(r, &body); err != nil {
		writeError(w, r, http.StatusBadRequest, "INVALID_JSON", "Request body is invalid")

		return
	}

	updated, userID, err := h.provisionEmployeeAccess(
		r.Context(),
		principal.OrganizationID,
		chi.URLParam(r, "employeeID"),
		body.DisplayName,
		body.Password,
	)
	if err != nil {
		switch {
		case errors.Is(err, errProvisionAccount):
			writeError(w, r, http.StatusBadRequest, "PROVISION_FAILED", err.Error())
		case errors.Is(err, errProvisionMember):
			writeError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to add membership")
		default:
			writeEmployeeError(w, r, err)
		}

		return
	}

	orgID := principal.OrganizationID
	if err := h.audit.Record(
		r.Context(),
		&orgID,
		principal.UserID,
		"employee.provisioned",
		"employee",
		updated.ID,
		map[string]any{"userId": userID},
	); err != nil {
		slog.ErrorContext(r.Context(), "audit employee provision failed", "error", err)
		writeError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to record audit event")

		return
	}

	writeJSON(w, r, http.StatusOK, updated)
}

func (h *Handler) provisionEmployeeAccess(
	ctx context.Context,
	organizationID, employeeID, displayName, password string,
) (Employee, string, error) {
	employee, err := h.svc.Get(ctx, organizationID, employeeID)
	if err != nil {
		return Employee{}, "", err
	}

	if displayName == "" {
		displayName = employee.FirstName + " " + employee.LastName
	}

	userID, err := h.accounts.CreateUserAccount(ctx, employee.WorkEmail, displayName, password)
	if err != nil {
		return Employee{}, "", fmt.Errorf("%w: %w", errProvisionAccount, err)
	}

	if err := h.members.AddEmployeeMember(ctx, organizationID, userID); err != nil {
		return Employee{}, "", fmt.Errorf("%w: %w", errProvisionMember, err)
	}

	updated, err := h.svc.LinkUser(ctx, organizationID, employeeID, userID)
	if err != nil {
		return Employee{}, "", err
	}

	return updated, userID, nil
}

func writeEmployeeError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, ErrInvalidInput):
		writeError(w, r, http.StatusBadRequest, "INVALID_INPUT", err.Error())
	case errors.Is(err, ErrInvalidReference):
		writeError(w, r, http.StatusBadRequest, "INVALID_REFERENCE", err.Error())
	case errors.Is(err, ErrEmailTaken):
		writeError(w, r, http.StatusConflict, "EMAIL_TAKEN", err.Error())
	case errors.Is(err, ErrAlreadyProvisioned):
		writeError(w, r, http.StatusConflict, "ALREADY_PROVISIONED", err.Error())
	case errors.Is(err, ErrNotFound):
		writeError(w, r, http.StatusNotFound, "NOT_FOUND", err.Error())
	default:
		slog.ErrorContext(r.Context(), "employee handler error", "error", err)
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
