# Codebase Review — 2026-07-27

Full-repo review of LaunchPad covering architecture & structure, code quality &
correctness, security, and tests & docs, per the conventions in `AGENTS.md`.

## Executive summary

LaunchPad is in good shape for its stage. The hexagonal architecture conventions in
`AGENTS.md` are genuinely followed — domain-owned repository interfaces with
compile-time adapter assertions, explicit cross-module ports, disciplined sentinel-error
mapping, `log/slog` everywhere, and Zod validation at the frontend trust boundary are
all real, not aspirational. `go test ./...` passes and `golangci-lint run ./...`
(with effectively all linters enabled) reports **0 issues**.

The review found **0 critical, 8 high, 25 medium, and 37 low** findings. The themes
worth attention, in order:

1. **Trust of client input in the most sensitive places** — quiz scores are accepted
   from the request body (H-1); three portals store 7-day refresh tokens in
   `localStorage` with no CSP (H-5); the SSO auth callback persists tokens from the URL
   hash without validation (M-20).
2. **A systematic form bug in the frontend** — `event.currentTarget.reset()` inside
   deferred async closures throws on 12 forms across three apps, so users see a failure
   message *after* a successful mutation (H-4). This is the single most user-visible bug.
3. **Deployment-time footguns** — the rate limiter keys on `RemoteAddr` with no `RealIP`
   middleware, so behind a load balancer all tenants share one 20-req/min bucket on the
   public auth endpoints (H-3); seed-on-boot clobbers admin plan/feature-flag edits on
   every restart (H-2); repo-wide port mismatches break the documented quickstart (M-16).
4. **Test gaps on the highest-risk paths** — billing and the frontend have zero tests,
   and Register/Login/Refresh/Logout are untested (H-6, H-7, H-8), while less critical
   modules (SSO, SCIM, invitations) are well tested.
5. **Non-atomic multi-step writes** that leave orphans and report committed changes as
   failures (M-5, M-6) — a retry after a reported "failure" fails again with 409.

## Resolution addendum (2026-07-27, same day)

All 70 findings were addressed in a coordinated fix effort on the same day as this
review: **69 FIXED, 1 PARTIAL (L-31)**, 0 deferred. Each finding carries an inline
resolution note below. Verification after the fixes:

| Check | Result |
| --- | --- |
| `go build ./...` | PASS (exit 0) |
| `go test ./...` | PASS (exit 0, 29 packages `ok`) |
| `golangci-lint run ./...` | PASS — `0 issues.` |
| `pnpm test` (vitest) | PASS (8/8) |
| `pnpm --filter … build` × 4 apps | PASS (marketing, employee, organization-admin, platform-admin) |

Follow-ups that are intentionally not part of the fixes (product/ops decisions):

- Quiz answer keys: H-1's fix makes quiz steps reject client scores; completing them
  needs a server-side answer-key model (PRD "assessments" — see `docs/product/prd-status.md`).
- Lift the shared portal auth-shell into `packages/ui` (L-31 remainder).
- "Load more" cursor UI on the platform-admin leads page (backend paginates as of L-9).
- Drop the old non-unique journeys step-position index on existing deployments (L-7).
- Redis-backed distributed rate limiter when running multiple replicas (L-21).
- Optional `@vitest/coverage` and per-app frontend tests beyond the packages suite (H-7).

---
## Verification results

| Check | Command | Result |
| --- | --- | --- |
| Go tests | `go test ./...` | **PASS** (exit 0). 22 packages with tests all `ok`; 36 packages (mostly `*/mongo` adapters and `pkg/*`) have no test files. |
| Lint | `golangci-lint run ./...` (v2.12.2, config per `.golangci.yml`) | **PASS** — `0 issues.` |

Limitations:

- The frontend was **not** built, type-checked, or linted during this review (no
  `lint`/`test` scripts exist for most apps; builds would have mutated the workspace).
  Frontend findings are from code reading only.
- Mongo-backed stores (`internal/*/mongo`) have no test coverage; their correctness was
  assessed by reading, not execution.
- No dynamic/security testing (no fuzzing, no dependency vulnerability scan, no runtime
  probing) was performed.

## Coverage

Every non-test Go file in `internal/` (all 21 non-empty domains), `pkg/`, `apps/api`,
and `scripts/migrate_indexes` was read; all 146 Go files and all 74 TS/TSX files were in
scope. `contracts/openapi/openapi.yaml` (1,469 lines) was compared route-by-route
against `internal/app/app.go`. Root configs (`docker-compose.yml`, `go.mod`,
`.golangci.yml`, `.env.example`, `.gitignore`, CI workflow) and all of `docs/`, the
PRDs, and `AGENTS.md` were reviewed. `internal/marketing/`, `internal/platformadmin/`,
`migrations/`, `contracts/events/`, `contracts/webhooks/`, `seeds/`, `docs/api/`,
`docs/runbooks/`, and `docs/security/` are empty directories (see L-36, M-18, M-25).

---

## High findings

### H-1 Quiz scores are client-supplied and trusted — any employee can pass any quiz

> ✅ **FIXED 2026-07-27:** Quiz steps now reject client-supplied scores (`ErrInvalidInput`) until server-side answer keys exist; regression test added in `internal/assignments`.

`internal/assignments/service.go:233` (also `internal/assignments/handlers.go:168-184`).
`HandleCompleteStep` accepts `score` from the request body; `applyCompleteStepInput`
copies it onto the step, and `finalizeStepCompletion` (service.go:242-254) passes any
quiz where `*step.Score >= 70`. Any employee can complete any quiz step by POSTing
`{"score": 100}`.
**Fix:** score quizzes server-side against stored correct answers, or at minimum ignore
client-supplied scores for quiz steps.

### H-2 Boot-time seeding silently reverts admin changes on every restart

> ✅ **FIXED 2026-07-27:** `SeedDefaults` (billing + featureflags) is now create-only — existing records are skipped, lookup errors surfaced; tests added.

`internal/billing/service.go:87-96` and `internal/featureflags/service.go:58-67`,
invoked from `internal/app/app.go:542,740-748`. `SeedDefaults` fetches each built-in
plan/flag, preserves only `CreatedAt`, then unconditionally upserts the hardcoded
defaults via full `ReplaceOne` (`internal/featureflags/mongo/store.go:54-63`). A
platform admin who changes the `growth` plan price or enables the `ai_assistant` flag
sees the change silently overwritten on the next deploy. Non-`ErrNotFound` `GetPlan`/
`GetFlag` errors are also swallowed (service.go:59-62).
**Fix:** seed create-only — skip the upsert when the record already exists; only treat
`ErrNotFound` as "absent" and surface other errors.

