#!/usr/bin/env node
// Smoke check: validate ANTHROPIC_API_KEY against the Anthropic API.
//
// This is a METADATA call only: `GET /v1/models` lists available models. It
// never creates a completion/message, so it costs nothing (no tokens billed).
// Anthropic has no test/live key distinction — every key can spend money —
// which is exactly why this script refuses to make any billed call.
//
// Usage:
//   ANTHROPIC_API_KEY=sk-ant-... node scripts/smoke-anthropic.mjs
//   pnpm smoke:anthropic            (reads ANTHROPIC_API_KEY from env or .env)
//
// Exit 0 = key accepted by the API. Exit 1 = missing key, auth failure, or
// unreachable API. The key is never printed.

import { existsSync, readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const root = join(dirname(fileURLToPath(import.meta.url)), "..");

function keyFromDotEnv() {
  const path = join(root, ".env");
  if (!existsSync(path)) return undefined;
  for (const line of readFileSync(path, "utf8").split("\n")) {
    const match = /^\s*(?:export\s+)?ANTHROPIC_API_KEY\s*=\s*(.*)$/.exec(line);
    if (match) return match[1].trim().replace(/^["']|["']$/g, "");
  }
  return undefined;
}

const apiKey = process.env.ANTHROPIC_API_KEY || keyFromDotEnv();
if (!apiKey) {
  console.error("FAIL smoke:anthropic — ANTHROPIC_API_KEY is not set (env or .env).");
  process.exit(1);
}

const baseURL = (process.env.ANTHROPIC_BASE_URL || "https://api.anthropic.com").replace(/\/+$/, "");

const res = await fetch(`${baseURL}/v1/models`, {
  method: "GET",
  headers: {
    "x-api-key": apiKey,
    "anthropic-version": "2023-06-01",
  },
  signal: AbortSignal.timeout(15_000),
}).catch((err) => {
  console.error(`FAIL smoke:anthropic — request to ${baseURL}/v1/models failed: ${err.message}`);
  process.exit(1);
});

if (res.status === 200) {
  const body = await res.json();
  const count = Array.isArray(body.data) ? body.data.length : "?";
  console.log(`PASS smoke:anthropic — GET /v1/models returned 200 (${count} models visible to this key).`);
  console.log("Note: metadata call only; no completion was created and no tokens were billed.");
  process.exit(0);
}

const detail = (await res.text()).slice(0, 200);
if (res.status === 401 || res.status === 403) {
  console.error(`FAIL smoke:anthropic — key rejected (HTTP ${res.status}). Rotate or re-issue ANTHROPIC_API_KEY.`);
} else {
  console.error(`FAIL smoke:anthropic — unexpected HTTP ${res.status}: ${detail}`);
}
process.exit(1);
