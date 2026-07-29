# LaunchPad — Agent Plan & Implementation Ledger

> Live ledger of PRD (`LaunchPad_Complete_PRD_and_Build_Spec.md`) features vs the
> codebase, compiled 2026-07-28 from a four-area reconciliation sweep with
> file:line evidence. Update this file with every feature commit (PRD §21 rule:
> contracts first, tests with each feature, audit privileged actions).

Legend: **DONE** = works end-to-end (API + UI where applicable) · **PARTIAL** = delta noted · **MISSING** = not started.

## Status by area

### Platform admin (§5.1–5.2)

| Feature | Status | Evidence / delta |
| --- | --- | --- |
| Dashboard org counts (total/active/trial/suspended) | DONE | `internal/platform/service.go:133` |
| Platform business/operations metrics | DONE | MRR/ARR, active subscriptions, tenant/lead totals, support SLA risk, onboarding funnel, integration health endpoints, Prometheus request/error metrics, security review queue, and operator job/delivery/storage views |
| Organization management (view, activate/suspend/close, plan change, flags) | DONE | Search/filter, lifecycle controls, plan changes, per-org flag editor, audited close |
| Impersonation / support sessions (§379–388) | DONE | Reason + expiry, read-only scoped token, revoke, owner notification, audit context |
| Billing: plan catalogue, prices, subscription status | DONE | `internal/billing` |
| Billing: invoices, payments, refunds, tax, dunning, coupons, revenue reporting, MRR | DONE | Paystack checkout/webhooks/refunds, invoice tax, coupon enforcement, scheduled dunning, MRR/ARR and audited operator UI |
| Feature flags (global, plan-based, per-tenant override, kill switch) | DONE | `internal/featureflags` |
| Flags: percentage rollouts, cohorts, expiry, history view | DONE | Stable tenant hashing, user test cohorts, automatic expiry, immutable history + platform UI (`internal/featureflags`) |
| Template marketplace admin (§5.2.5) | DONE | Official/customer templates, moderation, versions, removal, featured state, installations and ratings (`internal/marketplace`) |
| Platform staff management (§5.2.6) | DONE | Full role set, lifecycle UI, MFA, owner-approved one-hour break-glass, revocation, and quarterly access attestation |
| Support tickets (create/list/status, priority) | DONE | `internal/support` |
| Support: notes, severity, assignment/escalation, SLA, attachments, canned responses, CSAT, categories | DONE | Conversation/internal-note model, priority SLA clocks and metrics, escalation/reassignment, attachments/canned response metadata, categories and CSAT |
| CMS pages (draft/publish, marketing render) | DONE | `internal/cms` |
| CMS: navigation, blog, FAQs, legal versions, SEO fields, scheduling | DONE | Typed content, SEO/nav fields, public navigation, versioned legal content and scheduled publication sweep |
| Platform security center (§5.2.9) | DONE | Privilege posture, review queue/attestation, emergency access controls and audit export |
| Platform operations (§5.2.10: jobs, DLQ, delivery logs, storage) | DONE | Scheduler status/manual run, retry/dead-letter delivery records, audited retry and live Mongo storage metrics |
| Launch readiness (extra, not in spec) | DONE | `internal/platform/readiness.go` |

### Organization app (§5.3)

