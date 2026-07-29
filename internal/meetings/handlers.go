package meetings

import (
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"launchpad/internal/audit"
	"launchpad/internal/sso"
	"launchpad/pkg/httpx"
	"launchpad/pkg/security"
)

// Handler exposes meeting and calendar-connection HTTP endpoints.
type Handler struct {
	svc         *Service
	audit       *audit.Service
	states      sso.StateStore
	oauth       map[string]*OAuthClient
	orgAdminURL string
}

// WithOAuth enables browser authorization-code flows for calendar providers.
func (h *Handler) WithOAuth(
	states sso.StateStore,
	google, microsoft *OAuthClient,
	orgAdminURL string,
) *Handler {
	h.states = states
	h.oauth = map[string]*OAuthClient{ProviderGoogle: google, ProviderMicrosoft: microsoft}
	h.orgAdminURL = strings.TrimRight(orgAdminURL, "/")

	return h
}

const calendarOAuthStateTTL = 10 * time.Minute

// HandleStartCalendarOAuth redirects an authenticated administrator to the
// selected provider. The opaque, one-time state binds the callback to tenant
// and actor without exposing either in the browser.
func (h *Handler) HandleStartCalendarOAuth(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(w, r)
	if !ok {
		return
	}
	provider := chi.URLParam(r, "provider")
	client := h.oauth[provider]
	if h.states == nil || client == nil || !client.Configured() {
		writeMeetingError(w, r, ErrInvalidState)
		return
	}
	state, err := security.NewRefreshToken()
	if err != nil {
		writeMeetingError(w, r, err)
		return
	}
	if err := h.states.Save(r.Context(), state, sso.AuthState{
		OrganizationID: principal.OrganizationID,
		ActorUserID:    principal.UserID,
		Provider:       provider,
	}, calendarOAuthStateTTL); err != nil {
		writeMeetingError(w, r, err)
		return
	}
	authorizationURL, err := client.AuthorizationURL(state)
	if err != nil {
		writeMeetingError(w, r, err)
		return
	}
	writeJSON(w, r, http.StatusOK, map[string]string{"authorizationUrl": authorizationURL})
}

// HandleCalendarOAuthCallback consumes the one-time state, exchanges the code,
// validates the resulting credential, and stores the encrypted token pair.
func (h *Handler) HandleCalendarOAuthCallback(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	if query.Get("error") != "" {
		h.redirectCalendarResult(w, r, "", "denied")
		return
	}
	if h.states == nil {
		writeMeetingError(w, r, ErrInvalidState)
		return
	}
	state, err := h.states.Consume(r.Context(), query.Get("state"))
	if err != nil || state.ActorUserID == "" || !validProvider(state.Provider) {
		writeMeetingError(w, r, ErrInvalidState)
		return
	}
	client := h.oauth[state.Provider]
	if client == nil {
		writeMeetingError(w, r, ErrInvalidState)
		return
	}
	token, err := client.Exchange(r.Context(), query.Get("code"))
	if err != nil {
		writeMeetingError(w, r, err)
		return
	}
	conn, err := h.svc.ConnectCalendarOAuth(
		r.Context(), state.OrganizationID, state.ActorUserID, state.Provider, token,
	)
	if err != nil {
		writeMeetingError(w, r, err)
		return
	}
	h.redirectCalendarResult(w, r, conn.Provider, "connected")
}

func (h *Handler) redirectCalendarResult(w http.ResponseWriter, r *http.Request, provider, result string) {
	if h.orgAdminURL == "" {
		writeJSON(w, r, http.StatusOK, map[string]string{"provider": provider, "result": result})
		return
	}
	target, _ := url.Parse(h.orgAdminURL + "/integrations")
	query := target.Query()
	query.Set("calendar", result)
	if provider != "" {
		query.Set("provider", provider)
	}
	target.RawQuery = query.Encode()
	http.Redirect(w, r, target.String(), http.StatusFound)
}

// NewHandler constructs a meetings Handler.
func NewHandler(svc *Service, auditSvc *audit.Service) *Handler {
	return &Handler{svc: svc, audit: auditSvc}
}

// HandleListMine lists the caller's own meetings.
func (h *Handler) HandleListMine(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(w, r)
	if !ok {
		return
	}

	items, err := h.svc.ListMine(r.Context(), principal.OrganizationID, principal.UserID)
	if err != nil {
		writeMeetingError(w, r, err)

		return
	}

	writeJSON(w, r, http.StatusOK, toResponses(items))
}

