# QA / UAT checklist — LaunchPad launch readiness

Full expected-behavior checklist across every product surface. Run against a
staging environment that mirrors production config before sign-off. Pair with
`docs/runbooks/launch-validation.md` (preflight gates + provider smokes).

## Legend

- ✅ **Pass** — behaves as described
- ❌ **Fail** — blocks sign-off; file a defect with repro
- ➖ **N/A** — not applicable to this release/environment (justify in notes)
- 🔧 **Regression-prone** — area touched by the 70 fixes in
  `docs/reviews/codebase-review-2026-07-27.md` (2026-07-27). Verify these
  even in a spot-check pass; they changed most recently.

## Environment preconditions

- [ ] Deployed commit matches the release candidate; `pnpm launch:check`
      passed on that commit (attach evidence template).
- [ ] `make migrate-indexes` (or CI `migrations` job) ran successfully
      against the target MongoDB. 🔧
- [ ] `/readyz` returns 200 (Mongo + Redis reachable). 🔧
- [ ] Test tenants exist: one org with `organization_owner`, one `hr_admin`,
      one `manager`, one `employee`; a second org for cross-tenant checks;
      one `platform_owner` account.
- [ ] Seed data: at least one published journey template with document, task,
      approval, and quiz steps; one active subscription plan.
- [ ] IdP test app configured for SSO/SCIM checks (or mark those rows ➖).
- [ ] Browser dev tools available to inspect cookies, headers, and console.

## 1. Auth & sessions (all portals)

- [ ] Register with valid email/password (≥ `PASSWORD_MIN_LENGTH`) succeeds
      and lands in the correct portal. 🔧
- [ ] Weak password is rejected client- and server-side. 🔧
- [ ] Login with wrong password fails with a generic message (no
      account-existence leak). 🔧
- [ ] After login, session is an `HttpOnly; SameSite=Lax` cookie (`Secure`
      outside local); **no tokens in `localStorage`** (dev tools → Application). 🔧
- [ ] Access-token expiry triggers silent single-flight refresh; the user is
      not signed out at 15 minutes. 🔧
- [ ] Concurrent tabs: one refresh in flight, no 401 storms. 🔧
- [ ] Logout clears the cookie and the session is invalidated server-side —
      replaying the old cookie/token gets 401 immediately. 🔧
- [ ] Logging into the wrong portal with a valid account is denied
      (employee → org-admin login rejected; non-`platform_*` → platform-admin
      rejected). 🔧
- [ ] API outage during shell load shows an error card with retry/sign-out,
      not an infinite "Loading…". 🔧

## 2. Signup → org-admin callback

- [ ] Marketing `/signup` submits and redirects to org-admin
      `/auth/callback`. 🔧
- [ ] Callback validates the tokens with the API (`me` + refresh exchange)
      before establishing the session; a tampered/forged token in the URL
      hash is rejected. 🔧
- [ ] New org is created with the registering user as `organization_owner`;
      slug is unique; a failed registration leaves no orphan user blocking
      retry with the same email. 🔧
- [ ] Re-submitting the signup form does not create duplicate orgs/leads.

## 3. Organization admin portal

- [ ] Nav is filtered by role: `hr_admin` does not see Billing;
      `organization_owner` sees all items. 🔧
- [ ] **Employees**: list, invite (creates invited record), edit, assign
      manager; an employee cannot be their own manager (rejected). 🔧
      Pagination past 100 employees works (`?offset=`). 🔧
- [ ] **Departments**: create/edit departments from the employees screen;
      after a successful create the form resets and the new row appears — no
      false failure message. 🔧
- [ ] **Journeys**: create template, add steps (document/task/quiz/approval),
      draft → publish versioning; duplicate step positions are impossible;
      `DueOffsetDays` rejects negatives. 🔧 Form resets after create. 🔧
- [ ] **Assignments**: assign a published journey to an employee; assigning
      the same journey twice returns a clean 409 (not a 500). 🔧
- [ ] **Assignments (authz)**: as `employee`, listing other employees'
      assignments or step submissions returns 403. 🔧
- [ ] **Approvals**: an approval step routes to the employee's manager (not
      the assignment creator); manager can approve/reject; a non-approver
      gets 403. 🔧
- [ ] **Quiz steps**: posting a client-supplied score is rejected; quiz
      completion follows server rules only. 🔧
- [ ] **Billing** (owner only): current plan/subscription renders;
      plan change persists. 🔧
- [ ] **Support**: create ticket, view status; form resets after submit. 🔧
- [ ] **Knowledge** (API-level; no dedicated page): upload → approve →
      indexed; approved content is answerable via the assistant. 🔧