| Feature | Status | Evidence / delta |
| --- | --- | --- |
| Setup wizard (§5.3.1) | DONE | Ten-stage guided `/setup` flow with shared durable progress, custom-domain/profile settings, and real destination actions |
| Employee directory + HRIS sync | DONE | `internal/employees`, `internal/hris` (BambooHR) |
| CSV bulk import | DONE | Audited, bounded header-based import with row-level 207 results and org-admin upload UI |
| Employment status change | DONE | PATCH route, audited status changes, roster UI |
| Offboarding / archive | DONE | Offboarded lifecycle + UI |
| Buddy assignment, team, location fields | DONE | Manager/buddy references plus team/location roster fields and UI |
| Invitations | DONE | Single-use activation plus pending list, resend (old-token revocation), revoke, expiry, and UI |
| Journey builder (draft/publish, due offsets) | DONE | `internal/journeys` |
| Step types | DONE | Full PRD catalogue plus assessment/request extensions; specialized steps retain typed config while generic steps use submission lifecycle |
| Journey versioning | DONE | New draft versions, clone, copy-forward rollback, pinned assignments |
| Business-day due dates, parallel stages, prerequisites, reminders, approval rules, localization | DONE | Business-day scheduling, stages/parallel groups, prerequisites, conditional skips, reusable subflows, retry escalation, audited overrides, locale snapshots/translations and reminder sweeps |
| Assignment rules engine (auto-assign by role/dept/start date) | DONE | `internal/assignments/rules.go` + `assignment_rules` collection, auto-assign on employee create/HRIS via employees `RuleApplier` port, run-on-demand, org `/assignment-rules` UI |
| Knowledge (ownership, review dates, scopes, approval-before-index, citations) | DONE | `internal/knowledge`, `internal/assistant` |
| Knowledge: version history, retention, stale alerts, external connectors | DONE | Immutable snapshots, re-approval versioning, retention/review scheduler alerts, SSRF-protected connector sync and admin sync controls |
| Knowledge org-admin UI | DONE | Lifecycle, sources, approval/index controls |
| Assessments (§5.3.6) | DONE | Reusable definitions, question types, randomization, attempts, manager review, certificates |
| Meetings & scheduling (§5.3.7) | DONE | Scheduling, rescheduling, attendance/no-show, notes, 24h deduplicated reminders, journey-step creation, employee/manager UI, and Google/Microsoft OAuth calendar sync (`internal/meetings`, `internal/jobs/meeting_reminders.go`) |
| Equipment & access requests (§5.3.8) | DONE | Employee requests, manager decision/fulfillment, journey-step creation |
| Manager dashboard (§5.3.9) | DONE | Direct-report rollup, overdue/approval/blocker views plus assignment stage/step inspection and audited override controls |
| Analytics: completion rate + avg time | DONE | `internal/analytics` |
| Analytics: breakdowns, overdue rate, milestones, AI question reports, drop-off | DONE | Department/role breakdowns, overdue rate, milestone funnel/drop-off and assistant question/refusal reports |

### Employee app (§5.4) + cross-cutting (§6)

| Feature | Status | Evidence / delta |
| --- | --- | --- |
| Employee home (welcome, progress) | DONE | `apps/employee-web` |
| Today/overdue task views, manager/buddy contacts | DONE | Employee due/overdue sections and manager/buddy contact cards |
| AI assistant UI (employee) | DONE | Cited answers, refusal/escalation, feedback |
| Employee support UI + ticket categories + blockers endpoint (`POST /me/blockers`) | DONE | Standalone support UI, categorized tickets/blockers, manager surfacing |
| Step start/submit endpoints | DONE | Explicit lifecycle endpoints wired |
| Auth (register/login/JWT/refresh rotation/revocation/invitations) | DONE | `internal/auth` |
| **Password reset** | DONE | Single-use hashed 1h tokens, always-202 request, all-session revocation, request/reset forms on all portals |
| Org switching (multi-org) | DONE | Authenticated membership list + audited session rotation; org-admin and employee header switchers |
| SSO (OIDC + SAML 2.0) + SCIM v2 | DONE | Tenant OIDC and signed SAML Web SSO, one-time state/request binding, SP metadata + admin configuration UI, pre-provisioned account mapping, SCIM v2 |
| **Email channel** (invitations, notifications) | DONE | Resend-compatible sender + log fallback; invitations, resets, and notification fan-out |
| In-app + Slack + Teams notifications | DONE | `internal/notifications` |
| Notification typing, due-soon/overdue, journey-completed, SMS | DONE | Typed/deep-linked in-app, email, Slack/Teams and provider-gated SMS delivery with retry/dead-letter tracking |
| RBAC (permissions, built-ins, custom roles, enforcement) | DONE | `internal/roles`, pulled ahead of Phase 4 |
| Audit event fields | DONE | IP, user-agent, request-id, result/failure, impersonation, actor type, and structured before/after fields |
| **CSRF protection** (cookie auth) | DONE | Double-submit CSRF middleware for cookie mutations; bearer requests exempt |
| MFA (TOTP) | DONE | Enrollment/confirmation, login challenge, backup codes, disable, portal UI |

