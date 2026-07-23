# LaunchPad

Enterprise employee onboarding platform (multi-tenant SaaS).

## Stack

| Layer | Choice |
| --- | --- |
| API | Go modular monolith |
| Datastore | MongoDB |
| Cache / sessions | Redis |
| Auth | Custom Go (email/password, JWT + Redis sessions) |
| Frontends | Next.js + TypeScript + Tailwind |
| Monorepo | pnpm workspaces + Go modules |

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
- **Phase 2 (in progress):** platform staff, tenant ops, leads, billing plans/subscriptions, feature flags, support tickets, platform + org admin consoles

See [LaunchPad_Complete_PRD_and_Build_Spec.md](./LaunchPad_Complete_PRD_and_Build_Spec.md), [CLAUDE.md](./CLAUDE.md), and [AGENTS.md](./AGENTS.md).