// HandleCancelMine cancels one of the caller's own scheduled meetings.
func (h *Handler) HandleCancelMine(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(w, r)
	if !ok {
		return
	}

	meeting, err := h.svc.CancelMine(
		r.Context(),
		principal.OrganizationID,
		principal.UserID,
		chi.URLParam(r, "meetingID"),
	)
	if err != nil {
		writeMeetingError(w, r, err)

		return
	}

	if !h.record(w, r, principal, "meeting.cancelled", meeting, nil) {
		return
	}

	writeJSON(w, r, http.StatusOK, meeting.ToResponse())
}

// HandleList lists meetings visible to the caller (team-scoped for managers),
// optionally filtered by ?status=.
func (h *Handler) HandleList(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(w, r)
	if !ok {
		return
	}

	items, err := h.svc.ListForUser(
		r.Context(),
		principal.OrganizationID,
		principal.UserID,
		r.URL.Query().Get("status"),
	)
	if err != nil {
		writeMeetingError(w, r, err)

		return
	}

	writeJSON(w, r, http.StatusOK, toResponses(items))
}

// HandleCreate schedules a meeting with the caller as organizer.
func (h *Handler) HandleCreate(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(w, r)
	if !ok {
		return
	}

	var body struct {
		Title              string `json:"title"`
		Type               string `json:"type"`
		AttendeeEmployeeID string `json:"attendeeEmployeeId"`
		StartsAt           string `json:"startsAt"`
		DurationMin        int    `json:"durationMin"`
		Location           string `json:"location"`
		NotesLink          string `json:"notesLink"`
	}
	if err := httpx.DecodeJSON(r, &body); err != nil {
		writeError(w, r, http.StatusBadRequest, "INVALID_JSON", "Request body is invalid")

		return
	}

	startsAt, err := time.Parse(time.RFC3339, body.StartsAt)
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "INVALID_INPUT", "startsAt must be an RFC3339 timestamp")

		return
	}

	meeting, err := h.svc.Create(r.Context(), CreateInput{
		OrganizationID:     principal.OrganizationID,
		Title:              body.Title,
		Type:               body.Type,
		OrganizerUserID:    principal.UserID,
		AttendeeEmployeeID: body.AttendeeEmployeeID,
		StartsAt:           startsAt,
		DurationMin:        body.DurationMin,
		Location:           body.Location,
		NotesLink:          body.NotesLink,
	})
	if err != nil {
		writeMeetingError(w, r, err)

		return
	}

	if !h.record(w, r, principal, "meeting.created", meeting, map[string]any{"type": meeting.Type}) {
		return
	}

	writeJSON(w, r, http.StatusCreated, meeting.ToResponse())
}

// HandleComplete records the outcome of a scheduled meeting.
func (h *Handler) HandleComplete(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(w, r)
	if !ok {
		return
	}

	var body struct {
		NoShow    bool   `json:"noShow"`
		NotesLink string `json:"notesLink"`
	}
	if err := httpx.DecodeJSON(r, &body); err != nil {
		writeError(w, r, http.StatusBadRequest, "INVALID_JSON", "Request body is invalid")

		return
	}

	meeting, err := h.svc.Complete(r.Context(), CompleteInput{
		OrganizationID: principal.OrganizationID,
		MeetingID:      chi.URLParam(r, "meetingID"),
		NoShow:         body.NoShow,
		NotesLink:      body.NotesLink,
	})
	if err != nil {
		writeMeetingError(w, r, err)

		return
	}

	if !h.record(w, r, principal, "meeting.completed", meeting, map[string]any{"status": meeting.Status}) {
		return
	}

	writeJSON(w, r, http.StatusOK, meeting.ToResponse())
}

// HandleCancel cancels a scheduled meeting (manager/admin action).
func (h *Handler) HandleCancel(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(w, r)
	if !ok {
		return
	}

	meeting, err := h.svc.Cancel(
		r.Context(),
		principal.OrganizationID,
		chi.URLParam(r, "meetingID"),
	)
	if err != nil {
		writeMeetingError(w, r, err)

		return
	}

	if !h.record(w, r, principal, "meeting.cancelled", meeting, nil) {
		return
	}

	writeJSON(w, r, http.StatusOK, meeting.ToResponse())
}

func (h *Handler) HandleReschedule(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(w, r)
	if !ok {
		return
	}
	var body struct {
		StartsAt    time.Time `json:"startsAt"`
		DurationMin int       `json:"durationMin"`
		Location    string    `json:"location"`
	}
	if err := httpx.DecodeJSON(r, &body); err != nil {
		writeError(w, r, http.StatusBadRequest, "INVALID_JSON", "Request body is invalid")
		return
	}
	meeting, err := h.svc.Reschedule(r.Context(), RescheduleInput{
		OrganizationID: principal.OrganizationID, MeetingID: chi.URLParam(r, "meetingID"),
		StartsAt: body.StartsAt, DurationMin: body.DurationMin, Location: body.Location,
	})
	if err != nil {
		writeMeetingError(w, r, err)
		return
	}
	if !h.record(w, r, principal, "meeting.rescheduled", meeting, nil) {
		return
	}
	writeJSON(w, r, http.StatusOK, meeting.ToResponse())
}