### Marketing (§5.1), NFR (§7), data model (§10)

| Feature | Status | Evidence / delta |
| --- | --- | --- |
| Home/product/pricing/demo/signup, SEO+OG+JSON-LD+sitemap+robots | DONE | `apps/marketing-web` |
| Feature pages (10), solutions pages (10) | DONE | SEO/SSG route families plus index pages, navigation, footer, sitemap |
| Integrations directory (public) | DONE | Public provider directory with status/detail content |
| **Legal/company pages (privacy, terms, DPA, security, contact)** | DONE | Metadata-backed routes with CMS-managed public emails, response-time messaging, and effective dates; linked in footer and sitemap |
| Blog/resources/templates preview, i18n, newsletter, UTM, cookie consent, chat/status, demo scheduling | DONE | CMS blog support plus public resources/templates, French locale, newsletter capture, persisted UTM attribution, consent controls, contact chat, live API status and preferred demo scheduling |
| GitHub/Jira integrations (connect/disconnect/health) | DONE | `internal/integrations` plus organization-admin connection/health UI |
| Calendars (Google/Microsoft) | DONE | Authorization-code flows with one-time state, encrypted access/refresh tokens, automatic refresh, event create/update, and two-provider admin UI |
| Observability: structured logs, healthz/readyz, launch gates | DONE | |
| Metrics (`/metrics`), error tracking, tracing, CI dependency scanning | DONE | Prometheus metrics/dependency gauges, CI scanning, W3C trace propagation and configurable authenticated external error/trace JSON exporters |
| Data export/delete (GDPR) | DONE | Tenant export + platform slug-confirmed cross-store purge and tombstone |
| Background jobs / scheduler | DONE | Bounded periodic sweeps for due/overdue and meeting reminders |

---

## Work queue (priority order)

> Work top-down. Each item: contracts first (OpenAPI), tests with the feature,
> audit privileged actions, update this ledger's status when shipped.

### P0 — launch blockers / broken promises

1. ☑ **Quiz engine minimal**: question/answer-key model + server-side grading for single/multiple-choice quiz steps; quiz steps become completable. (§5.3.6, `assignments/service.go:289`) — DONE 2026-07-28 (single-choice only; `internal/journeys/quiz.go`, `internal/assignments/quiz_test.go`)
2. ☑ **Password reset flow**: request-reset (token, hashed, TTL) + reset form on all three portals' login pages. (§6.1:876) — DONE 2026-07-29 (backend atomic consume/all-session revoke plus request/reset UI and login links across employee, organization, and platform portals)
3. ☑ **Email channel**: provider-agnostic mail sender (env-configured SMTP/Resend-style HTTP API); invitations emailed; notification fan-out to email. (§6.5:951) — DONE 2026-07-29 (Resend-compatible sender + log fallback; invitations, reset links, and typed notification email fan-out)
4. ☑ **CSRF protection**: token middleware for cookie-authenticated mutations. (§7.3:997) — DONE 2026-07-28 (`pkg/middleware/csrf.go` double-submit, bearer exempt; api-client sends X-CSRF-Token)
5. ☑ **Audit completeness**: capture IP (RealIP), user-agent, request-id, result/failure into events. (§6.4) — DONE 2026-07-28 (`internal/audit/context.go` middleware; IP/UA/request-id/result on all events; login-failure + org suspend/activate failure paths wired)
6. ☑ **Scheduler primitive**: minimal background ticker with Mongo-backed job records; due-soon + overdue notifications; journey-completed notification. (§6.5, §5.3.3) — DONE 2026-07-28 (`internal/jobs` scheduler + due sweep with dedupe, `{status,dueAt}` index; journey-completed notify on 100%; wired into Run)
7. ☑ **Legal/company marketing pages**: privacy, terms, security, contact (+ footer links). (§5.1.1:277) — DONE 2026-07-28 (static routes with real drafted content, footer Legal column, sitemap; placeholder emails @launchpad.example to replace before launch)
8. ☑ **Assistant UI in employee-web**: ask box, cited answers, feedback buttons. (§5.4.1:771) — DONE 2026-07-28 (`/assistant` chat page with citations + feedback + refusal state)
9. ☑ **Employee lifecycle**: PATCH /employees/{id} (status change), offboarding/archive, buddy field. (§5.3.2) — DONE 2026-07-28 (PATCH route wired, offboarded status + audit, BuddyEmployeeID with reference validation, roster edit UI)
10. ☑ **Manager dashboard**: direct-reports rollup, overdue list, stage view. (§5.3.9) — DONE 2026-07-29 (rollup + overdue + pending approvals via `GET /manager/team`, org-admin `/manager` page, and assignment stage/step inspection with override controls)
11. ☑ **Blockers**: `POST /me/blockers` + manager surfacing. (§5.4.4, §11.2:1522) — DONE 2026-07-28 (`internal/support` blocker records + categorized tickets, `GET /manager/blockers`, employee step-card action; audited)
12. ☑ **Integrations UI** (backend DONE 2026-07-28): org-admin integrations page, nav entry. — DONE 2026-07-28 (`/integrations` provider cards, connect/disconnect/health, nav + user-menu entries)