- [ ] **HRIS** (API-level): configure BambooHR, sync, apply; a Mongo outage
      during sync surfaces as 500, never as "HRIS not configured". 🔧
      Stored API token is encrypted at rest when `ENCRYPTION_KEY` is set
      (`enc:v1:` prefix in Mongo). 🔧
- [ ] **SSO** (API-level): save OIDC config; client secret never returned by
      the API and is encrypted at rest. 🔧
- [ ] **SCIM** (API-level): issue token; plaintext shown once; token carries
      a 90-day `expiresAt` in the response; old token stops working after
      rotation. 🔧
- [ ] **Settings** (API-level): update org profile/branding; audit events
      recorded for each change.
- [ ] **Analytics**: onboarding summary headcount is exact even above 100
      employees. 🔧

## 4. Employee portal

- [ ] Login restricted to employee-role accounts; nav filtered by role. 🔧
- [ ] Assignments list shows only the signed-in employee's assignments. 🔧
- [ ] Assignment detail: steps render in position order; complete
      document/task steps; due dates shown.
- [ ] Notifications page loads the user's notifications (bounded list). 🔧
- [ ] Assistant (`/assistant/ask`): answers are citation-grounded; a
      question with no supporting knowledge is refused, not hallucinated;
      rate-limited (~10 req/min per IP) — burst returns 429. 🔧
- [ ] Rapid navigation between pages shows no stale/overwritten data
      (out-of-order fetch guard). 🔧

## 5. Platform admin portal

- [ ] Login restricted to `platform_owner` / `platform_admin` exactly (no
      other `platform_*` string gains access). 🔧
- [ ] Overview dashboard loads (`GET /platform/overview`).
- [ ] **Organizations**: list, view, suspend, activate; suspended org's
      users lose access. Responses expose only public org fields. 🔧
- [ ] **Leads**: list is paginated (`before` cursor); no unbounded load. 🔧
- [ ] **Feature flags**: create/edit global flag; set per-tenant override;
      concurrent overrides don't 500. 🔧 **Restart the API and confirm flag
      edits survive** (seed is create-only). 🔧
- [ ] **Plans/billing**: create/edit plan; assign subscription to org;
      **restart the API and confirm plan edits survive**. 🔧
- [ ] **Support**: triage tickets, update status.
- [ ] **CMS**: draft → publish a page; published content appears on the
      marketing site.
- [ ] No impersonation UI exists (Planned feature) — confirm none is
      reachable. ➖ expected

## 6. Marketing site

- [ ] `/product`, `/pricing`, `/demo`, `/signup` and CMS `[slug]` pages
      render; header/footer link only to real pages (no placeholder links). 🔧
- [ ] CMS page fallback: on API 404 the hardcoded fallback renders; on API
      5xx the error surfaces (no silent stale content). 🔧
- [ ] **Demo form**: submit creates a lead; success message shows; form
      resets — no false failure after a created lead. 🔧 Duplicate
      submission is handled gracefully.
- [ ] **Signup form**: creates account + org and redirects per section 2. 🔧
- [ ] **Theme switcher**: toggles light/dark; persists across reload and
      navigation; no flash of wrong theme on load.
- [ ] Security headers present (CSP, `X-Content-Type-Options`,
      `Referrer-Policy`, `frame-ancestors`). 🔧

## 7. API hardening

- [ ] `GET /healthz` returns 200 (liveness only).
- [ ] `GET /readyz` returns 200; with Mongo stopped it returns non-200. 🔧
- [ ] Rate limits: >20 req/min on `/auth/login` from one IP returns 429;
      SSO start/callback and public CMS pages are also limited. 🔧
- [ ] Behind the LB, limiter keys on the real client IP (RealIP middleware
      installed; spoofed `X-Forwarded-For` is overwritten by ingress). 🔧
- [ ] API responses carry `X-Content-Type-Options: nosniff`. 🔧
- [ ] Error envelope shape is consistent (`error.code` / `error.message`);
      provisioning failures return a static message, never raw driver
      errors. 🔧
- [ ] No Mongo document internals (`_id`, bson tags) appear in any API
      response body. 🔧
- [ ] CORS allows only configured origins.
- [ ] Requests with oversized bodies are rejected; unknown JSON fields are
      rejected.

## Sign-off

| Field | Value |
| --- | --- |
| Date (UTC) | |
| Environment | |
| Commit SHA | |
| QA operator | |
| Rows ✅ / ❌ / ➖ | |
| Defects filed (links) | |
| Open blockers | |
| Launch decision | GO / NO-GO |
| Sign-off (name, role) | |