### H-3 Rate limiter keys on `RemoteAddr`, but no `RealIP` middleware is installed

> ✅ **FIXED 2026-07-27:** Own `middleware.RealIP` installed ahead of the rate limiter (replaces deprecated `chimw.RealIP`); trusted-LB caveat documented in `docs/runbooks/README.md`.

`pkg/middleware/middleware.go:96-103` + `internal/app/app.go:834-838,858-865`. The
limiter's own doc comment (middleware.go:19-23) says it must be "fronted with a
trusted-proxy RealIP layer when deployed behind a load balancer", but `newRouter`
installs only `RequestID`, `RequestLogger`, `Recoverer`, and `CORS`. Behind the LB in
`infra/kubernetes`, every client's `RemoteAddr` is the LB IP, so all tenants share one
20-req/min bucket on `/auth/login`, `/auth/register`, `/auth/refresh`, and invitation
acceptance — a global self-DoS on the auth path.
**Fix:** add `chimw.RealIP` (configured for trusted proxies) ahead of the rate limiter,
or make the limiter's key function configurable; add a deployment note to
`docs/runbooks/`.

### H-4 `event.currentTarget.reset()` inside deferred closures throws after successful mutations

> ✅ **FIXED 2026-07-27:** Form element captured synchronously at all 12 sites; repo-wide sweep confirms zero `currentTarget.reset` remain; all four apps build.

`apps/organization-admin-web/app/employees/page.tsx:72` (also lines 91, 118, 139, 161,
184), `apps/organization-admin-web/app/journeys/page.tsx:64`,
`apps/organization-admin-web/app/journeys/[journeyID]/page.tsx:75`,
`apps/organization-admin-web/app/support/page.tsx:64`,
`apps/platform-admin-web/app/feature-flags/page.tsx:85`,
`apps/platform-admin-web/app/billing/page.tsx:80`, `apps/marketing-web/app/demo/page.tsx:41`.
`SyntheticEvent.currentTarget` is only valid during dispatch; by the time the
`startTransition`/async body runs it is `null`, so `.reset()` throws a `TypeError`.
Because the call sits between the successful `await` and the success handling, the
`catch` fires and the UI shows a failure message ("Unable to create department"…)
*even though the mutation succeeded*; `setMessage`, `reload()`, and on
`journeys/page.tsx:66` the `router.push` never run. On the public demo form, users see
failure after their lead was actually created, inviting duplicate submissions.
**Fix:** capture the form element synchronously (`const formEl = event.currentTarget;`
before `startTransition`) and call `formEl.reset()` in the success path, or switch to
controlled inputs as the CMS page already does.

### H-5 Session tokens (including 7-day refresh tokens) stored in `localStorage` on all three portals

> ✅ **FIXED 2026-07-27:** API sets `HttpOnly; SameSite=Lax` cookies (`Secure` outside local) on login/register/refresh/invite-accept/SSO-callback and clears them on logout; portals no longer store tokens; api-client sends `credentials:'include'` with single-flight refresh; `Authenticate` accepts the cookie fallback.

`apps/employee-web/lib/session.ts:4-7`, `apps/organization-admin-web/lib/session.ts:4-7`,
`apps/platform-admin-web/lib/session.ts:4-7`. Any XSS in these apps exfiltrates both
tokens; the refresh token is valid 7 days (`JWT_REFRESH_TTL=168h` in `.env.example`).
Compounded by the absence of a CSP on every app (M-23).
**Fix:** move session tokens to `HttpOnly; Secure; SameSite=Strict` cookies set by the
API (or a BFF/Route Handler), keeping only non-sensitive profile state client-side; at
minimum add a strict CSP and drop the refresh token from browser storage.

### H-6 Billing has zero tests

> ✅ **FIXED 2026-07-27:** `internal/billing/billing_test.go` added — plan validation, subscription assign/change, SeedDefaults, full error table.

`internal/billing/` contains no `*_test.go`. The billing service (351 lines: plan
catalog, subscription assignment, platform plan/subscription operations exposed at
`/billing/*` and `/platform/plans|subscriptions`) is the money-critical path.
**Fix:** add service-level tests with an in-memory repository fake (the pattern already
used in `internal/sso/sso_test.go`) covering plan validation, subscription
assign/change, and error paths.

### H-7 The entire frontend has no tests and no test runner

> ✅ **FIXED 2026-07-27:** vitest added at workspace root; Zod-boundary + ui primitive tests (8 passing); `pnpm test` wired into scripts and `.github/workflows/ci.yml`.

Root `package.json:4-8`. Zero `*.test.*`/`*.spec.*` files under `apps/` or `packages/`;
app scripts contain only `dev`/`build`/`lint`; no vitest/jest/playwright dependency.
`AGENTS.md` mandates Zod validation at trust boundaries and permission checks in
navigation — nothing verifies either.
**Fix:** add vitest to the workspace with at least schema-validation tests for the Zod
boundary code and a `test` script wired into CI.

### H-8 Core auth flows (Register, Login, Refresh, Logout) have no direct tests

> ✅ **FIXED 2026-07-27:** `internal/auth/service_test.go` added — Register/Login/Refresh/Logout, rotation, reuse rejection, theft detection, no membership-existence leak.

`internal/auth/service.go:73,102,182,229`. Auth tests cover only the invitation
lifecycle (`invitations_test.go:227-416`, well done) and refresh-token string parsing.
Credential verification, token rotation/reuse, and logout revocation — the highest-risk
code in the repo — are untested.
**Fix:** extend the existing in-memory fakes (`fakeUsers`, `fakeSessions`) to cover
Login success/failure, Refresh rotation and reuse rejection, and Register validation.

---

## Medium findings

### Architecture & correctness (backend)

**M-1 Analytics under-reports headcount above 100 employees.**

> ✅ **FIXED 2026-07-27:** `Count` added to the employees port/service/mongo store; analytics uses a real count query.
`internal/analytics/service.go:34,50` + `internal/analytics/types.go:35`.
`OnboardingSummary` lists employees with `analyticsListCap = 100`, then sets
`EmployeeCount = len(employeeItems)` — silently capped.
*Fix:* use a dedicated count query instead of `len()` of a limited list.

**M-2 Any authenticated org member can list and read every employee's assignments.**

