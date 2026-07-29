# Outbound chat webhooks — Slack and Microsoft Teams

LaunchPad delivers tenant notifications to chat tools via **outbound** HTTP
POSTs to per-organization incoming-webhook URLs. These are not inbound
webhooks LaunchPad receives; they are credentials LaunchPad holds and posts
to. Source of truth: `internal/notifications/webhook/dispatcher.go`.

## Delivery model

- Trigger: a notification created for a user in the tenant is also fanned out
  to every configured chat channel (best-effort; delivery failures are logged,
  not retried, and never fail the originating API request).
- Destination URLs are set per organization via `PUT /api/v1/notifications/channels`
  (managers only). They are validated before storage: **https only**, host must
  be on the Slack/Teams allowlist. Empty string clears a channel.
- Request: `POST <webhook URL>` with `Content-Type: application/json`.
- Client behavior: 5s timeout, **no redirect following** (a redirect is treated
  as a failed delivery), response body discarded after 64 KiB. Any non-2xx
  status is a failure. Failures across channels are joined and logged with the
  webhook URL stripped (`redactURL`), so the secret URL never reaches logs.

The notification input has two fields, both plain strings: `title` and `body`
(`internal/notifications/types.go` `Notification`).

## Slack payload

Built by `slackPayload` (`dispatcher.go:116-120`).

```json
{
  "text": "*<notification title>*\n<notification body>"
}
```

Single field: `text` — the title bolded with Slack `*...*` markup, a newline,
then the body.

## Microsoft Teams payload

Built by `teamsPayload` (`dispatcher.go:122-130`). Legacy MessageCard format,
as expected by Teams incoming webhook connectors.

```json
{
  "@type": "MessageCard",
  "@context": "https://schema.org/extensions",
  "summary": "<notification title>",
  "title": "<notification title>",
  "text": "<notification body>"
}
```

## Security notes

- Webhook URLs are bearer credentials: anyone holding one can post to the
  channel. They are stored per tenant and are **never returned by the API** —
  reads expose only `slackConfigured` / `teamsConfigured`
  (`ChannelStatus`).
- The dispatcher does not follow redirects so an allowlisted host cannot
  bounce delivery to an internal address (SSRF bypass).
