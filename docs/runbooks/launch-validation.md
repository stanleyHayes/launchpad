# Launch validation runbook

Preflight and provider smoke checks to run before a LaunchPad deploy is
declared live. Every step lists the exact command and the evidence that
counts as a pass. Record results in the Evidence Template at the bottom and
attach it to the release ticket.

Related: `docs/runbooks/README.md` (index provisioning, SCIM rotation,
trusted forwarded headers), `docs/QA-UAT-checklist.md` (full UAT pass).

## 1. Preflight (static gates)

Run from the repo root on the release commit:

```bash
pnpm launch:check
# or, where services/credentials are intentionally absent (local dry-run):
node scripts/check-launch-gates.mjs --warn-only
```

Gates evaluated (`scripts/check-launch-gates.mjs`):

| Gate | Pass evidence |
| --- | --- |
| env: required variables | `[PASS]` — `JWT_SECRET`, `MONGODB_URI`, `REDIS_URL` set (in env or `.env`; values are never printed) |
| env: watch variables | `[PASS]` or `[WARN]` — `ANTHROPIC_API_KEY`, `ENCRYPTION_KEY` (watch only, never blocks) |
| go build | `[PASS]` — `go build ./...` exit 0 |
| go test | `[PASS]` — `go test ./...` exit 0 |
| golangci-lint | `[PASS]` — `golangci-lint run ./...` reports 0 issues; `[SKIP]` only if the binary is absent (note it in the evidence) |
| pnpm test | `[PASS]` — vitest suite green |
| migrate-indexes compiles | `[PASS]` — `go vet ./scripts/migrate_indexes` exit 0 |

**Pass evidence:** final line reads `0 failed` and the script exits 0. Any
`FAIL` blocks launch; a `WARN` must be acknowledged by name in the evidence.

## 2. Database indexes

Indexes are ensured at API startup and independently via the shared registry
(`app.MongoIndexers`). Before opening traffic on a fresh environment:

```bash
make migrate-indexes   # go run ./scripts/migrate_indexes
```

**Pass evidence:** exit 0 against the target `MONGODB_URI`. CI runs the same
command in the `migrations` job (`.github/workflows/ci.yml`) against a
throwaway MongoDB with the docker-compose credentials.

Ops note (review L-7): on deployments that existed before 2026-07-27, drop
the old non-unique journeys step-position index once; the unique replacement
is created by the registry.

## 3. Provider smoke checks

### 3.1 Anthropic (AI assistant)

```bash
ANTHROPIC_API_KEY=... pnpm smoke:anthropic
```

The script makes a **metadata call only** (`GET /v1/models`) — Anthropic has
no test/live key distinction, so the script never creates a completion and
**no tokens are billed**. The key is read from the environment (or `.env`)
and is never printed.

**Pass evidence:** `PASS smoke:anthropic — GET /v1/models returned 200`.
A 401/403 means the key is invalid or revoked — rotate before launch.
If the key is absent the assistant feature is degraded, not down: decide
explicitly whether to launch with `/assistant/ask` returning 503.

### 3.2 OIDC SSO (per configured identity provider)

```bash
pnpm smoke:oidc -- https://<tenant>.<idp>.com
```

Read-only: one unauthenticated GET of the discovery document. Asserts https
plus the fields LaunchPad's verifier needs (`issuer`, `jwks_uri`,
`token_endpoint`).

**Pass evidence:** `PASS smoke:oidc` with the printed `issuer` matching the
issuer configured in the org's SSO settings. If the document's `issuer`
differs from the URL you passed, update the SSO config to the printed value.
Run once per IdP that has a configured org.

### 3.3 MongoDB / Redis

Covered dynamically by the readiness probe rather than a script:

```bash
curl -fsS https://<api-host>/readyz
```

**Pass evidence:** HTTP 200 — `/readyz` pings both MongoDB and Redis (2s
budget). `/healthz` is liveness only and proves nothing about dependencies.

## 4. Security spot checks

After deploy, from outside the network:

```bash
curl -sI https://<api-host>/healthz        # expect X-Content-Type-Options: nosniff
curl -sI https://<marketing-host>/         # expect CSP + Referrer-Policy headers
curl -H 'X-Forwarded-For: 1.2.3.4' https://<api-host>/healthz
```

**Pass evidence:** security headers present on API and frontends; the request
log for the last call shows the real client IP, **not** `1.2.3.4` (ingress
must overwrite forwarded headers — see "Deployment: trusted forwarded
headers" in `docs/runbooks/README.md`).

## 5. Rollback note

If any blocking gate fails after deploy:

1. Roll back to the previous release (ArgoCD: `argocd app rollback` / revert
   the release commit; see `infra/argocd`). The application has no destructive
   migration step — indexes created by `migrate-indexes` are additive and
   safe to leave behind on rollback.
2. Do **not** drop indexes or collections as part of rollback.
3. Seeded defaults (plans, feature flags) are create-only since 2026-07-27,
   so a rollback/restart does not clobber admin edits.
4. Record the failing gate, its output, and the rollback action in the
   evidence template; launch stays blocked until the gate passes on a
   re-deploy.

## 6. Evidence Template

Copy into the release ticket and fill in:

```
Launch validation evidence
Date:                YYYY-MM-DD HH:MM UTC
Environment:         staging | production
Commit:              <full SHA>
Operator:            <name>

Preflight (pnpm launch:check):
  Result:            PASS | FAIL (--warn-only used? yes/no)
  Failed gates:      <none | list>
  Warnings:          <none | list, acknowledged by>

Indexes (make migrate-indexes):   PASS | FAIL | N/A (pre-existing)

Provider smokes:
  Anthropic:         PASS | FAIL | N/A (launching without assistant)
  OIDC IdP(s):       <issuer URL> PASS | FAIL   (one line per IdP)
  readyz:            PASS | FAIL

Security spot checks:
  API headers:       PASS | FAIL
  Frontend headers:  PASS | FAIL
  Forwarded-IP:      PASS | FAIL

UAT:                 docs/QA-UAT-checklist.md completed (link/attachment)

Open blockers:       <none | list with owners>
Sign-off:            <name, role, timestamp>
```