> ✅ **FIXED 2026-07-27:** `HandleList` manager-gated; `HandleGet`/`HandleListSteps` restricted to managers or the owning employee (403 via `GetForActor`/`ListStepsForActor`).
`internal/assignments/handlers.go:93,128,145`. `HandleList`, `HandleGet`, and
`HandleListSteps` use `requirePrincipal` with no role or ownership check while the
service returns org-wide data; an ordinary employee can enumerate all colleagues'
onboarding assignments and step submissions.
*Fix:* gate `HandleList` behind `requireManager` (as `HandleListApprovals` already
does); restrict `HandleGet`/`HandleListSteps` to managers or the owning employee.

**M-3 Approval steps are hardcoded to the assignment creator as approver.**

> ✅ **FIXED 2026-07-27:** Approver resolved from the employee's `ManagerEmployeeID` with actor fallback; tests added.
`internal/assignments/service.go:550-560,311`. `ApproverUserID: actorUserID`;
`DecideApproval` rejects anyone else; the employee's actual manager
(`ManagerEmployeeID`) is never consulted, and if the assigning manager leaves, pending
approvals are permanently undecidable.
*Fix:* resolve the approver from the employee's manager (with fallback), or allow any
org manager to decide.

**M-4 Mongo documents leak as API responses (AGENTS.md violation).**

