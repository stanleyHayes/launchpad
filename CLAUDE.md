# CLAUDE.md — LaunchPad

Rules for AI coding agents working in this repository.

## Product

LaunchPad is a multi-tenant employee onboarding SaaS. Read `LaunchPad_Complete_PRD_and_Build_Spec.md` before large changes.

## Stack

- Go modular monolith (`apps/api`, `internal/*`, `pkg/*`)
- MongoDB primary datastore (`pkg/mongo` Connect/Close — no connection logic in domain code)
- Redis for sessions, cache, rate limits (`pkg/redis`)
- Custom Go auth (no Clerk/Auth0 for core auth)
- Logging via standard library `log/slog` only (no zap/logrus/zerolog)
- Next.js apps under `apps/*-web`
- Shared UI in `packages/ui`
- Lint with `golangci-lint` (`make lint`) — configuration enables all linters by default

## Non-negotiables

1. Implement one bounded module at a time.
2. Update OpenAPI contracts under `contracts/openapi` before new endpoints.
3. Every tenant query must filter by `organizationId`.
4. Authorize every protected operation; never rely on UI alone.
5. Write audit events for privileged and critical actions.
6. Add tests with each feature.
7. Never commit secrets; use `.env` / CI secrets.
8. Prefer modular monolith growth; do not split services without an ADR in `docs/decisions`.
9. Idempotency keys for retryable writes.
10. Keep changes scoped — no unrelated refactors.
11. Never ignore errors (`_ = err` is forbidden). Handle, wrap, or return them.
12. Keep `main` thin: open connections via package helpers, then call `app.Run`.

## Layout

```text
apps/api            Go HTTP entrypoint
internal/<domain>   Domain modules (auth, organizations, audit, ...)
pkg/                Shared libraries
packages/ui         Shared React design system
contracts/          OpenAPI, events, webhooks
docs/               Product, architecture, design, ADRs
migrations/         MongoDB index bootstrap scripts
```

## Auth

- Email/password with argon2id or bcrypt
- Short-lived JWT access tokens
- Refresh/session tokens stored in Redis (hashed)
- Organization context on authenticated requests

## Definition of done

Requirements met, tests pass, tenant isolation verified, authorization verified, audit verified, docs/contracts updated when behavior changes.
