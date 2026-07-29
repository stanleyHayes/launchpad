#!/usr/bin/env node
// Smoke check: validate an OIDC identity provider's discovery document.
//
// Fetches `<issuer>/.well-known/openid-configuration` and asserts:
//   - the issuer URL uses https
//   - the document parses as JSON
//   - required fields exist and are non-empty strings: issuer, jwks_uri,
//     token_endpoint
//   - jwks_uri and token_endpoint are https URLs
//
// Usage:
//   node scripts/smoke-oidc.mjs https://accounts.google.com
//   pnpm smoke:oidc -- https://your-idp.example.com
//
// Exit 0 = discovery document valid. Exit 1 = any assertion failed.
// Read-only: this performs a single unauthenticated GET.

// pnpm/npm may forward a literal "--" separator; strip it.
const args = process.argv.slice(2).filter((a) => a !== "--");
const issuer = args[0];
if (!issuer) {
  console.error("Usage: node scripts/smoke-oidc.mjs <issuer-url>");
  console.error("Example: node scripts/smoke-oidc.mjs https://accounts.google.com");
  process.exit(1);
}

let issuerURL;
try {
  issuerURL = new URL(issuer);
} catch {
  console.error(`FAIL smoke:oidc — "${issuer}" is not a valid URL.`);
  process.exit(1);
}

if (issuerURL.protocol !== "https:") {
  console.error(`FAIL smoke:oidc — issuer must use https, got "${issuerURL.protocol}//...". OIDC over plain http is refused.`);
  process.exit(1);
}

const discoveryURL = `${issuer.replace(/\/+$/, "")}/.well-known/openid-configuration`;

const res = await fetch(discoveryURL, {
  method: "GET",
  headers: { accept: "application/json" },
  signal: AbortSignal.timeout(15_000),
  redirect: "follow",
}).catch((err) => {
  console.error(`FAIL smoke:oidc — GET ${discoveryURL} failed: ${err.message}`);
  process.exit(1);
});

if (res.status !== 200) {
  console.error(`FAIL smoke:oidc — GET ${discoveryURL} returned HTTP ${res.status}.`);
  process.exit(1);
}

const doc = await res.json().catch(() => undefined);
if (!doc || typeof doc !== "object") {
  console.error("FAIL smoke:oidc — discovery response is not valid JSON.");
  process.exit(1);
}

const required = ["issuer", "jwks_uri", "token_endpoint"];
const problems = [];
for (const field of required) {
  if (typeof doc[field] !== "string" || doc[field] === "") {
    problems.push(`missing or empty required field: ${field}`);
  }
}
for (const field of ["jwks_uri", "token_endpoint"]) {
  if (typeof doc[field] === "string" && doc[field] !== "" && !doc[field].startsWith("https://")) {
    problems.push(`${field} is not an https URL`);
  }
}

if (problems.length > 0) {
  for (const p of problems) console.error(`FAIL smoke:oidc — ${p}`);
  process.exit(1);
}

console.log(`PASS smoke:oidc — ${discoveryURL}`);
console.log(`  issuer:         ${doc.issuer}`);
console.log(`  jwks_uri:       ${doc.jwks_uri}`);
console.log(`  token_endpoint: ${doc.token_endpoint}`);
if (doc.issuer !== issuer.replace(/\/+$/, "")) {
  console.log(`  note: document issuer differs from the URL you passed — use "${doc.issuer}" as the configured issuer.`);
}
process.exit(0);
