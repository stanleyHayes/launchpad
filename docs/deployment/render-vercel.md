# Render and Vercel deployment

This repository is prepared for:

- one Docker API web service on Render;
- one Render Key Value instance for sessions and transient cache state;
- MongoDB Atlas (or another externally hosted MongoDB) for durable data;
- four independent Vercel projects for the Next.js frontends.

The checked-in [`render.yaml`](../../render.yaml) uses free Render instances.
That is suitable for demos and acceptance testing, not a dependable production
SLA. Free web services sleep after 15 minutes without inbound traffic and can
take about a minute to wake. Free Key Value has no persistence, so a restart
invalidates sessions and clears transient state. Upgrade the API and Key Value
plans before relying on the platform for production workloads.

## 1. Prepare MongoDB

Render does not offer managed MongoDB. Create a MongoDB Atlas deployment,
create a least-privilege database user, allow connections from the Render
service, and copy its connection string. The database name defaults to
`launchpad`.

Do not commit the connection string. Enter it as `MONGODB_URI` when Render
prompts during Blueprint creation.

## 2. Create the Render Blueprint

1. In Render, choose **New > Blueprint** and connect this repository.
2. Select `render.yaml`.
3. Supply every required value marked `sync: false`.
4. Leave optional providers blank if they are not in use.
5. Create the Blueprint and wait for `/readyz` to become healthy.

Minimum values:

| Variable | Value |
| --- | --- |
| `MONGODB_URI` | MongoDB Atlas connection string |
| `CORS_ORIGINS` | comma-separated production frontend origins |
| `CORS_ORIGIN_PATTERNS` | comma-separated, project-scoped Vercel preview patterns |
| `API_PUBLIC_URL` | `https://launchpad-api.onrender.com` or the API custom domain |
| `ORG_ADMIN_URL` | production organization-admin URL |
| `PLATFORM_OWNER_EMAIL` | initial platform owner email |
| `PLATFORM_OWNER_PASSWORD` | strong initial password |

Example origin configuration after the Vercel projects have final names:

```dotenv
CORS_ORIGINS=https://launchpad.example,https://platform.launchpad.example,https://organization.launchpad.example,https://employee.launchpad.example
CORS_ORIGIN_PATTERNS=https://launchpad-marketing-*.vercel.app,https://launchpad-platform-*.vercel.app,https://launchpad-organization-*.vercel.app,https://launchpad-employee-*.vercel.app
```

Never use `https://*.vercel.app`; it would trust deployments owned by other
Vercel users. If previews are not required, leave `CORS_ORIGIN_PATTERNS` blank.

The Blueprint generates `JWT_SECRET` and `ENCRYPTION_KEY`. Provider credentials
(Anthropic, Resend, SMS, calendars, SAML, Paystack, and observability) stay
dashboard-managed secrets.

## 3. Create four Vercel projects

Import this Git repository four times. Set each project's Root Directory:

| Vercel project | Root Directory |
| --- | --- |
| Marketing | `apps/marketing-web` |
| Platform admin | `apps/platform-admin-web` |
| Organization admin | `apps/organization-admin-web` |
| Employee portal | `apps/employee-web` |

Keep **Include source files outside of the Root Directory** enabled because the
apps consume `packages/ui` and `packages/api-client`. Vercel detects the root
`pnpm-lock.yaml`, `pnpm-workspace.yaml`, and pinned `pnpm@11.16.0`.

The app-local `vercel.json` files select the Next.js framework. No custom output
directory is required.

## 4. Configure Vercel environment variables

Set these for both Production and Preview, then redeploy:

| Project | Variables |
| --- | --- |
| Marketing | `NEXT_PUBLIC_API_BASE_URL`, `NEXT_PUBLIC_ORG_ADMIN_URL`, `NEXT_PUBLIC_MARKETING_URL` |
| Platform admin | `NEXT_PUBLIC_API_BASE_URL` |
| Organization admin | `NEXT_PUBLIC_API_BASE_URL`, `NEXT_PUBLIC_MARKETING_URL` |
| Employee portal | `NEXT_PUBLIC_API_BASE_URL`, `NEXT_PUBLIC_MARKETING_URL` |

`NEXT_PUBLIC_API_BASE_URL` must be the HTTPS Render API URL without `/api/v1`.
The frontend client adds API paths itself.

Use the production marketing URL for `NEXT_PUBLIC_MARKETING_URL` in both
Production and Preview unless preview-to-preview navigation is specifically
needed.

## 5. Final alignment

After Vercel assigns the final project domains:

1. Update `CORS_ORIGINS` and `CORS_ORIGIN_PATTERNS` in Render.
2. Set `ORG_ADMIN_URL` to the organization-admin production domain.
3. Set `API_PUBLIC_URL` to the Render API domain.
4. Register `${API_PUBLIC_URL}/api/v1/calendar/oauth/callback` with Google and
   Microsoft if calendar OAuth is enabled.
5. Configure Paystack's webhook against the API domain if payments are enabled.
6. Redeploy the API so all values are active.

## 6. Verify

```bash
curl -fsS https://<api-host>/healthz
curl -fsS https://<api-host>/readyz
curl -i -X OPTIONS https://<api-host>/api/v1/auth/login \
  -H 'Origin: https://<frontend-host>' \
  -H 'Access-Control-Request-Method: POST'
```

The preflight response should echo the approved frontend origin in
`Access-Control-Allow-Origin`. Then verify login in each portal, invitation
links, marketing contact/demo submission, session refresh, and logout.

For a production launch, move Render API and Key Value to persistent,
non-sleeping paid plans and configure backups/alerts for MongoDB Atlas.