// HandleGetCalendarConnection returns the tenant's calendar connection
// (masked, no token) or 404 when none is connected.
func (h *Handler) HandleGetCalendarConnection(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(w, r)
	if !ok {
		return
	}

	provider := r.URL.Query().Get("provider")
	if provider == "" {
		provider = ProviderGoogle
	}
	conn, err := h.svc.GetCalendarConnectionForProvider(r.Context(), principal.OrganizationID, provider)
	if err != nil {
		writeMeetingError(w, r, err)

		return
	}

	writeJSON(w, r, http.StatusOK, conn)
}

// HandleConnectCalendar validates and stores a provider access token.
func (h *Handler) HandleConnectCalendar(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(w, r)
	if !ok {
		return
	}

	var body struct {
		Token    string `json:"token"`
		Provider string `json:"provider"`
	}
	if err := httpx.DecodeJSON(r, &body); err != nil {
		writeError(w, r, http.StatusBadRequest, "INVALID_JSON", "Request body is invalid")

		return
	}

	if body.Provider == "" {
		body.Provider = ProviderGoogle
	}
	conn, err := h.svc.ConnectCalendarProvider(r.Context(), principal.OrganizationID, principal.UserID, body.Provider, body.Token)
	if err != nil {
		writeMeetingError(w, r, err)

		return
	}

	if !h.recordCalendar(w, r, principal, "calendar.connected", map[string]any{
		"provider":      conn.Provider,
		"accountHandle": conn.AccountHandle,
	}) {
		return
	}

	writeJSON(w, r, http.StatusOK, conn)
}

// HandleDisconnectCalendar removes the tenant's calendar connection.
func (h *Handler) HandleDisconnectCalendar(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(w, r)
	if !ok {
		return
	}

	provider := r.URL.Query().Get("provider")
	if provider == "" {
		provider = ProviderGoogle
	}
	if err := h.svc.DisconnectCalendarProvider(r.Context(), principal.OrganizationID, provider); err != nil {
		writeMeetingError(w, r, err)

		return
	}

	if !h.recordCalendar(w, r, principal, "calendar.disconnected", map[string]any{
		"provider": provider,
	}) {
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// record writes an audit event for a meeting action. Failures abort the
// response with a 500, matching the requests module's audit policy. It
// returns false when the response was already written.
func (h *Handler) record(
	w http.ResponseWriter,
	r *http.Request,
	principal security.Principal,
	action string,
	meeting Meeting,
	metadata map[string]any,
) bool {
	orgID := meeting.OrganizationID
	if err := h.audit.Record(
		r.Context(),
		&orgID,
		principal.UserID,
		action,
		"meeting",
		meeting.ID,
		metadata,
	); err != nil {
		slog.ErrorContext(r.Context(), "audit meeting action failed", "action", action, "error", err)
		writeError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to record audit event")

		return false
	}

	return true
}

// recordCalendar writes an audit event for a calendar connection action.
func (h *Handler) recordCalendar(
	w http.ResponseWriter,
	r *http.Request,
	principal security.Principal,
	action string,
	metadata map[string]any,
) bool {
	orgID := principal.OrganizationID
	if err := h.audit.Record(
		r.Context(),
		&orgID,
		principal.UserID,
		action,
		"calendar_connection",
		ProviderGoogle,
		metadata,
	); err != nil {
		slog.ErrorContext(r.Context(), "audit calendar action failed", "action", action, "error", err)
		writeError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to record audit event")

		return false
	}

	return true
}

func requirePrincipal(w http.ResponseWriter, r *http.Request) (security.Principal, bool) {
	principal, ok := security.PrincipalFromContext(r.Context())
	if !ok {
		writeError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")

		return security.Principal{}, false
	}

	return principal, true
}

func writeMeetingError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, ErrInvalidInput):
		writeError(w, r, http.StatusBadRequest, "INVALID_INPUT", err.Error())
	case errors.Is(err, ErrInvalidCredential):
		writeError(w, r, http.StatusBadRequest, "INVALID_CREDENTIAL", "Calendar provider rejected the credential")
	case errors.Is(err, ErrNotFound):
		writeError(w, r, http.StatusNotFound, "NOT_FOUND", "Meeting or calendar connection not found")
	case errors.Is(err, ErrForbidden):
		writeError(w, r, http.StatusForbidden, "FORBIDDEN", "You may not access this meeting")
	case errors.Is(err, ErrInvalidState):
		writeError(w, r, http.StatusConflict, "INVALID_STATE", err.Error())
	default:
		slog.ErrorContext(r.Context(), "meetings handler error", "error", err)
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