### P1 — core product depth

13. ☑ Journey versioning: edit published → new draft version, clone, rollback. (§5.3.3) — DONE 2026-07-28 (`CreateNewVersion` copies published steps into editable draft, clone, copy-forward rollback preserving pinned assignments, versions panel + step delete in org UI; routes wired)
14. ☑ Assignment rules: auto-assign on employee create/HRIS sync by role+department. (§5.3.4) — DONE 2026-07-28 (`internal/assignments/rules.go` RuleService + `assignment_rules` collection, `employees.Service.SetRuleApplier` nil-safe port invoked after Create, `POST /assignment-rules/{id}/run`, org `/assignment-rules` page; app.go wiring reported to coordinator)
15. ☑ Analytics depth: department/role breakdowns, overdue rate, AI question reports. (§5.3.10) — DONE 2026-07-28 (`/analytics/onboarding/breakdown`, overdue rate on summary, `/analytics/assistant` report with top refused questions; analytics page extended)
16. ☑ Knowledge org-admin UI. (§5.3.5) — DONE 2026-07-28 (`/knowledge` page with lifecycle actions, 9 source types; knowledge paths added to OpenAPI which was missing them)
17. ☑ Notification typing + deep links. (§6.5) — DONE 2026-07-28 (type + link on all creation sites, badges + click-through in employee notifications)
18. ☑ Employee home today/overdue views; step start/submit endpoints. (§5.4.1, §11.2) — DONE 2026-07-28 (`/step-assignments/{id}/start|submit` wired; Due today + Overdue home sections)
19. ☑ Public integrations directory (marketing /integrations). (§5.1.1:239) — DONE 2026-07-28 (8 live integrations + coming-next calendars, nav/footer/sitemap)
20. ☑ Employee support UI + ticket categories; assistant→ticket escalation. (§5.4.4, §5.4.3:842) — DONE 2026-07-28 (`/support` page with categories, refused-answer 'Create support ticket' action prefilled with the question)

### P2 — enterprise & expansion

