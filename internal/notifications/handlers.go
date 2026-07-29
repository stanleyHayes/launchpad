package notifications

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"launchpad/internal/audit"
	"launchpad/internal/organizations"
	"launchpad/pkg/httpx"
	"launchpad/pkg/security"
)

// Handler exposes notification HTTP endpoints.
type Handler struct {
	svc   *Service
	audit *audit.Service
}

func (h *Handler) WithAudit(service *audit.Service) *Handler {
	h.audit = service
	return h
}

// NewHandler constructs a notification Handler.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// HandleList lists notifications for the authenticated user.
func (h *Handler) HandleList(w http.ResponseWriter, r *http.Request) {
	principal, ok := security.PrincipalFromContext(r.Context())
	if !ok {
		writeError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")

		return
	}

	items, err := h.svc.ListForUser(r.Context(), principal.OrganizationID, principal.UserID)
	if err != nil {
		writeNotificationError(w, r, err)

		return
	}

	writeJSON(w, r, items)
}

// HandleMarkRead marks the authenticated user's notification as read.
func (h *Handler) HandleMarkRead(w http.ResponseWriter, r *http.Request) {
	principal, ok := security.PrincipalFromContext(r.Context())
	if !ok {
		writeError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")

		return
	}

	notification, err := h.svc.MarkRead(
		r.Context(),
		principal.OrganizationID,
		principal.UserID,
		chi.URLParam(r, "id"),
	)
	if err != nil {
		writeNotificationError(w, r, err)

		return
	}

	writeJSON(w, r, notification)
}

// HandleGetChannels returns the tenant's outbound channel configuration.
func (h *Handler) HandleGetChannels(w http.ResponseWriter, r *http.Request) {
	principal, ok := requireManager(w, r)
	if !ok {
		return
	}

	config, err := h.svc.GetChannelConfig(r.Context(), principal.OrganizationID)
	if err != nil {
		writeNotificationError(w, r, err)

		return
	}

	writeJSON(w, r, ToChannelStatus(config))
}

// HandleSetChannels updates the tenant's Slack/Teams webhook URLs.
func (h *Handler) HandleSetChannels(w http.ResponseWriter, r *http.Request) {
	principal, ok := requireManager(w, r)
	if !ok {
		return
	}

	var body struct {
		SlackWebhookURL *string `json:"slackWebhookUrl"`
		TeamsWebhookURL *string `json:"teamsWebhookUrl"`
	}
	if err := httpx.DecodeJSON(r, &body); err != nil {
		writeError(w, r, http.StatusBadRequest, "INVALID_JSON", "Request body is invalid")

		return
	}

	config, err := h.svc.SetChannelConfig(r.Context(), principal.OrganizationID, SetChannelsInput{
		SlackWebhookURL: body.SlackWebhookURL,
		TeamsWebhookURL: body.TeamsWebhookURL,
	})
	if err != nil {
		writeNotificationError(w, r, err)

		return
	}

	writeJSON(w, r, ToChannelStatus(config))
}

func (h *Handler) HandlePlatformDeliveries(w http.ResponseWriter, r *http.Request) {
	items, err := h.svc.ListDeliveries(r.Context())
	if err != nil {
		writeNotificationError(w, r, err)
		return
	}
	writeJSON(w, r, items)
}

func (h *Handler) HandlePlatformRetryDelivery(w http.ResponseWriter, r *http.Request) {
	principal, ok := security.PrincipalFromContext(r.Context())
	if !ok {
		writeError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
		return
	}
	delivery, err := h.svc.RetryDelivery(r.Context(), chi.URLParam(r, "deliveryID"))
	if err != nil {
		writeNotificationError(w, r, err)
		return
	}
	if h.audit != nil {
		if err := h.audit.Record(r.Context(), nil, principal.UserID, "notification_delivery.retried", "notification_delivery", delivery.ID, map[string]any{"status": delivery.Status}); err != nil {
			slog.ErrorContext(r.Context(), "audit delivery retry", "error", err)
		}
	}
	writeJSON(w, r, delivery)
}

// requireManager keeps the legacy owner/hr_admin gate on the tenant channel
// configuration: the channels routes declare no permission and outbound
// webhook URLs are too sensitive to open to every org member. The /me
// notification endpoints above stay principal-scoped.
func requireManager(w http.ResponseWriter, r *http.Request) (security.Principal, bool) {
	principal, ok := security.PrincipalFromContext(r.Context())
	if !ok {
		writeError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")

		return security.Principal{}, false
	}

	if !organizations.CanManageOrganization(principal.RoleCode) {
		writeError(w, r, http.StatusForbidden, "FORBIDDEN", "Insufficient permissions")

		return security.Principal{}, false
	}

	return principal, true
}

func writeNotificationError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, ErrInvalidInput):
		writeError(w, r, http.StatusBadRequest, "INVALID_INPUT", err.Error())
	case errors.Is(err, ErrNotFound):
		writeError(w, r, http.StatusNotFound, "NOT_FOUND", "Notification not found")
	default:
		slog.ErrorContext(r.Context(), "notification handler error", "error", err)
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
