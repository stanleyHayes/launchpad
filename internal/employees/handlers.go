package employees

import (
	"encoding/csv"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"launchpad/internal/audit"
	"launchpad/internal/entitlements"
	"launchpad/pkg/httpx"
	"launchpad/pkg/security"
)

const maxEmployeeCSVBytes = 2 << 20

// Handler exposes employee HTTP endpoints.
type Handler struct {
	svc         *Service
	audit       *audit.Service
	provisioner *Provisioner
}

// NewHandler constructs a Handler.
func NewHandler(svc *Service, auditSvc *audit.Service, accounts AccountCreator, members MemberAdder) *Handler {
	return &Handler{svc: svc, audit: auditSvc, provisioner: NewProvisioner(svc, accounts, members)}
}

// HandleList lists employees for the current organization.
func (h *Handler) HandleList(w http.ResponseWriter, r *http.Request) {
	principal, ok := security.PrincipalFromContext(r.Context())
	if !ok {
		writeError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")

		return
	}

	limit := int64(0)
	offset := int64(0)

	if raw := r.URL.Query().Get("limit"); raw != "" {
		parsed, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			writeError(w, r, http.StatusBadRequest, "INVALID_LIMIT", "limit must be an integer")

			return
		}

		limit = parsed
	}

	if raw := r.URL.Query().Get("offset"); raw != "" {
		parsed, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			writeError(w, r, http.StatusBadRequest, "INVALID_OFFSET", "offset must be an integer")

			return
		}

		offset = parsed
	}

	items, err := h.svc.List(r.Context(), principal.OrganizationID, offset, limit)
	if err != nil {
		slog.ErrorContext(r.Context(), "list employees failed", "error", err)
		writeError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to list employees")

		return
	}

	writeJSON(w, r, http.StatusOK, items)
}

func (h *Handler) HandleMyContacts(w http.ResponseWriter, r *http.Request) {
	principal, ok := security.PrincipalFromContext(r.Context())
	if !ok {
		writeError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
		return
	}
	contacts, err := h.svc.ContactsForUser(r.Context(), principal.OrganizationID, principal.UserID)
	if err != nil {
		writeEmployeeError(w, r, err)
		return
	}
	writeJSON(w, r, http.StatusOK, contacts)
}

func (h *Handler) HandleMyProfile(w http.ResponseWriter, r *http.Request) {
	principal, ok := security.PrincipalFromContext(r.Context())
	if !ok {
		writeError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
		return
	}
	employee, err := h.svc.ProfileForUser(r.Context(), principal.OrganizationID, principal.UserID)
	if err != nil {
		writeEmployeeError(w, r, err)
		return
	}
	writeJSON(w, r, http.StatusOK, employee)
}

func (h *Handler) HandleUpdateMyProfile(w http.ResponseWriter, r *http.Request) {
	principal, ok := security.PrincipalFromContext(r.Context())
	if !ok {
		writeError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
		return
	}
	var body struct {
		MobilePhone string `json:"mobilePhone"`
	}
	if err := httpx.DecodeJSON(r, &body); err != nil {
		writeError(w, r, http.StatusBadRequest, "INVALID_JSON", "Request body is invalid")
		return
	}
	employee, err := h.svc.UpdateSelf(
		r.Context(), principal.OrganizationID, principal.UserID, body.MobilePhone,
	)
	if err != nil {
		writeEmployeeError(w, r, err)
		return
	}
	if err := h.audit.Record(
		r.Context(), &principal.OrganizationID, principal.UserID,
		"employee.profile_updated", "employee", employee.ID, nil,
	); err != nil {
		slog.ErrorContext(r.Context(), "audit employee profile update", "error", err)
		writeError(w, r, http.StatusInternalServerError, "AUDIT_FAILED", "Profile updated but audit recording failed")
		return
	}
	writeJSON(w, r, http.StatusOK, employee)
}