21. ☑ MFA (TOTP enroll/verify/challenge). (§7.3:992) — DONE 2026-07-28 (stdlib TOTP RFC 6238, enroll/confirm/disable + login mfaTicket flow, single-use backup codes, Security settings card on both admins, mfaRequired login branch on all portals; enforcement = required-when-enabled, no hard-lock)
22. ☑ Platform staff management (create/deactivate, full role set). (§5.2.6) — DONE 2026-07-28 (full 8-role set + `middleware.RequirePlatformRole` per-route gating, staff CRUD `POST/GET/PATCH /platform/staff` + `/deactivate` + `/reactivate` with temp-password/invite-email provisioning, deactivation blocks staff login, platform-admin `/staff` page; all actions audited; app.go wiring reported to coordinator)
23. ☑ Audited impersonation / support sessions. (§5.2.2:379) — DONE 2026-07-29 (`internal/supportsessions`: mandatory reason, capped duration, read-only short-lived JWT, early revocation, owner notification, impersonation-context audit; platform organization UI; verified with `go test ./internal/supportsessions ./internal/auth ./pkg/middleware ./internal/app`)
24. ☑ Billing: payment provider, invoices, MRR/ARR on dashboard. (§5.2.3) — DONE 2026-07-29 (`internal/billing/paystack`: hosted checkout + verification; signed idempotent webhook settlement; org invoices; MRR/ARR revenue summary + platform billing/dashboard UI; verified with `go test ./internal/billing/... ./internal/platform ./internal/app` and `pnpm --filter @launchpad/platform-admin-web exec tsc --noEmit`)
25. ☑ Meetings & calendar integration. (§5.3.7) — DONE 2026-07-29 (scoped scheduling + rescheduling, journey-step creation, attendance/no-show, notes, employee/manager UI, 24h deduplicated reminder sweep, Google and Microsoft authorization-code flows with one-time state, encrypted refresh tokens + automatic refresh, and provider event create/update; provider-account verification remains environment-dependent per the stop rule; verified in the full Go suite plus org-admin/API-client TypeScript tests)
26. ☑ Equipment & access requests. (§5.3.8) — DONE 2026-07-28 (new `internal/requests` domain: lifecycle pending→approved/rejected→fulfilled + cancel, audited decide/fulfill; employee `/requests` + org-admin queue pages; `equipment_request`/`access_request` journey step types auto-create requests via nil-safe port in assignments; OpenAPI updated; app.go route wiring reported to coordinator)
27. ☑ Assessments full subsystem (banks, randomization, attempts, certificates). (§5.3.6) — DONE 2026-07-29 (`internal/assessments`: reusable draft/publish/archive definitions, single/multiple/true-false/short-answer questions, randomized take view, limited/unlimited attempts, automatic + manager grading, certificate issuance, assignment completion port, org-admin and employee UI; verified with `go test ./internal/assessments ./internal/assignments ./internal/app`)
28. ☑ Observability: /metrics, error tracking, CI dependency scanning. (§7.6) — DONE 2026-07-29 (Prometheus /metrics with route-template labels + dependency gauges; govulncheck blocking + pnpm audit non-blocking in CI; configurable authenticated external error and W3C trace exporters)
29. ☑ Data export/delete (GDPR). (§7.4:1013) — DONE 2026-07-28 (`data.export` permission, org export endpoint, platform purge across 25 store groups incl. MFA + requests with slug-confirmation and platform tombstone)
30. Expansion completion set (split for verifiable handoff):
    - ☑ **30a Organization operations:** search/filter and audited close. — DONE 2026-07-29 (case-insensitive name/slug search + status/plan filters; terminal `closed` lifecycle with owner/admin route, audit event, portal confirmation UI, and login/refresh denial for suspended/closed tenants; verified with `go test ./internal/organizations ./internal/platform ./internal/auth ./internal/app`, API-client 40-test Vitest suite, and platform portal TypeScript check)
    - ☑ **30b Feature-flag rollouts:** percentage, cohorts, expiry, and history. — DONE 2026-07-29 (stable tenant percentage bucketing, explicit user test cohorts, UTC expiry, kill switch, immutable Mongo rollout history and platform editor/history UI; verified with feature-flag/app Go tests, API-client 40-test suite, and platform portal TypeScript check)
    - ☑ **30c Organization setup wizard.** — DONE 2026-07-29 (ten PRD stages, durable monotonic organization progress + audited completion, custom-domain setting, guided org-admin route and nav; verified with organization/app Go tests, API-client suite, and org-admin TypeScript check)
    - ☑ **30d In-session organization switching UX.** — DONE 2026-07-29 (server-authoritative active membership list, audited session rotation with role re-resolution, shared portal switcher, org-admin + employee UX; verified with auth/app Go tests, API-client suite, and both portal TypeScript checks)
    - ☑ **30e Marketing feature and solutions pages.** — DONE 2026-07-29 (10 PRD feature pages + 10 audience solution pages, index hubs, per-page metadata, header/footer links, sitemap coverage; marketing TypeScript and production build pass with all 20 paths statically generated)
    - ☑ **30f Template marketplace.** — DONE 2026-07-29 (`internal/marketplace`: official drafts, customer submissions, categories, moderation, version snapshots, removal, featured state, tenant installation into independent journey drafts, ratings and counters; platform + org UI; Mongo indexes; verified domain/journey/app tests, API-client suite, and both admin TypeScript checks)
    - ☑ **30g SAML enterprise SSO.** — DONE 2026-07-29 (tenant IdP metadata + email mapping, stable SP key/certificate configuration, per-tenant SP metadata and ACS endpoints, SP-initiated redirect flow, one-time RelayState + request-ID binding, signed assertion/audience/time validation via `crewjam/saml` v0.5.1, pre-provisioned federated session issuance, login/admin UI, GDPR purge coverage, tests; live IdP verification remains environment-dependent per the stop rule)

