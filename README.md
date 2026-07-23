# LaunchPad

Enterprise employee onboarding platform (multi-tenant SaaS).

## Stack (v0.3)

| Layer | Choice |
| --- | --- |
| API | Go modular monolith |
| Datastore | MongoDB |
| Cache / sessions | Redis |
| Auth | Custom Go (email/password, JWT + Redis sessions) |
| Frontends | Next.js + TypeScript + Tailwind |
| Monorepo | pnpm workspaces + Go modules |

## Product surfaces

1. `apps/marketing-web` — public marketing site
2. `apps/platform-admin-web` — LaunchPad internal ops
3. `apps/organization-admin-web` — customer HR/IT/managers
4. `apps/employee-web` — employee onboarding journey
5. `apps/api` — Go REST API

## Quick start

```bash
cp .env.example .env
make up
make deps
make migrate-indexes
make api
# separate terminal
make marketing
```

- API: http://localhost:8080/healthz
- Marketing: http://localhost:3000

## Phase 0 scope

Foundation includes monorepo, CI, custom auth, organization model, audit events, design system starter, product design notes, and marketing site shell.

See [LaunchPad_Complete_PRD_and_Build_Spec.md](./LaunchPad_Complete_PRD_and_Build_Spec.md), [CLAUDE.md](./CLAUDE.md), and [AGENTS.md](./AGENTS.md).
