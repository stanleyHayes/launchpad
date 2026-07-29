# PRD implementation status map

Tracks `LaunchPad_Complete_PRD_and_Build_Spec.md` sections against the code
(as of 2026-07-28). Statuses: **Implemented** (shipped end-to-end),
**Partial** (core exists, scope reduced), **Planned** (no code yet).
Update this map whenever a section changes status.

## 3. Product scope

| PRD section | Status | Notes |
| --- | --- | --- |
| 3.1 In Scope — multi-tenancy, org & employee management, journeys, public site, platform admin, analytics | Implemented | See per-surface rows below. |
| 3.1 — Meetings and scheduling | Planned | No scheduling code. |
| 3.1 — Assessments | Partial | `quiz` exists as a journey step type (`internal/journeys/types.go`); no question bank, scoring is client-supplied (see review H-1). |
| 3.1 — Notifications: in-app, Slack, Teams | Implemented | `internal/notifications` + webhook dispatcher. |
| 3.1 — Notifications: email | Planned | No email sender. |
| 3.1 — AI knowledge assistant | Implemented | `internal/assistant` + `internal/knowledge`, citation-grounded. |
| 3.1 — Integrations: HRIS | Implemented | BambooHR (`internal/hris`). |
| 3.1 — Integrations: identity providers | Implemented | OIDC SSO (`internal/sso`) + SCIM 2.0 (`internal/scim`). |
| 3.1 — Integrations: calendars, communication platforms, source control, project management | Planned | Only Slack/Teams *outbound webhooks* exist; no calendar, SCM, or PM integrations. |

## 4. User types

| Persona | Status | Notes |
| --- | --- | --- |
| 4.1 Public visitor, 4.2 Platform super admin, 4.3 Organization owner, 4.4 HR administrator, 4.9 New employee | Implemented | Roles: `platform_*`, `organization_owner`, `hr_admin`, `employee`. |
| 4.5 IT administrator, 4.6 Security/compliance administrator, 4.7 Manager/team lead, 4.8 Buddy/mentor, 4.10 Executive viewer | Partial | No distinct role codes; managers act via `hr_admin`/owner and the approval flow. |

## 5. Major product surfaces

| PRD section | Status | Notes |
| --- | --- | --- |
| 5.1 Public marketing website (pages, pricing, 5.1.3 lead capture, 5.1.4 content admin) | Implemented | `apps/marketing-web`, `internal/cms`, `internal/leads`. |
| 5.2.1 Platform dashboard | Implemented | `GET /platform/overview`. |
| 5.2.2 Organization management | Partial | List/get/suspend/activate implemented; **support impersonation is Planned** — no impersonation mode exists. |
| 5.2.3 Subscription and billing operations | Implemented | Plans + subscriptions (`internal/billing`); no payment provider. |
| 5.2.4 Feature flag management | Implemented | Global flags + per-tenant overrides (`internal/featureflags`). |
| 5.2.5 Global template marketplace | Planned | No marketplace; journey templates are per-tenant only. |
| 5.2.6 Platform user and staff management | Implemented | Full 8-role set with per-route gating (`middleware.RequirePlatformRole`); staff CRUD `/platform/staff` (temp password once or invite email); deactivate/reactivate blocks staff login; platform-admin `/staff` UI. MFA hard-lock, break-glass accounts, and access-review reports remain open. |
| 5.2.7 Customer support management | Implemented | Ticket triage/status (`internal/support`). |
| 5.2.8 Platform content management | Implemented | CMS draft/publish. |
| 5.2.9 Platform security center / 5.2.10 Platform operations | Planned | Audit events exist (`internal/audit`); no security-center UI. |
| 5.3.1 Organization setup wizard | Partial | Signup + settings update; no guided wizard. |
| 5.3.2 Employee directory | Implemented | `internal/employees` (+ HRIS sync/apply). |
| 5.3.3 Journey template builder | Implemented | Draft/publish versioning (`internal/journeys`); step types: document, quiz, task, approval. Drag-and-drop UI not present. |
| 5.3.4 Assignment rules | Partial | Manual + department-scoped assignment shipped (`internal/assignments`: assign to a department, fans out to its active employees); no rules engine. |
| 5.3.5 Content and knowledge management | Implemented | `internal/knowledge` approve → index lifecycle. |
| 5.3.6 Assessments | Planned | Beyond the `quiz` step type, nothing exists. |
| 5.3.7 Meetings and scheduling | Planned | No code. |
| 5.3.8 Equipment and access requests | Planned | No code. |
| 5.3.9 Manager dashboard | Partial | Approvals list + onboarding analytics. |
| 5.3.10 Analytics and reports | Partial | Onboarding summary (`internal/analytics`); no custom reports. |
| 5.3.11 Organization settings | Implemented | Profile, branding, SSO, HRIS, SCIM token, invitations, notification channels. |
| 5.4.1 Employee home / 5.4.2 Guided journey | Implemented | Assignments, step completion, due dates. |
| 5.4.3 AI onboarding assistant | Implemented | `POST /assistant/ask` with citations and refusals. |
| 5.4.4 Employee support | Implemented | Org-side ticket creation/view. |

## 6. Functional requirements by domain

| PRD section | Status | Notes |
| --- | --- | --- |
| 6.1 Authentication and identity | Implemented | Password auth, JWT sessions, invitations, OIDC SSO, SCIM. MFA flag exists on users but no MFA flow. |
| 6.2 Multi-tenancy | Implemented | Org-scoped repositories throughout. |
| 6.3 RBAC and permission model | Implemented | Server-side checks plus role-gated logins/nav in all three portals (M-19 closed). Custom roles ship as an enterprise-plan-gated capability. |
| 6.4 Audit logging | Implemented | `internal/audit`; catalog in `contracts/events/audit-events.md`. |
| 6.5 Notifications | Partial | In-app + Slack/Teams; email planned. |
