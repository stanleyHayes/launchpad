# Security documentation

How LaunchPad handles credentials and tenant secrets. Platform conventions:
secrets live in environment variables only (`AGENTS.md`), tokens are stored
hashed, and tenant third-party credentials are write-only in the API.

## Environment secrets (platform operators)

- `JWT_SECRET` signs all access tokens (HS256). The dev default
  `local-dev-only-change-me` is rejected outside `APP_ENV=local`; use a long
  random value (≥32 bytes) everywhere else.
- `MONGODB_URI` / `REDIS_URL` carry database credentials.
- `PLATFORM_OWNER_*` bootstraps the first platform account — leave empty in
  production after bootstrap.
- Local development uses `${VAR:-launchpad}`-style defaults in
  `docker-compose.yml`; these are dev dummies, never production values.

## SCIM bearer tokens

- Issued per organization via `POST /api/v1/organizations/current/scim-token`
  (org managers only); plaintext is shown **once**.
- 32 bytes from `crypto/rand`; only the SHA-256 hash is stored
  (`internal/scim`), so a database dump does not reveal usable tokens.
- Grant full SCIM user/group provisioning for that tenant. Tokens expire
  **90 days** after issue (`scim.TokenTTL`); expiry is enforced in
  `ResolveOrganization` with the same error as an invalid token (no expiry
  oracle). Tokens issued before expiry existed carry no expiry and stay valid
  until rotated — regenerate them to pick up a TTL. Issuance is audit-logged
  as `scim.token.generated` and every authenticated use as `scim.token.used`.

## SSO client secrets

- Per-tenant OIDC configuration (`PUT /api/v1/organizations/current/sso`)
  includes the IdP `clientSecret`.
- The secret is **write-only**: `json:"-"` keeps it out of every API response,
  including the tenant's own `GET` — there is no read-back path.
- Verification is RS256-only with issuer/audience/expiry/nonce checks and
  `email_verified:false` refusal (`internal/sso/oidc`); SSO state is
  single-use via Redis `GETDEL`.
- At rest the secret is envelope-encrypted when `ENCRYPTION_KEY` is set (see
  *Tenant-secret encryption at rest* below); without a key it is stored in
  plaintext, relying on MongoDB encryption-at-rest and access controls.

## Outbound webhook URLs (Slack / Teams)

- A chat webhook URL is a bearer credential — anyone holding it can post to
  the channel — so it is treated like a secret (`internal/notifications`).
- **Never returned by the API**: reads expose only `slackConfigured` /
  `teamsConfigured` (`ChannelStatus`).
- Validated on write: https only, host must be on the Slack/Teams allowlist.
- Delivery hardening (`internal/notifications/webhook/dispatcher.go`): no
  redirect following (SSRF bypass prevention), 5s timeout, response body
  discarded, and URLs are stripped from logged errors (`redactURL`).

## SSRF defenses for tenant-controlled URLs

All outbound calls to tenant-configured endpoints (OIDC issuer endpoints,
HRIS base URL) go through `pkg/safehttp`: dial-time post-DNS IP rejection
(private/loopback/link-local ranges) and no redirects.

## Token hygiene summary

| Secret | Storage | TTL |
| --- | --- | --- |
| Access token (JWT) | client-side; signed, not stored | 15m default |
| Refresh token | SHA-256 hash in Redis, rotated on use | 7d default |
| Invitation token | SHA-256 hash, single-use, TTL index | expires |
| SSO state | Redis, consumed via `GETDEL` | single-use |
| SCIM bearer token | SHA-256 hash in MongoDB | 90 days (legacy tokens: none until rotated) |

## Tenant-secret encryption at rest

Tenant third-party credentials — HRIS API tokens (`internal/hris`), SSO
client secrets (`internal/sso`), and Slack/Teams webhook URLs
(`internal/notifications`) — are encrypted at the Mongo store layer with
AES-256-GCM (`pkg/security` `SecretCipher`), keyed by the `ENCRYPTION_KEY`
environment variable (base64 of 32 bytes; generate with
`openssl rand -base64 32`). The nonce is prepended to the ciphertext and the
result is base64-encoded behind an `enc:v1:` prefix, so ciphertext is
distinguishable from plaintext. Key rotation therefore means: decrypt with
the old key, re-encrypt with the new one (no dual-key support yet).

Operational behavior:

- `ENCRYPTION_KEY` **set**: new writes are encrypted; reads decrypt `enc:v1:`
  values and pass anything else through unchanged, so rows written before the
  key was configured keep working (and are re-encrypted on their next write).
- `ENCRYPTION_KEY` **unset**: values are stored and read as plaintext
  (historical behavior); a one-time `slog.Warn` is logged. If a database
  already contains `enc:v1:` values, reads fail closed until the key is
  restored.
- The key is a process secret: keep it in the environment/secret manager,
  never in the database or source. Losing it makes stored ciphertext
  unrecoverable.
