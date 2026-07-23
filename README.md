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
- **Next (Phase 3):** knowledge ingestion, AI assistant, SSO/SCIM, Slack/Teams, HRIS sync

See [LaunchPad_Complete_PRD_and_Build_Spec.md](./LaunchPad_Complete_PRD_and_Build_Spec.md), [CLAUDE.md](./CLAUDE.md), and [AGENTS.md](./AGENTS.md).