// HandleCreate creates an employee.
// Authorization is enforced by the route-level RequirePermission
// (employees.create); the handler only needs the authenticated principal.
func (h *Handler) HandleCreate(w http.ResponseWriter, r *http.Request) {
	principal, ok := security.PrincipalFromContext(r.Context())
	if !ok {
		writeError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")

		return
	}

	var body struct {
		EmployeeNumber    string `json:"employeeNumber"`
		FirstName         string `json:"firstName"`
		LastName          string `json:"lastName"`
		WorkEmail         string `json:"workEmail"`
		MobilePhone       string `json:"mobilePhone"`
		JobRoleID         string `json:"jobRoleId"`
		DepartmentID      string `json:"departmentId"`
		ManagerEmployeeID string `json:"managerEmployeeId"`
		BuddyEmployeeID   string `json:"buddyEmployeeId"`
		Team              string `json:"team"`
		Location          string `json:"location"`
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
		MobilePhone:       body.MobilePhone,
		JobRoleID:         body.JobRoleID,
		DepartmentID:      body.DepartmentID,
		ManagerEmployeeID: body.ManagerEmployeeID,
		BuddyEmployeeID:   body.BuddyEmployeeID,
		Team:              body.Team,
		Location:          body.Location,
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
		// The employee is already persisted; a failed audit write must not
		// turn a committed change into a reported failure.
		slog.ErrorContext(r.Context(), "audit employee create failed", "error", err)
	}

	writeJSON(w, r, http.StatusCreated, employee)
}

// HandleImportCSV imports employees from a header-based CSV. Valid rows are
// committed independently and row errors are returned without rolling back
// successful employees.
func (h *Handler) HandleImportCSV(w http.ResponseWriter, r *http.Request) {
	principal, ok := security.PrincipalFromContext(r.Context())
	if !ok {
		writeError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxEmployeeCSVBytes)
	reader := csv.NewReader(r.Body)
	reader.FieldsPerRecord = -1
	header, err := reader.Read()
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "INVALID_CSV", "CSV header is required")
		return
	}
	columns := make(map[string]int, len(header))
	for index, name := range header {
		columns[strings.ToLower(strings.TrimSpace(name))] = index
	}
	for _, required := range []string{"firstname", "lastname", "workemail", "startdate"} {
		if _, exists := columns[required]; !exists {
			writeError(w, r, http.StatusBadRequest, "INVALID_CSV", "Required CSV columns: firstName,lastName,workEmail,startDate")
			return
		}
	}
	type rowError struct {
		Row     int    `json:"row"`
		Message string `json:"message"`
	}
	result := struct {
		Created int        `json:"created"`
		Failed  int        `json:"failed"`
		Errors  []rowError `json:"errors"`
	}{Errors: make([]rowError, 0)}
	value := func(record []string, name string) string {
		index, exists := columns[name]
		if !exists || index >= len(record) {
			return ""
		}
		return strings.TrimSpace(record[index])
	}
	for row := 2; row <= 1001; row++ {
		record, readErr := reader.Read()
		if errors.Is(readErr, io.EOF) {
			break
		}
		if readErr != nil {
			result.Failed++
			result.Errors = append(result.Errors, rowError{Row: row, Message: "invalid CSV row"})
			continue
		}
		startDate, parseErr := time.Parse(time.DateOnly, value(record, "startdate"))
		if parseErr != nil {
			result.Failed++
			result.Errors = append(result.Errors, rowError{Row: row, Message: "startDate must be YYYY-MM-DD"})
			continue
		}
		_, createErr := h.svc.Create(r.Context(), principal.OrganizationID, CreateInput{
			FirstName: value(record, "firstname"), LastName: value(record, "lastname"),
			WorkEmail: value(record, "workemail"), StartDate: startDate,
			MobilePhone:    value(record, "mobilephone"),
			EmployeeNumber: value(record, "employeenumber"), Team: value(record, "team"),
			Location: value(record, "location"),
		})
		if createErr != nil {
			result.Failed++
			result.Errors = append(result.Errors, rowError{Row: row, Message: createErr.Error()})
			continue
		}
		result.Created++
	}
	if extra, readErr := reader.Read(); readErr == nil && len(extra) > 0 {
		writeError(w, r, http.StatusBadRequest, "CSV_LIMIT_EXCEEDED", "CSV is limited to 1,000 employees")
		return
	}
	orgID := principal.OrganizationID
	if err := h.audit.Record(r.Context(), &orgID, principal.UserID, "employees.imported", "employee", "", map[string]any{
		"created": result.Created, "failed": result.Failed,
	}); err != nil {
		slog.ErrorContext(r.Context(), "audit employee import failed", "error", err)
	}
	status := http.StatusOK
	if result.Failed > 0 {
		status = http.StatusMultiStatus
	}
	writeJSON(w, r, status, result)
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

