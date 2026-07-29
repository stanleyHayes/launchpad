# LaunchPad

Enterprise employee onboarding platform (multi-tenant SaaS).

## Stack

| Layer | Choice |
| --- | --- |
| API | Go modular monolith (hexagonal ports/adapters) |
| Datastore | MongoDB (swap-ready behind repository ports) |
| Cache / sessions | Redis |
| Auth | Custom Go (email/password, JWT + Redis sessions) |
| Frontends | Next.js + TypeScript + Tailwind |
| Monorepo | pnpm workspaces + Go modules |

## Backend architecture

Domain packages under `internal/<domain>` own types, use cases, HTTP handlers, and **repository ports** (`Repository` / `UserRepository` / `SessionRepository`). Persistence drivers live in adapters:

- `internal/<domain>/mongo` — MongoDB adapters implementing domain ports
- `internal/auth/redis` — Redis session adapter

`internal/app` is the composition root: it constructs adapters and injects them into services. To switch MongoDB for Postgres later, add `internal/<domain>/postgres` implementations of the same ports and change wiring in `app` — handlers and services stay unchanged.

## Product surfaces

1. `apps/marketing-web` — public marketing site (`:3000`)
2. `apps/platform-admin-web` — LaunchPad internal ops (`:3001`)
3. `apps/organization-admin-web` — customer HR/IT/managers (`:3002`)
4. `apps/employee-web` — employee onboarding journey (`:3003`)
5. `apps/api` — Go REST API (`:8080`)

## Quick start

```bash
cp .env.example .env
# optional: set PLATFORM_OWNER_EMAIL / PLATFORM_OWNER_PASSWORD to bootstrap platform staff
make up
make deps
make migrate-indexes
make api
```

In separate terminals:

```bash
make marketing        # :3000
make platform-admin   # :3001
make org-admin        # :3002
make employee         # :3003
```

Or run the API in Docker with Mongo/Redis:

```bash
docker compose up --build
```

## Current delivery

- **Phase 0–1:** auth, orgs, departments/roles, employees, journeys, assignments, approvals, notifications, org-admin + employee portals
- **Phase 2:** platform staff, tenant ops, leads, billing, feature flags, support tickets, CMS (publish + public marketing pages), analytics, platform + org admin consoles
- **Phase 3 (in progress):**
  - **Knowledge management** — per-tenant document lifecycle (draft → approved → indexed → archived) with a human approval gate before AI indexing and access scopes
  - **AI onboarding assistant** — grounded, citation-backed retrieval over approved documents: refuses when no source is found, cites only sources the answer used, logs feedback (PRD §16.2). Embeddings + vector search live in MongoDB (Atlas Vector Search–ready); answering uses the Claude Messages API (`ANTHROPIC_API_KEY`, `ASSISTANT_MODEL`) with an offline extractive fallback so it runs with zero external keys
  - **Tenant branding** — organizations set their own brand colors; the shared UI exposes an overridable brand token layer (`brandCssVars`) over the default "Ocean Depths" palette
  - **SCIM 2.0 provisioning** — a customer's IdP (Okta/Entra) creates, updates, and deactivates users in its tenant via `/scim/v2/Users`, authenticated by a per-organization bearer token (hashed at rest, issued once via `/organizations/current/scim-token`). Provisioning creates the account + active membership; deactivation/`DELETE` suspends membership. Every request is tenant-scoped by the token
  - **Slack/Teams notifications** — onboarding notifications are delivered to a tenant's configured Slack/Teams incoming webhook (best-effort, alongside the in-app notification). Webhook URLs are set by managers via `/notifications/channels` and validated against an https host allowlist (plus a no-redirect HTTP client) to prevent SSRF
  - **SSO login (OIDC)** — employees sign in through their company IdP via the OIDC Authorization Code flow (`/auth/sso/{orgSlug}/start` → `/auth/sso/callback`). Per-org config is manager-set (`/organizations/current/sso`); the id_token is cryptographically verified (RS256 via the IdP's JWKS, plus issuer/audience/expiry/nonce), and the verified email maps to an already-provisioned member — SSO authenticates, it does not create accounts
  - **HRIS sync** — pulls a tenant's employee directory from their HR system (BambooHR provider adapter; pluggable) and stores it as a per-org snapshot plus a sync-run history. Manager-set config + manual trigger (`/organizations/current/hris`, `.../hris/sync`, `.../hris/directory`); a failed sync preserves the previous snapshot
- **Phase 4 (in progress):**
  - **HRIS → employees apply** — a manager applies the stored HRIS directory snapshot into `employees` (`POST /organizations/current/hris/apply`). For each entry whose work email is not already present it creates a minimal *invited* employee, mapping the entry's department **name** to this tenant's department **id** (unmapped names create the employee with no department). The operation is org-scoped, idempotent (already-present emails are skipped), records an audit event, and returns a `created / skipped / failed` summary with a bounded list of per-entry failures
  - **SCIM 2.0 group provisioning** — a customer's IdP manages groups in its tenant via `/scim/v2/Groups` (create, get, list/filter, replace, PATCH add/remove/replace members + rename, delete), authenticated by the same per-organization SCIM bearer token as `/Users`. Group members reference SCIM user resource ids in the same tenant; member ids that don't resolve to a SCIM user in the org are dropped (prevents cross-tenant references and tolerates users synced after the group). Every operation is org-scoped and privileged changes are audited
  - **SCIM pagination** — both `/scim/v2/Users` and `/scim/v2/Groups` list endpoints support SCIM `startIndex`/`count` pagination and report the true `totalResults` (via a tenant-scoped count), so an IdP can page through directories larger than one response
  - **Passwordless invite flow** — a manager invites a new member (`POST /organizations/current/invitations`, role `hr_admin`/`employee`, never owner): this creates a *pending* account (no password, can't log in) with an org membership and returns a single-use, expiring invitation token. The invitee redeems it publicly (`POST /auth/invitations/accept`) to set their own password, activate the account, and receive a session. Tokens are stored hashed (SHA-256) with a TTL index; issue and accept are idempotent/retryable so a transient failure can't orphan an account. This unblocks bulk provisioning without admin-set passwords
  - **Security hardening** (from an adversarial audit) — SSRF-hardened outbound HTTP for tenant-controlled URLs (HRIS/SSO) via a no-redirect client that refuses non-public addresses at dial time (`pkg/safehttp`); per-IP rate limiting on public auth endpoints; a request-body size cap; `http.Server` read/write/idle timeouts; a manager gate on the audit-log endpoint; chat webhook URLs masked in responses and redacted from logs; and an OIDC `email_verified` check
- **Next:** Phase 4 continues — HRIS manager-graph, bulk HRIS→employees provisioning (via invitations), and enterprise hardening

See [LaunchPad_Complete_PRD_and_Build_Spec.md](./LaunchPad_Complete_PRD_and_Build_Spec.md), [CLAUDE.md](./CLAUDE.md), and [AGENTS.md](./AGENTS.md).
