# Audit event catalog

Audit events are the de-facto domain events of LaunchPad. Every mutating use
case records one through `audit.Service.Record` (`internal/audit/audit.go`).
This catalog is the contract for those events — keep it in sync when adding or
changing `audit.Record` call sites (AGENTS.md workflow step 2).

## Event shape

`audit.Event` (`internal/audit/audit.go:10-19`), also the `GET /api/v1/audit-events`
response item:

```json
{
  "id": "string",
  "organizationId": "string, omitted for platform-scoped events",
  "actorUserId": "string",
  "action": "string (see catalog below)",
  "resourceType": "string",
  "resourceId": "string",
  "metadata": "object, optional, action-specific",
  "createdAt": "RFC3339 timestamp"
}
```

Events are immutable and append-only. `organizationId` is omitted for
platform-scoped events (global feature flags, billing plans, platform logins);
tenant events always carry it. SCIM-actor events use a synthetic SCIM actor id
rather than a human user id.

## Action catalog

Naming: `<resource>.<past-tense verb>` (dot-separated, snake_case segments).

### auth — `internal/auth/`

| Action | Resource type | Metadata | Recorded at |
| --- | --- | --- | --- |
| `auth.register` | `organization` | `{email, slug}` | `service.go` Register |
| `auth.login` | `user` | — | `service.go` Login (org and platform) |
| `auth.sso_login` | `user` | — | `service.go` SSO login |
| `auth.invitation.issued` | `user` | `{email, role}` | `invitations.go` IssueInvitation |
| `auth.invitation.accepted` | `user` | — | `invitations.go` AcceptInvitation |

### organizations & platform — `internal/organizations/`, `internal/platform/`

| Action | Resource type | Metadata | Recorded at |
| --- | --- | --- | --- |
| `organization.updated` | `organization` | — | `organizations/handlers.go` HandleUpdate |
| `organization.suspended` | `organization` | — | `platform/handlers.go` |
| `organization.activated` | `organization` | — | `platform/handlers.go` |
| `staff.created` | `platform_staff` | `{email, roleCode}` | `platform/handlers.go` (platform-scoped) |
| `staff.role_updated` | `platform_staff` | `{roleCode}` | `platform/handlers.go` (platform-scoped) |
| `staff.deactivated` | `platform_staff` | `{roleCode}` | `platform/handlers.go` (platform-scoped) |
| `staff.reactivated` | `platform_staff` | `{roleCode}` | `platform/handlers.go` (platform-scoped) |
| `membership.invited` | `membership` | `{roleCode, email}` | `organizations/handlers.go` HandleInviteMember |

### org structure — `internal/departments/`, `internal/employees/`

| Action | Resource type | Metadata | Recorded at |
| --- | --- | --- | --- |
| `department.created` | `department` | `{name}` | `departments/handlers.go` |
| `job_role.created` | `job_role` | `{name}` | `departments/handlers.go` |
| `employee.created` | `employee` | `{workEmail}` | `employees/handlers.go` HandleCreate |
| `employee.provisioned` | `employee` | `{userId}` | `employees/handlers.go` HandleProvisionAccess |

### journeys & assignments — `internal/journeys/`, `internal/assignments/`

| Action | Resource type | Metadata | Recorded at |
| --- | --- | --- | --- |
| `journey.created` | `journey_template` | `{name}` | `journeys/handlers.go` HandleCreate |
| `journey.step_added` | `journey_step` | `{title, stepType}` | `journeys/handlers.go` HandleAddStep |
| `journey.published` | `journey_template` | — | `journeys/handlers.go` HandlePublish |
| `assignment.created` | `journey_assignment` | `{employeeId}` | `assignments/handlers.go` HandleAssign |

### knowledge & CMS — `internal/knowledge/`, `internal/cms/`

| Action | Resource type | Metadata | Recorded at |
| --- | --- | --- | --- |
| `knowledge_document.created` | `knowledge_document` | — | `knowledge/handlers.go` |
| `knowledge_document.updated` | `knowledge_document` | — | `knowledge/handlers.go` |
| `knowledge_document.approved` | `knowledge_document` | — | `knowledge/handlers.go` |
| `knowledge_document.indexed` | `knowledge_document` | — | `knowledge/handlers.go` |
| `knowledge_document.archived` | `knowledge_document` | — | `knowledge/handlers.go` |
| `cms_page.created` | `cms_page` | — | `cms/handlers.go` |
| `cms_page.updated` | `cms_page` | — | `cms/handlers.go` |
| `cms_page.published` | `cms_page` | — | `cms/handlers.go` |

### billing & feature flags — `internal/billing/`, `internal/featureflags/`

| Action | Resource type | Metadata | Recorded at |
| --- | --- | --- | --- |
| `billing_plan.created` | `billing_plan` | — | `billing/handlers.go` (platform-scoped) |
| `billing_plan.updated` | `billing_plan` | — | `billing/handlers.go` (platform-scoped) |
| `subscription.updated` | `subscription` | `{planCode, status}` | `billing/handlers.go` |
| `feature_flag.created` | `feature_flag` | — | `featureflags/handlers.go` (platform-scoped) |
| `feature_flag.updated` | `feature_flag` | — | `featureflags/handlers.go` (platform-scoped) |
| `feature_flag_override.set` | `feature_flag` | — | `featureflags/handlers.go` (tenant-scoped) |

### support & HRIS — `internal/support/`, `internal/hris/`

| Action | Resource type | Metadata | Recorded at |
| --- | --- | --- | --- |
| `support_ticket.status_updated` | `support_ticket` | `{status}` | `support/handlers.go` |
| `hris.directory.applied` | `hris` | `{total, created, skipped, failed}` | `hris/service.go` ApplyDirectory |

### SCIM — `internal/scim/`

| Action | Resource type | Metadata | Recorded at |
| --- | --- | --- | --- |
| `scim.token.generated` | `organization` | — | `scim/service.go` IssueToken |
| `scim.user.provisioned` | `user` | `{userName, active}` | `scim/service.go` CreateUser |
| `scim.user.updated` | `user` | `{active}` | `scim/service.go` Replace/PatchUser |
| `scim.user.deprovisioned` | `user` | `{userName}` | `scim/service.go` DeleteUser |
| `scim.group.created` | `group` | `{displayName, members}` | `scim/groups_service.go` |
| `scim.group.updated` | `group` | `{displayName, members}` | `scim/groups_service.go` |
| `scim.group.deleted` | `group` | — | `scim/groups_service.go` |

## Consumer notes

- Query per tenant via `GET /api/v1/audit-events` (managers only); newest
  first, limit clamped to 100 (`internal/audit/mongo/store.go`).
- Treat `action` as an enum that only grows: new actions may appear at any
  deploy; existing action strings are never renamed or reused for a different
  meaning.
- `metadata` is best-effort context for humans; do not build integrations that
  depend on its exact keys.
