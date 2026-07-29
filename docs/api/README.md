# API documentation

The LaunchPad REST API is defined by a single OpenAPI contract:

- **`contracts/openapi/openapi.yaml`** — the source of truth for every route,
  request body, and the shared response/error envelope.

## Reading the contract

- Base URL (local): `http://localhost:8080/api/v1` (`make up && make api`).
- Authentication: `Authorization: Bearer <accessToken>` for user routes
  (`bearerAuth`); per-organization SCIM tokens for `/scim/v2/*` (`scimBearer`).
- Every response is wrapped in the standard envelope
  (`pkg/httpx.Envelope`): `{ "data": ... }` on success,
  `{ "error": { "code", "message" } }` on failure — see the `AuthSession`,
  `Employee`, `JourneyAssignment`, and `ErrorEnvelope` schemas in the spec.
- Health: `GET /healthz` (liveness), `GET /readyz` (pings MongoDB and Redis).

Render it with any OpenAPI viewer, e.g. `npx @redocly/cli preview-docs
contracts/openapi/openapi.yaml`, or import it into Postman/Insomnia.

## Related contracts

- `contracts/events/audit-events.md` — audit-event action catalog (the
  de-facto domain events).
- `contracts/webhooks/chat-webhooks.md` — outbound Slack/Teams webhook
  payloads.

## Conventions when changing the API

Per `AGENTS.md`, update the contract **before** implementing: add the route,
request schema, and response schemas to `openapi.yaml`, then implement the
handler. New fields that carry secrets must be write-only in the schema (see
`SSOConfig.clientSecret`, `HRISConfig.apiToken`, `NotificationChannels`).
