# Runbooks

Operational procedures for LaunchPad. Each entry is self-contained.

## MongoDB index provisioning (`make migrate-indexes`)

The API ensures its MongoDB indexes at startup (`internal/app/app.go`
`ensureIndexes`, via the shared `app.MongoIndexers` registry). The same
registry backs the standalone provisioning step used when the app is not
(yet) running or when indexes must be created ahead of a deploy:

```bash
make migrate-indexes   # go run ./scripts/migrate_indexes
```

- Requires `MONGODB_URI` / `MONGODB_DATABASE` (see `.env.example`).
- Idempotent — safe to re-run; existing indexes are left untouched.
- If it fails, the API will also fail to boot on the same index set. Check
  connectivity and credentials first (`mongosh "$MONGODB_URI" --eval
  db.adminCommand('ping')`), then re-run.
- **When adding a collection or changing indexes**: add the index set to
  `app.MongoIndexers` — both startup and this script pick it up. Do not add
  indexes only to the script; that drift is what the shared registry prevents.

## SCIM provisioning token rotation

Each organization authenticates SCIM 2.0 calls with a bearer token issued via
`POST /api/v1/organizations/current/scim-token` (org managers only). Tokens
are 32-byte random values stored only as SHA-256 hashes and expire 90 days
after issue (the response includes `expiresAt`) — rotate before expiry and
immediately on suspicion of leakage.

1. **Issue a new token** — call `POST /api/v1/organizations/current/scim-token`
   as an org owner/hr_admin. The plaintext token is shown **once** in the
   response; record it in the secret store.
2. **Update the IdP** — replace the bearer token in the identity provider's
   SCIM provisioning configuration (e.g. Okta/Entra enterprise app).
3. **Verify** — confirm the IdP's next provisioning cycle succeeds
   (`GET /api/v1/scim/v2/Users` with the new token returns 200).
4. **Audit** — token issuance records `scim.token.generated`
   (`contracts/events/audit-events.md`); check `GET /api/v1/audit-events`.

Caveats:

- Issuing a token replaces the organization's stored hash — the **old token
  stops working immediately**. There is no overlap window; schedule a short
  provisioning pause or accept one failed IdP retry cycle.
- A leaked token grants full user/group provisioning for that tenant until
  rotated. There is no self-service revoke beyond rotation.

## Deployment: trusted forwarded headers (H-3)

The API uses chi's `middleware.RealIP` (`internal/app/app.go` `newRouter`),
which rewrites `RemoteAddr` from `X-Forwarded-For` / `X-Real-IP`
**unconditionally**. The per-IP rate limiter keys on that address, so a
client that can set those headers itself can rotate IPs and defeat rate
limiting (login brute-force, SSO callback, lead spam).

**Requirement:** the API must only ever run behind a load balancer / ingress
that **overwrites** (not appends, not passes through) `X-Forwarded-For` and
`X-Real-IP` with the real client IP.

- Never expose the API Service directly to the internet (no `LoadBalancer` /
  host-port bypass around the ingress).
- Verify after any ingress change: `curl -H 'X-Forwarded-For: 1.2.3.4'
  https://<api>/healthz` — the request log's client IP must NOT be `1.2.3.4`.
- If the API must ever be internet-exposed without a trusted proxy, remove
  `chimw.RealIP` first.
