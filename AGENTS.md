# AGENTS.md — LaunchPad

Operating procedures for human and AI agents.

## Workflow

1. Read the PRD and related ADRs for the domain you touch.
2. Create or update contracts before implementation.
3. Implement domain logic in `internal/<domain>`.
4. Wire routes in `apps/api`.
5. Add Mongo indexes in `migrations` / `scripts/migrate_indexes` when collections change.
6. Add unit or integration tests.
7. Update docs if behavior or APIs change.

## Git conventions

When a Jira key exists:

- Branch: `feature/PROJECTKEY-123-short-name`
- Commit: `PROJECTKEY-123 implement short description`
- PR title: `PROJECTKEY-123 Short description`

Otherwise use clear conventional messages focused on why.

## Frontend rules

- Domain UI under `features/`
- Shared primitives in `packages/ui`
- Validate API responses at trust boundaries (Zod)
- Permission checks in navigation and API

## Backend rules

- Handlers have no business logic
- Services do not import HTTP frameworks
- Repository interfaces live in the domain package
- Mongo documents do not leak as API responses
- Cross-module calls use explicit interfaces

## Local commands

```bash
make up
make migrate-indexes
make api
make marketing
make test
make lint
```

Linting uses `golangci-lint` with all linters enabled by default (see `.golangci.yml`). Logging must use `log/slog` only.

## Security

Never expose API tokens, client data, database credentials, or cloud secrets. Store secrets in environment variables only.