### P3 — reconciliation gaps discovered during takeover

31. ☑ **Platform depth:** ☑ remaining dashboard metrics (MRR, ARR, active subscriptions, overdue/urgent SLA risk plus tenant/lead/support totals); ☑ refund/tax/dunning/coupon billing operations (invoice tax calculation; persistent percentage/fixed coupon catalog with expiry, currency, usage-limit and redemption enforcement; overdue collection attempts; past-due propagation; uncollectible escalation; Paystack provider-side partial/full refunds; audited platform invoice/coupon UI; scheduled dunning; live provider-account verification remains environment-dependent); ☑ break-glass/access reviews (owner-approved reason-required elevation, one-hour maximum, expiry/revocation, effective-role resolution, quarterly attestation and full audit trail; stale JWT privilege bounded by the existing 15-minute access-token TTL); ☑ advanced support (priority-based SLA clocks, overdue/urgent/unassigned/response metrics, public conversation and internal notes, first-response/resolution timing, audited escalation and reassignment); ☑ CMS navigation/blog/FAQ/legal scheduling (typed content, editable public navigation label/order, published navigation API/header integration, and scheduled publication sweep); ☑ security center (role-gated privilege posture metrics, review queue, attestation, and emergency-access controls); ☑ operator jobs/DLQ/delivery/storage surfaces (role-gated scheduler health, failure history, duplicate-run protection, persistent outbound delivery attempts/retries/dead-letter state, audited manual retries, GDPR cleanup, and live Mongo object/data/storage/index metrics).
32. ☑ **Organization depth:** ☑ CSV import; ☑ team/location/mobile contact and invitation management; ☑ journey workflow depth (full step catalog, business-day due dates, parallel groups, prerequisites, approvals, conditional branches, reusable subflows, retry escalation, audited manager overrides and locale translation); ☑ knowledge versions/retention/stale alerts/connectors (immutable history, re-approval versioning, retention policy, deduplicated scheduler alerts, safe connector synchronization and UI); ☑ analytics milestones/drop-off. — DONE 2026-07-29 (verified with the full Go suite, API-client tests, and organization/employee TypeScript checks)
33. ☑ **Cross-cutting/public depth:** ☑ manager/buddy contacts; ☑ provider-gated SMS channel with employee mobile recipients and persistent delivery retries; ☑ audit before/after + actor type (structured mutation metadata is surfaced as `after`, explicit before/after supported); ☑ DPA; ☑ public resources/templates/newsletter/UTM/consent/chat/live status/demo scheduling/French locale; ☑ configurable external error tracking and W3C tracing exporters. — DONE 2026-07-29 (verified with the full Go suite, API-client tests, all portal TypeScript checks, marketing production build, and OpenAPI YAML parse)
34. ☑ **Subscription entitlements and organization drill-down:** server-enforced plan quotas for employees, journey templates, knowledge documents, and integrations; platform tenant profile with live usage/capacity; organization directory detail action and server-backed pagination. — DONE 2026-07-29 (contracts, central entitlement policy, four domain-service guards, platform usage adapter, responsive detail UI, quota and pagination tests; verified with targeted Go tests, the API-client suite, platform TypeScript check, and production build)

## Execution notes

- No `agent_plan.md` existed before this file; `docs/product/prd-status.md` predates it and stays as the summary view.
- Spec roadmap sequencing is stale vs code: SSO/SCIM and custom roles (Phase 3/4) are already DONE.
- No background-job infrastructure exists; every scheduler-dependent spec item routes through P0-6 first.
- External providers (email, payments, calendars) need real accounts to verify end-to-end; implement against documented APIs with sandbox credentials and record verification limits per the stop rule.