// HandleUpdate updates mutable employee fields (name, number, references,
// buddy, status). Setting status to offboarded records an
// employee.offboarded audit event.
// Authorization: route-level RequirePermission (employees.update).
func (h *Handler) HandleUpdate(w http.ResponseWriter, r *http.Request) {
	principal, ok := security.PrincipalFromContext(r.Context())
	if !ok {
		writeError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")

		return
	}

	var body struct {
		FirstName         *string `json:"firstName"`
		LastName          *string `json:"lastName"`
		EmployeeNumber    *string `json:"employeeNumber"`
		MobilePhone       *string `json:"mobilePhone"`
		JobRoleID         *string `json:"jobRoleId"`
		DepartmentID      *string `json:"departmentId"`
		ManagerEmployeeID *string `json:"managerEmployeeId"`
		BuddyEmployeeID   *string `json:"buddyEmployeeId"`
		Team              *string `json:"team"`
		Location          *string `json:"location"`
		Status            *string `json:"status"`
	}
	if err := httpx.DecodeJSON(r, &body); err != nil {
		writeError(w, r, http.StatusBadRequest, "INVALID_JSON", "Request body is invalid")

		return
	}

	employeeID := chi.URLParam(r, "employeeID")

	updated, err := h.svc.Update(r.Context(), principal.OrganizationID, employeeID, UpdateInput{
		FirstName:         body.FirstName,
		LastName:          body.LastName,
		EmployeeNumber:    body.EmployeeNumber,
		MobilePhone:       body.MobilePhone,
		JobRoleID:         body.JobRoleID,
		DepartmentID:      body.DepartmentID,
		ManagerEmployeeID: body.ManagerEmployeeID,
		BuddyEmployeeID:   body.BuddyEmployeeID,
		Team:              body.Team,
		Location:          body.Location,
		Status:            body.Status,
	})
	if err != nil {
		writeEmployeeError(w, r, err)

		return
	}

	action := "employee.updated"
	if body.Status != nil && updated.Status == statusOffboarded {
		action = "employee.offboarded"
	}

	orgID := principal.OrganizationID
	if err := h.audit.Record(
		r.Context(),
		&orgID,
		principal.UserID,
		action,
		"employee",
		updated.ID,
		map[string]any{"status": updated.Status},
	); err != nil {
		// The update is already persisted; a failed audit write must not
		// turn a committed change into a reported failure.
		slog.ErrorContext(r.Context(), "audit employee update failed", "error", err)
	}

	writeJSON(w, r, http.StatusOK, updated)
}

// HandleProvisionAccess creates login credentials for an invited employee.
// Authorization: route-level RequirePermission (employees.update).
func (h *Handler) HandleProvisionAccess(w http.ResponseWriter, r *http.Request) {
	principal, ok := security.PrincipalFromContext(r.Context())
	if !ok {
		writeError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")

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

	updated, userID, err := h.provisioner.Provision(
		r.Context(),
		principal.OrganizationID,
		chi.URLParam(r, "employeeID"),
		body.DisplayName,
		body.Password,
	)
	if err != nil {
		switch {
		case errors.Is(err, errProvisionAccount):
			slog.ErrorContext(r.Context(), "provision employee account failed", "error", err)
			writeError(w, r, http.StatusBadRequest, "PROVISION_FAILED", "Unable to provision employee account")
		case errors.Is(err, errProvisionMember):
			slog.ErrorContext(r.Context(), "provision employee membership failed", "error", err)
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
		// Provisioning is already committed; a failed audit write must not
		// turn a committed change into a reported failure.
		slog.ErrorContext(r.Context(), "audit employee provision failed", "error", err)
	}

	writeJSON(w, r, http.StatusOK, updated)
}

func writeEmployeeError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, entitlements.ErrLimitExceeded):
		writeError(w, r, http.StatusConflict, "PLAN_LIMIT_EXCEEDED", err.Error())
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