> ✅ **FIXED 2026-07-27:** Response DTOs + `ToResponse` in assignments/cms/billing/audit/organizations/support; `auth` `Me`+`Result` use `OrganizationPublic`; platform handlers map via `ToResponse`; JSON field names byte-identical (marshal-compare tests).
`internal/assignments/types.go:40-83`, `internal/cms/types.go:27-37`,
`internal/billing/types.go:38-59`, `internal/audit/audit.go:10-19`,
`internal/organizations/types.go:39-49`, `internal/support/types.go:29-40`; handlers
serialize these structs (with `bson:"_id"` tags) directly to clients, and
`auth.Service.Me` (`internal/auth/service.go:258-263`) embeds the raw
`organizations.Organization` document. Nothing sensitive leaks today, but the public API
contract is coupled to persistence layout and any future sensitive field is exposed by
default. The intended pattern already exists (`auth.UserPublic`, `sso.Config`'s
`json:"-"`, notifications' `ChannelStatus` DTO).
*Fix:* introduce response DTOs / `ToResponse` mappers at the handler boundary.

**M-5 Multi-document writes lack atomicity; committed changes are reported as failures.**

> ✅ **FIXED 2026-07-27:** `Register` creates the org before the user; audit writes are best-effort after commit in all affected handlers; assignments notification failure now logs and returns success instead of 500.
`internal/assignments/service.go:119-125,367-388` — `Assign` persists assignment +
steps + approvals across three collections with no transaction; after a successful
persist, a notification failure returns 500, and the client's retry then fails with
`ErrAlreadyAssigned` (409). `internal/auth/service.go:79-87` — `Register` creates the
user before the organization; org-creation failure leaves an orphan user and
re-registration returns `EMAIL_TAKEN`. Same pattern where a mutation succeeds but
`audit.Record` fails → 500: `internal/departments/handlers.go:131-145`,
`internal/employees/handlers.go:135-149,227-241`,
`internal/journeys/handlers.go:189-203`, `internal/knowledge/handlers.go:263-276`,
`internal/featureflags/handlers.go:189-203` — a retrying client gets 409 for a resource
it was told failed.
*Fix:* wrap multi-writes in Mongo transactions where supported; treat post-commit
notification/audit failure as non-fatal (log, still return success); reorder
Register to create the org before the user.

**M-6 Provisioning orchestration lives in HTTP handlers and leaves unrecoverable orphans.**

> ✅ **FIXED 2026-07-27:** Orchestration moved into services (`employees` provisioner, `organizations.InviteMember`) with retry-reuse compensation; `ErrAlreadyMember` sentinel; handlers thin.
`internal/employees/handlers.go:246-274` (`provisionEmployeeAccess`: create account →
add member → link, no compensation; a mid-flow failure plus retry fails permanently
with `EMAIL_TAKEN` → 400 `PROVISION_FAILED`). Same shape in
`internal/organizations/handlers.go:154-221` (`HandleInviteMember`, including the
hr_admin rule — business logic in a handler). Also violates "handlers have no business
logic".
*Fix:* move orchestration into a service/use-case layer with compensation (delete or
reuse the created account on retry).

**M-7 `LinkUser` is a check-then-act race; the atomic alternative is dead code.**

> ✅ **FIXED 2026-07-27:** `LinkUser` uses the atomic `ProvisionAccess` conditional update; concurrent provisions get `ErrAlreadyProvisioned`.
`internal/employees/service.go:227-246` does `GetByID` → check `UserID == ""` → full
`Update`. Two concurrent provisions both pass; last write wins and one caller's account
is silently discarded. The store already implements the atomic conditional
`ProvisionAccess` (`internal/employees/mongo/store.go:185-209`), exposed via
`Service.ProvisionAccess` (`service.go:134`) — but nothing in production calls it.
*Fix:* have `LinkUser` use `repo.ProvisionAccess`.

**M-8 HRIS `Sync` maps infrastructure errors to "not configured".**

> ✅ **FIXED 2026-07-27:** `hris.Sync` maps only not-found/disabled to `ErrNotConfigured`; infrastructure errors surface as 500s.
`internal/hris/service.go:88-91` — any `GetByOrganization` error (Mongo outage, decode
failure) becomes `ErrNotConfigured` → 404 `HRIS_NOT_CONFIGURED`
(`internal/hris/handlers.go:151-152`), sending operators chasing config instead of the
real outage.
*Fix:* only translate `ErrNotFound` to `ErrNotConfigured`; wrap and return other errors;
check `config.Enabled` separately.

**M-9 Tenant secrets stored unencrypted at rest in MongoDB.**

> ✅ **FIXED 2026-07-27:** AES-256-GCM envelope encryption (`enc:v1:`) for HRIS tokens, SSO client secrets, and webhook URLs; `ENCRYPTION_KEY` env var; plaintext passthrough for legacy rows; documented in `docs/security/`.
`internal/hris/types.go:37` (BambooHR `APIToken`), `internal/sso/types.go:33` (OIDC
`ClientSecret`), `internal/notifications/mongo/store.go:168-179` (Slack/Teams webhook
URLs — bearer credentials per `internal/notifications/types.go:46`). All are correctly
masked from API responses (`json:"-"`), but DB dumps/backups expose every tenant's
third-party credentials. `pkg/security` has no encryption helper.
*Fix:* envelope-encrypt these fields (key from env/KMS), or document the accepted risk
and reliance on MongoDB encryption-at-rest in `docs/security/`.

**M-10 `scripts/migrate_indexes` has drifted from the app's index list.**

> ✅ **FIXED 2026-07-27:** Shared `app.MongoIndexers` registry (24 index sets) now used by both startup and `scripts/migrate_indexes`.
`scripts/migrate_indexes/main.go:61-79` ensures 14 index sets; startup
(`internal/app/app.go:793-822`) ensures 24. Missing from the script: auth-invitations,
notification channels, knowledge, assistant-chunks, assistant-interactions,
scim-users/tokens/groups, sso, hris-config, hris-state. `make migrate-indexes` is the
documented provisioning step (`Makefile:29-30`, AGENTS.md step 5); where only the
script runs, the SCIM unique indexes backing the race-safety of
`scim.Service.CreateUser` would never exist.
*Fix:* share a single indexer registry between `internal/app` and the script (or drop
the script).

**M-11 Server `WriteTimeout` equals the Anthropic client timeout.**

> ✅ **FIXED 2026-07-27:** `writeTimeout` raised to 120s (above the 60s Anthropic client timeout).
`internal/app/app.go:75` (60s) vs `internal/assistant/anthropic/client.go:32,70` (60s).
`WriteTimeout` bounds the whole handler (embeddings + vector search + LLM call) while
the LLM call alone may take 60s — a slow-but-successful response gets its connection
killed before it can be written.
*Fix:* raise `writeTimeout` to 90–120s or drop the Anthropic timeout to ~45s.

**M-12 `PROVISION_FAILED` responses echo raw wrapped driver errors.**

> ✅ **FIXED 2026-07-27:** Static `PROVISION_FAILED` message; wrapped error logged server-side.
`internal/employees/handlers.go:217`, rooted at
`internal/auth/mongo/user_store.go:51` (`fmt.Errorf("insert user: %w", err)`). The full
chain — potentially Mongo driver error text with topology/server details — is written
verbatim to the client via `err.Error()`. The adjacent case at line 219 already does
the right thing (static message); this is an outlier, not systemic.
*Fix:* respond with a static message for `errProvisionAccount`; log the wrapped error.

### Security

**M-13 Logout and deprovisioning do not invalidate access tokens.**

> ✅ **FIXED 2026-07-27:** `Authenticate` verifies `sessionId` via a `SessionChecker` (Redis `EXISTS`); fail-closed; 503 on session-store outage.
`pkg/middleware/middleware.go:168-203`. `Authenticate` verifies only JWT
signature/expiry; it never checks that the `sessionId` claim still exists in Redis.
`Logout` (`internal/auth/service.go:229-235`) deletes the session and SCIM
`SetActive(false)` (`internal/scim/service.go:185`) suspends the membership, but a
captured access token — or one held by a just-deprovisioned employee — works until
expiry (default 15m, `pkg/config/config.go:18`). Role changes likewise lag.
*Fix:* verify session existence in Redis in `Authenticate` (cheap GET), and/or keep
`AccessTTL` short and document the revocation window.

**M-14 `/assistant/ask` is unrate-limited despite calling a paid LLM and scanning the whole corpus in memory.**

> ✅ **FIXED 2026-07-27:** Assistant routes rate-limited (10 req/min per client IP).
`internal/app/app.go:1054` + `internal/assistant/mongo/store.go:122-135`. Only public
auth/lead endpoints are rate-limited (`app.go:858-865`). Any authenticated member can
fire unlimited requests that fan out to Anthropic — a direct cost-amplification vector —
and each `Ask` loads the tenant's entire chunk collection (embeddings included) into
memory and ranks in-process (documented as a deliberate tradeoff in the file header,
but unbounded).
*Fix:* per-user/per-org rate limit on the assistant group; cap the candidate set or
move to `$vectorSearch` as the header comment suggests.

**M-15 SCIM provisioning tokens never expire and there is only one per org.**

> ✅ **FIXED 2026-07-27:** SCIM tokens carry 90-day expiry enforced in `ResolveOrganization` (no oracle); use audit-logged; `expiresAt` in response and OpenAPI.
`internal/scim/service.go:54-74`, `internal/scim/mongo/store.go:213-245`. Tokens are
32-byte random stored SHA-256-hashed (good), but there is no TTL and rotation is manual
only; a leaked token grants indefinite full user/group provisioning for that tenant.
*Fix:* token expiry/rotation metadata enforced in `ResolveOrganization`; audit-log
token use.

### Contracts, config, dev experience

**M-16 Repo-wide port mismatches break the documented quickstart.**

> ✅ **FIXED 2026-07-27:** Compose publishes 27017/6379; dev+start scripts on 3000-3003; code fallbacks and docs aligned; quickstart works end-to-end.
`docker-compose.yml:9,22` publishes Mongo on host **27018** and Redis on **6380**, but
`.env.example:7,11` points at 27017/6379 — so README's quickstart
(`cp .env.example .env && make up && make api`) cannot connect. Independently, the four
frontend dev scripts bind **3300–3303** (`apps/*/package.json:6`) while `CORS_ORIGINS`,
documented `NEXT_PUBLIC_*_URL`s, and in-code fallbacks (e.g.
`apps/marketing-web/app/signup/signup-form.tsx:14`) all point at 3000–3003 — dev
origins are CORS-blocked and signup redirects to a dead port. `Makefile:10-13` help
text also advertises the wrong (3000-range) ports.
*Fix:* pick one scheme and align compose, `.env.example`, dev scripts, CORS origins,
fallback constants, and Makefile help.

**M-17 OpenAPI spec is missing most request bodies and nearly all response schemas.**

> ✅ **FIXED 2026-07-27:** 18 `requestBody` schemas plus response schemas (auth session, employee, assignment family, error envelope, SCIM `expiresAt`) added, field-verified against handlers.
`contracts/openapi/openapi.yaml`. ~15 endpoints whose handlers decode JSON have no
`requestBody` — verified examples: `POST /employees/{employeeID}/provision` (spec
908-921; handler requires `{password, displayName}` at
`internal/employees/handlers.go:197-205`), `POST /departments`, `POST /job-roles`,
`POST /journeys` + `/steps`, `POST /assignments`, `POST /step-assignments/{id}/complete`,
`POST /approvals/{id}/decide`, `POST /support/tickets`, `POST /platform/support/tickets/{id}/status`,
`POST/PATCH /platform/feature-flags…`, `POST/PATCH /platform/plans…`. Except for a
handful of `$ref`s, every response is a bare `description:` with no content schema —
the `pkg/httpx.Envelope` error shape isn't in the spec at all — so the contract can't
drive client generation or the Zod-at-the-boundary validation AGENTS.md mandates.
(Route *coverage* is genuinely complete: zero undocumented endpoints, zero phantom
paths; the request schemas that do exist match code exactly.)
*Fix:* add request/response schemas, or generate the spec from code.

**M-18 `contracts/events/` and `contracts/webhooks/` are empty while outbound webhook code exists.**

> ✅ **FIXED 2026-07-27:** `contracts/webhooks/chat-webhooks.md` and `contracts/events/audit-events.md` (37 actions) added.
The Slack/Teams dispatcher builds concrete payloads
(`internal/notifications/webhook/dispatcher.go:116-130`) and audit events carry defined
action strings (e.g. `"employee.created"`, `internal/employees/handlers.go:140`) — the
de-facto domain events — none documented as contracts, against AGENTS.md workflow
step 2.
*Fix:* document the webhook payloads and audit-event catalog, or delete the scaffold
dirs.

### Frontend

**M-19 No permission/role checks in org-admin or employee apps (AGENTS.md violation).**

> ✅ **FIXED 2026-07-27:** Logins and shells role-gated in all three portals; nav filtered by `me.roleCode`.
`apps/organization-admin-web/app/login/page.tsx:35-37`,
`apps/employee-web/app/login/page.tsx:35-37`,
`apps/organization-admin-web/components/admin-shell.tsx:11-32`.
`platform-admin-web` does it right (`app/login/page.tsx:36` rejects non-`platform_`
roles), but org-admin accepts any valid account — an employee signs in and sees the
full admin nav (Journeys, Employees, Billing, Support), every item failing only at the
API layer with 403s. Nav arrays in all three shells are static, unfiltered by
`me.roleCode`.
*Fix:* verify an expected role set per app after login (and in the shell after `me()`),
deny/clear-session otherwise, and filter nav items by role.

**M-20 The auth callback persists tokens from the URL hash without validation (login CSRF).**

> ✅ **FIXED 2026-07-27:** Auth callback validates via `meWithToken` + refresh exchange before persisting the session.
`apps/organization-admin-web/app/auth/callback/page.tsx:11-24`. Whatever
`accessToken`/`refreshToken` appear in the fragment are stored; an attacker can link a
victim to `…/auth/callback#accessToken=<attacker's>…` and the victim's subsequent
actions land in the attacker's org.
*Fix:* call `me()` with the received token and persist only after the API confirms it,
ideally matched against state created at signup start; better, avoid token-in-URL
handoff entirely (post the code to the API, set cookies).

**M-21 App shells silently swallow all non-401 `me()` failures.**

> ✅ **FIXED 2026-07-27:** Shells render an error card with retry/sign-out on non-401 `me()` failures.
`apps/employee-web/components/employee-shell.tsx:36-41`,
`apps/organization-admin-web/components/admin-shell.tsx:48-53`,
`apps/platform-admin-web/components/platform-shell.tsx:45-50`. A 403 (wrong-portal
token, see M-19), 5xx, or network drop leaves the user on "Loading…" forever with no
error and no retry.
*Fix:* shell-level error state with retry/sign-out for non-401 failures.

**M-22 Empty `next.config.js` files shadow the real `next.config.ts` in two apps.**

> ✅ **FIXED 2026-07-27:** Shadowing `next.config.js` files deleted; `next.config.ts` now takes effect.
`apps/employee-web/next.config.js:1-3`, `apps/platform-admin-web/next.config.js:1-3`.
Next resolves config in order `.js` → `.mjs` → `.ts` (verified against installed
`next@16.2.11`), so the empty `.js` wins and `transpilePackages: ["@launchpad/ui",
"@launchpad/api-client"]` in `next.config.ts` is silently dead. Builds survive today
because symlinked workspace packages are transpiled anyway, but every future setting in
those two files will be silently ignored.
*Fix:* delete both `next.config.js` files.

**M-23 No HTTP security headers on any frontend app; platform-admin lacks `noindex`.**

> ✅ **FIXED 2026-07-27:** Shared security `headers()` (nosniff, Referrer-Policy, CSP, frame-ancestors) on all four apps; `noindex` on the three portals; API sets nosniff via `middleware.SecurityHeaders`.
`apps/marketing-web/next.config.ts:3-5` (identical in all four apps) — no `headers()`,
so no CSP, `X-Frame-Options`/`frame-ancestors`, `X-Content-Type-Options`,
`Referrer-Policy`, or HSTS anywhere, including the platform control plane, which also
lacks `noindex` metadata (`apps/platform-admin-web/app/layout.tsx:10-13`).
*Fix:* shared `headers()` block (minimum: `nosniff`, `Referrer-Policy:
strict-origin-when-cross-origin`, `frame-ancestors 'none'` for portals, and a CSP) and
`robots: { index: false }` for the three portal apps. The Go API likewise emits no
security headers (`pkg/httpx/httpx.go:30-53` sets only `Content-Type`) — add `nosniff`
at minimum (one-line middleware in `newRouter`).

### Tests & docs

**M-24 Journeys, audit, leads, and platform have no tests; organizations' coverage is a single `Slugify` test.**

> ✅ **FIXED 2026-07-27:** Tests added for journeys (versioning/publish), organizations (service), audit (filtering), leads, platform.
`internal/journeys/service.go` (205 lines of draft/publish versioning — core product
logic), `internal/audit/` (compliance surface), `internal/leads/`, and
`internal/platform/` contain no `*_test.go`.
`internal/organizations/organizations_test.go:9` is 22 lines testing only `Slugify`.
(No trivially-empty or always-pass tests were found — everything that exists is
genuine.)
*Fix:* prioritize a journeys publish/versioning test, then service tests with fake
stores mirroring `sso_test.go` for organizations and audit list filtering.

**M-25 Architecture doc is two phases stale; `docs/api/`, `docs/runbooks/`, `docs/security/` are empty.**

> ✅ **FIXED 2026-07-27:** `docs/architecture/overview.md` rewritten for the current system; `docs/api`, `docs/runbooks`, `docs/security` seeded.
`docs/architecture/overview.md:1-18` is titled "Phase 0" and shows exactly three
modules (auth, organizations, audit); the codebase now has 23 `internal/` modules. The
three empty doc dirs mean there are no runbooks (e.g. `make migrate-indexes` failures,
SCIM token rotation) and no security docs despite SCIM bearer tokens, SSO client
secrets, and per-tenant webhook URLs. (ADR `docs/decisions/0001-mongodb-redis.md` is
accurate.)
*Fix:* rewrite the overview for the current module set (or mark it historical and add a
new one); seed each empty dir with an index doc; document token/secret handling under
`docs/security/`.

---

## Low findings

### Backend correctness

- **L-1** `internal/audit/mongo/store.go:69-71` — limit clamping bug: `limit=150` resets to the *default* 50 instead of clamping to the 100 max (`limit > maxListLimit` falls into the same branch as `limit <= 0`). *Fix:* clamp to `maxListLimit`. → ✅ **FIXED 2026-07-27:** clamps to `maxListLimit`.
- **L-2** `internal/employees/mongo/store.go:131-134` — same clamping bug (`limit=100`, the documented max, resets to 50); additionally no cursor/offset, so lists beyond 100 employees are unreachable. *Fix:* clamp correctly; add keyset pagination. → ✅ **FIXED 2026-07-27:** clamp fixed; offset pagination added (`?offset=`).
- **L-3** `internal/auth/invitations.go:138-167` — invitation single-use is check-then-act, not atomic; two concurrent `AcceptInvitation` calls with the same token can both succeed. *Fix:* consume the token with an atomic `findOneAndDelete` gated on expiry before activating. → ✅ **FIXED 2026-07-27:** atomic `FindOneAndDelete` consumption gated on expiry.
- **L-4** `internal/app/app.go:858-869` — public `GET /cms/pages/{slug}`, `GET /auth/sso/{orgSlug}/start`, and `GET /auth/sso/callback` sit outside the rate-limit group; the SSO callback triggers an outbound OIDC token exchange per request (abuse/amplification vector). *Fix:* include at least the SSO callback in the rate-limited group. → ✅ **FIXED 2026-07-27:** SSO start/callback and public CMS pages moved into the rate-limited group.
- **L-5** `internal/auth/service.go:371-382` — login without an explicit `organizationId` picks `memberships[0]` with no defined ordering. *Fix:* sort deterministically or require an explicit org when multiple memberships exist. → ✅ **FIXED 2026-07-27:** memberships sorted by `OrganizationID` before picking.
- **L-6** `internal/featureflags/mongo/store.go:139-173` — `UpsertOverride` is a non-atomic find-then-insert; concurrent `SetOverride` calls hit the unique index and the raw duplicate-key error bubbles up as a 500. *Fix:* `UpdateOne` with upsert filtered on `{organizationId, key}` (`$set`/`$setOnInsert`). → ✅ **FIXED 2026-07-27:** single atomic `UpdateOne` upsert (`$set`/`$setOnInsert`).
- **L-7** `internal/journeys/service.go:99-117` — step `Position` assigned as `CountSteps + 1` with no unique index on `(organizationId, journeyTemplateId, version, position)` (`internal/journeys/mongo/store.go:50-63` creates it non-unique); concurrent `AddStep` calls produce duplicate positions. `DueOffsetDays` also accepts negatives without validation. *Fix:* unique index + duplicate-key → conflict mapping; validate `DueOffsetDays >= 0`. → ✅ **FIXED 2026-07-27:** unique position index + duplicate-key→409; `DueOffsetDays >= 0` validated. Ops note: drop the old non-unique index on existing deployments.
- **L-8** `internal/employees/service.go:147-177,248-282` — `Update` lets an employee become their own manager; `validateReferences` checks existence but never rejects `managerEmployeeID == employeeID`. *Fix:* reject self-reference. → ✅ **FIXED 2026-07-27:** self-as-manager rejected (`INVALID_REFERENCE`).
- **L-9** `internal/leads/mongo/store.go:54-58` — `List` is an unbounded `Find(bson.M{})` across all leads. *Fix:* add a limit and keyset pagination. → ✅ **FIXED 2026-07-27:** limit + keyset (`before`) pagination added.
- **L-10** `internal/notifications/mongo/store.go:21,63-67` — `defaultListLimit = int64(50)` is defined but unused; `ListForUser` issues an unbounded `Find`, loading every notification a user has ever received on each `GET /notifications`. *Fix:* `SetLimit(defaultListLimit)`. → ✅ **FIXED 2026-07-27:** `SetLimit(defaultListLimit)` applied.
- **L-11** `internal/scim/service.go:106-140` — `CreateUser` provisions the account and membership before persisting the SCIM record and audit event; a later failure leaves an active member the IdP won't cleanly retry. *Fix:* persist the SCIM record first or compensate on failure. → ✅ **FIXED 2026-07-27:** compensating `SetActive(false)` when SCIM record persistence fails.
- **L-12** `internal/organizations/service.go:48-62` — `CreateWithOwner` is non-transactional; if membership creation fails, the org exists with no owner and the slug is burned (unique index, `internal/organizations/mongo/store.go:44`). *Fix:* transaction or compensating delete. → ✅ **FIXED 2026-07-27:** compensating `DeleteOrganization` frees the slug on membership failure.

### Security hardening

- **L-13** `pkg/config/config.go:88-94` — no minimum-strength check on `JWT_SECRET` outside local; only empty and the literal dev value are rejected. HS256 secrets are brute-forceable offline from any captured token. *Fix:* require ≥32 bytes when `AppEnv != "local"`. → ✅ **FIXED 2026-07-27:** ≥32 bytes required when `APP_ENV != "local"`.
- **L-14** `internal/auth/service.go:188` — refresh-token hash compared with `!=` instead of `crypto/subtle.ConstantTimeCompare`. Timing exploitation of a SHA-256 hex compare is impractical, but the idiomatic control costs nothing. → ✅ **FIXED 2026-07-27:** `crypto/subtle.ConstantTimeCompare` used.
- **L-15** `pkg/middleware/middleware.go:226-230` — `RequirePlatform` accepts any role with prefix `platform_`; any future role string with that prefix (including an accidental one) gains full platform access. *Fix:* exact allowlist of known platform roles. → ✅ **FIXED 2026-07-27:** exact allowlist (`platform_owner`, `platform_admin`).
- **L-16** `pkg/config/config.go:60` vs `.env.example:17` — `PASSWORD_MIN_LENGTH` is documented but never read; `Load` hardcodes the default. *Fix:* parse and validate the env var, or remove it from the example. → ✅ **FIXED 2026-07-27:** `PASSWORD_MIN_LENGTH` parsed with validation.
- **L-17** `pkg/security/security.go:124-131` — `ParseAccessToken` never validates the `iss` claim even though `IssueAccessToken` sets a fixed issuer (security.go:21,109). *Fix:* `jwt.WithIssuer(tokenIssuer)`. → ✅ **FIXED 2026-07-27:** `jwt.WithIssuer(tokenIssuer)` enforced.

### Config, ops, repo hygiene

- **L-18** `pkg/config/config.go:15-17,56-58` — `MONGODB_URI` and `REDIS_URL` default to localhost regardless of `APP_ENV`; boot fails fast on ping, but the error points at localhost, which is misleading in a container. *Fix:* require both explicitly outside local. → ✅ **FIXED 2026-07-27:** `MONGODB_URI`/`REDIS_URL` required explicitly outside local.
- **L-19** `internal/app/app.go:1066-1074` — `http.Server.ErrorLog` is unset, so server-level errors go through stdlib `log`, violating "logging must use `log/slog` only" and bypassing the JSON handler. *Fix:* `ErrorLog: slog.NewLogLogger(slog.Default().Handler(), slog.LevelError)`. → ✅ **FIXED 2026-07-27:** `http.Server.ErrorLog` wired to slog.
- **L-20** `internal/app/app.go:840-844` — `/healthz` returns `ok` without touching Mongo/Redis, and every LB probe is written to the request log. If used as a readiness probe, a pod with a dead database still receives traffic. *Fix:* add `/readyz` that pings both dependencies; skip/downgrade probe logging. → ✅ **FIXED 2026-07-27:** `/readyz` pings Mongo+Redis (2s); health probes skipped in the request log.
- **L-21** `pkg/middleware/middleware.go:24-31` — the rate limiter is in-memory and per-process: with N replicas the effective public-auth limit is N×20/min, and limits reset on every deploy. *Fix:* move to the already-required Redis when correctness at scale matters. → ✅ **FIXED 2026-07-27:** per-process limitation and Redis path documented on the limiter.
- **L-22** `apps/api/cmd/api/main.go:43` — `mongoDB.Close` runs with `context.WithoutCancel(ctx)` and no timeout; `Disconnect` can block shutdown indefinitely. *Fix:* wrap in a 5s timeout. → ✅ **FIXED 2026-07-27:** 5s timeout on `mongoDB.Close`.
- **L-23** `docker-compose.yml:6-7,36,40` — committed credentials (`launchpad/launchpad`, `JWT_SECRET: local-dev-only-change-me`). Clearly local-dev dummies, but AGENTS.md says "secrets in environment variables only". *Fix:* `${VAR:-default}` interpolation. → ✅ **FIXED 2026-07-27:** `${VAR:-default}` interpolation in compose.
- **L-24** `docker-compose.yml:31-49` + `apps/api/Dockerfile:1` — the `api` service has no healthcheck despite `/healthz` existing; images use floating major tags (`mongo:7`, `redis:7-alpine`, `golang:1.26-alpine`). *Fix:* add a healthcheck; pin minor versions/digests. → ✅ **FIXED 2026-07-27:** api healthcheck added; images pinned (compose minors; Dockerfile `golang:1.26.5-alpine3.23`).
- **L-25** `.gitignore` — missing `*.tsbuildinfo` and `.raven/`; four `apps/*/tsconfig.tsbuildinfo` caches and the `.raven/` tool dir show as untracked noise. *Fix:* add both patterns. → ✅ **FIXED 2026-07-27:** `*.tsbuildinfo` and `.raven/` added to `.gitignore`.
- **L-26** Repo root — `AI_Development_Workflow_Training_Manual.docx` and `AI_Native_Software_Engineering_Operations_Manual.docx` are tracked binaries at root. *Fix:* move under `docs/` or out of the repo. (`coverage.out` at root is correctly gitignored — no action.) → ✅ **FIXED 2026-07-27:** both `.docx` manuals moved to `docs/`.

### Frontend

- **L-27** `apps/organization-admin-web/lib/session.ts:18-20` (same in all three portals) — the refresh token is persisted but never used; nothing calls `/auth/refresh`, so users are signed out at every 15-minute access-token expiry. *Fix:* wire a `refresh()` into the api-client with single-flight retry on 401, or stop storing the token. → ✅ **FIXED 2026-07-27:** single-flight `refresh()` retries the original request once on 401.
- **L-28** `apps/employee-web/app/assignments/[assignmentID]/page.tsx:26-56`, `apps/employee-web/app/notifications/page.tsx:19-44` (pattern copied into every portal page) — fetch effects have no cancellation/stale-response guard; overlapping loads can resolve out of order. *Fix:* ignore flag or `AbortController`. → ✅ **FIXED 2026-07-27:** stale-response guards on all 17 portal pages.
- **L-29** `apps/marketing-web/app/[slug]/page.tsx:43-54` — the CMS loader swallows all errors and silently serves hardcoded fallback copy on any API outage; the `err.status === 404` branch is dead code (both branches return the identical expression). *Fix:* fall back only on 404; propagate or log other errors; collapse the duplicate branch. → ✅ **FIXED 2026-07-27:** fallback only on 404; dead branch removed; other errors propagate.
- **L-30** `apps/marketing-web/app/site-header.tsx:10-11`, `apps/marketing-web/app/site-footer.tsx:18-31,83-85` — mislabeled/placeholder navigation: header "Customers"→`/product`, "Resources"→`/demo`; nine footer links (About, Careers, Documentation, Security, Status, Privacy, Terms…) all point to `/product` or `/demo`. *Fix:* point at real CMS slugs or remove until content exists. → ✅ **FIXED 2026-07-27:** placeholder links removed; header/footer link only real CMS slugs.
- **L-31** `apps/employee-web/lib/api.ts:6-13`, `apps/organization-admin-web/lib/api.ts:6-13`, `apps/platform-admin-web/lib/api.ts:6-13` (plus the three shells, `formString` in 7 files, `formatPrice` in both billing pages) — per-app duplication: the three `lib/api.ts` are byte-identical, the three `lib/session.ts` differ only in storage-key prefix, the shells are ~90% the same logic. Three copies of security-sensitive session code can now drift. *Fix:* keyed `createSessionStorage(prefix)` in `packages/api-client`; parameterized auth-shell in `packages/ui`. → ⚠️ **PARTIAL 2026-07-27:** shared `createSessionStorage` and thin per-app api wrappers done; the shared auth-shell was not lifted into `packages/ui` (follow-up).
- **L-32** `packages/api-client/src/index.ts:611,671,751,860` — path parameters interpolated without `encodeURIComponent` while flag keys/slugs in the same file do encode (:797,809,936). Latent today (server-generated ObjectIDs) but invites injection if an ID ever becomes user-influenced. *Fix:* encode every interpolated segment uniformly. → ✅ **FIXED 2026-07-27:** every interpolated path segment `encodeURIComponent`-wrapped.
- **L-33** `apps/employee-web/lib/api.ts:6-7` (and five other `NEXT_PUBLIC_` fallback sites) — silent `http://localhost:8080` fallback when the env var is missing, turning a config mistake into a confusing runtime outage. *Fix:* validate env at build/startup and throw outside development. → ✅ **FIXED 2026-07-27:** zod-validated env per app; localhost fallback allowed only in dev/build, production throws.

### Structure & docs

- **L-34** Empty directories: `internal/marketing/`, `internal/platformadmin/`, `migrations/`, `contracts/events/`, `contracts/webhooks/`, `seeds/`, plus `pkg/testutil/` and `pkg/validation/` (untracked). AGENTS.md step 5 references `migrations/`, which contains nothing. *Fix:* confirm intent; delete or populate; align AGENTS.md. → ✅ **FIXED 2026-07-27:** empty scaffold dirs removed (incl. `seeds/`); `AGENTS.md` step 5 updated.
- **L-35** `Makefile:10-13` — help text advertises ports 3000–3003; the apps bind 3300–3303 (folded into M-16 but the doc fix stands alone). → ✅ **FIXED 2026-07-27:** verified already correct after the port alignment; no edit needed.
- **L-36** `LaunchPad_Complete_PRD_and_Build_Spec.md:76,79,381` — several in-scope PRD features have no corresponding code: meetings/scheduling (3.1), equipment and access requests (5.3.8), calendar/communication/SCM/PM integrations beyond HRIS/IdP/Slack/Teams-webhooks, support impersonation (5.2.2); assessments exist only as a `quiz` step-type constant (`internal/journeys/types.go:30`). Defensible for a phased build, but no doc tracks status. *Fix:* add a PRD-section → implemented/planned map in `docs/product/`. → ✅ **FIXED 2026-07-27:** `docs/product/prd-status.md` added (PRD section → Implemented/Partial/Planned).
- **L-37** `docs/api/` — empty while `contracts/openapi/openapi.yaml` exists; readers looking under `docs/` find nothing. *Fix:* index doc pointing at the contract. → ✅ **FIXED 2026-07-27:** `docs/api/README.md` indexes the API docs and points at the OpenAPI contract.

---

## Convention compliance scorecard (AGENTS.md)

| Rule | Status | Notes |
| --- | --- | --- |
| Handlers have no business logic | ⚠ Violated in 2 places | `employees` provisioning (M-6), `organizations` invite (M-6); otherwise clean |
| Services do not import HTTP frameworks | ✅ Followed | Verified across all 21 domains |
| Repository interfaces live in the domain package | ✅ Followed | With `var _ Iface = (*Store)(nil)` compile checks in adapters |
| Mongo documents do not leak as API responses | ⚠ Violated in 6 modules | M-4 |
| Cross-module calls use explicit interfaces | ✅ Followed | Small ports (`AssignmentSource`, `OrgDirectory`, `Provisioner`…) with app-layer adapters |
| Logging must use `log/slog` only | ✅ Followed | One exception: unset `http.Server.ErrorLog` falls back to stdlib `log` (L-19) |
| Validate API responses at trust boundaries (Zod) | ✅ Followed | `packages/api-client` validates every envelope (`parseEnvelope`, index.ts:480-504); no blind casts |
| Permission checks in navigation and API | ⚠ Frontend gap | Server-side checks are pervasive; org-admin/employee apps have none (M-19) |
| Contracts before implementation | ⚠ Drifting | Route coverage complete; bodies/schemas and event/webhook contracts missing (M-17, M-18) |
| Secrets in environment variables only | ⚠ At-rest gap | No hardcoded secrets in source (verified); tenant secrets unencrypted at rest (M-9); dev dummies in compose (L-23) |

## What looked good

- **Consistent hexagonal discipline** — every module defines its ports in the domain
  package with compile-time adapter assertions; `internal/app/app.go` wires
  cross-module dependencies through explicit small adapters.
- **Token hygiene** — 32-byte `crypto/rand` refresh/invite/SCIM/state tokens, all
  stored only as SHA-256 hashes; refresh rotation with delete-on-mismatch
  (`internal/auth/service.go:182-226`); invitation TTL index + expiry filter
  (`internal/auth/mongo/invitation_store.go:65-81`); atomic SSO state consume via
  `GETDEL` (`internal/sso/redis/store.go:45-46`).
- **OIDC verification is textbook** — RS256-only, issuer/audience/expiry, nonce,
  `email_verified:false` refusal (`internal/sso/oidc/verifier.go:142-173`).
- **Layered SSRF defenses** — `pkg/safehttp` (dial-time post-DNS address rejection, no
  redirects) wired into both tenant-controlled-URL clients (HRIS, OIDC); host
  allowlists for chat webhooks; webhook URLs stripped from logged errors
  (`internal/notifications/webhook/dispatcher.go:107-114`).
- **Assistant anti-hallucination design** — citations built from retrieved chunks,
  never model output; citation-free answers refused
  (`internal/assistant/service.go:100-107`); Anthropic system prompt treats passages as
  untrusted data; error bodies capped.
- **Tenant isolation is consistent** — every tenant-scoped repository query filters by
  `organizationId`; SCIM explicitly refuses to graft foreign-tenant accounts
  (`internal/scim/service.go:214-251`).
- **Error hygiene** — sentinel errors mapped to codes/statuses, unknown errors become a
  generic `INTERNAL_ERROR` with the real error logged; cursor decode/close errors
  joined; request bodies size-limited with `DisallowUnknownFields`; server timeouts all
  set; graceful shutdown with drain and `errors.Join`.
- **SCIM/SSO tests** — ~1,050 lines exercising org-scoped stores over `httptest`,
  including cross-tenant isolation — a model for closing the gaps in H-6/H-8/M-24.
- **OpenAPI route coverage is complete** — zero undocumented endpoints, zero phantom
  paths; existing request schemas match code exactly.

## Recommended priority order

1. **This week (user-visible / exploit-cheap):** H-4 (form reset bug — one mechanical
   fix across 12 call sites), H-1 (quiz scores), H-3 (RealIP before the limiter), M-16
   (port alignment), M-22 (delete shadowing `next.config.js` files).
2. **Before the next deploy:** H-2 (seed create-only), M-2 (assignments authz), M-20
   (auth callback validation), M-13 (session revocation check), M-14 (assistant rate
   limit), M-11 (write timeout).
3. **Next sprint:** H-5 + M-23 (cookie-based sessions + security headers — do
   together), H-6/H-7/H-8/M-24 (test backfill, reusing the SCIM/SSO fake pattern),
   M-5/M-6/M-7 (atomicity of multi-step writes), M-10 (shared index registry).
4. **Track as hardening:** M-9 (encryption at rest), M-15 (SCIM token expiry), M-17/M-18
   (contract completeness), M-25 + L-36 (docs), all remaining lows.
