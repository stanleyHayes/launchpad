#!/usr/bin/env node
// Launch readiness gate check (no dependencies).
//
// Runs every launch gate and prints one line per gate: name, status, summary.
// Statuses:
//   PASS  — gate satisfied
//   FAIL  — gate blocked; without --warn-only the script exits 1
//   WARN  — watch item, not blocked (e.g. optional env vars)
//   SKIP  — gate could not run (tool missing); not blocked, but read the note
//
// Usage:
//   node scripts/check-launch-gates.mjs [--warn-only]
//
// --warn-only: always exit 0; use for local dry-runs where services or
// credentials are absent.
//
// Security: this script reports only whether a secret is SET, never its value.

import { spawnSync } from "node:child_process";
import { existsSync, readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const root = join(dirname(fileURLToPath(import.meta.url)), "..");
const warnOnly = process.argv.includes("--warn-only");

const STATUS = { PASS: "PASS", FAIL: "FAIL", WARN: "WARN", SKIP: "SKIP" };
const results = [];

function report(name, status, summary) {
  results.push({ name, status, summary });
  console.log(`[${status}] ${name} — ${summary}`);
}

function run(name, command, args) {
  const proc = spawnSync(command, args, {
    cwd: root,
    encoding: "utf8",
    shell: false,
    timeout: 15 * 60 * 1000,
  });
  if (proc.error) {
    report(name, STATUS.FAIL, `failed to run ${command}: ${proc.error.message}`);
    return;
  }
  if (proc.status === 0) {
    report(name, STATUS.PASS, `\`${command} ${args.join(" ")}\` exited 0`);
    return;
  }
  const tail = `${proc.stdout}\n${proc.stderr}`
    .trim()
    .split("\n")
    .slice(-8)
    .join("\n");
  report(name, STATUS.FAIL, `\`${command} ${args.join(" ")}\` exited ${proc.status}`);
  console.log(`  --- last output lines ---\n${tail}\n  -------------------------`);
}

// --- Gate: environment variables -------------------------------------------
// Present means set in the process environment OR defined in ./.env.
// Values are never read into output — only key presence is checked.

function envKeysFromDotEnv() {
  const path = join(root, ".env");
  if (!existsSync(path)) return new Set();
  const keys = new Set();
  for (const line of readFileSync(path, "utf8").split("\n")) {
    const match = /^\s*(?:export\s+)?([A-Za-z_][A-Za-z0-9_]*)\s*=/.exec(line);
    if (match) keys.add(match[1]);
  }
  return keys;
}

function checkEnv() {
  const dotenv = envKeysFromDotEnv();
  const isSet = (key) =>
    (process.env[key] !== undefined && process.env[key] !== "") || dotenv.has(key);

  const required = ["JWT_SECRET", "MONGODB_URI", "REDIS_URL"];
  const watch = ["ANTHROPIC_API_KEY", "ENCRYPTION_KEY"];

  const missingRequired = required.filter((k) => !isSet(k));
  const missingWatch = watch.filter((k) => !isSet(k));
  const setList = [...required, ...watch].filter(isSet);

  if (missingRequired.length > 0) {
    report(
      "env: required variables",
      STATUS.FAIL,
      `missing: ${missingRequired.join(", ")} (set: ${setList.join(", ") || "none"}). Values intentionally not printed.`,
    );
  } else {
    report(
      "env: required variables",
      STATUS.PASS,
      `set: ${required.join(", ")} (values intentionally not printed)`,
    );
  }

  if (missingWatch.length > 0) {
    report(
      "env: watch variables",
      STATUS.WARN,
      `not set (watch, not blocked): ${missingWatch.join(", ")} — ANTHROPIC_API_KEY powers /assistant/ask; ENCRYPTION_KEY encrypts tenant secrets at rest`,
    );
  } else {
    report("env: watch variables", STATUS.PASS, `set: ${watch.join(", ")}`);
  }
}

// --- Gate: golangci-lint (skipped if the binary is absent) ------------------

function checkLint() {
  const probe = spawnSync("golangci-lint", ["version"], { encoding: "utf8" });
  if (probe.error) {
    report(
      "golangci-lint",
      STATUS.SKIP,
      "golangci-lint binary not found on PATH — install it (see Makefile / CI) and re-run; lint gate NOT evaluated",
    );
    return;
  }
  run("golangci-lint", "golangci-lint", ["run", "./..."]);
}

// --- Run all gates ----------------------------------------------------------

console.log(`Launch gates — ${new Date().toISOString()}${warnOnly ? " (--warn-only)" : ""}\n`);

checkEnv();
run("go build", "go", ["build", "./..."]);
run("go test", "go", ["test", "./..."]);
checkLint();
run("pnpm test", "pnpm", ["test"]);
run("migrate-indexes compiles", "go", ["vet", "./scripts/migrate_indexes"]);

const failed = results.filter((r) => r.status === STATUS.FAIL);
const warned = results.filter((r) => r.status === STATUS.WARN);
const skipped = results.filter((r) => r.status === STATUS.SKIP);

console.log(
  `\nSummary: ${results.filter((r) => r.status === STATUS.PASS).length} passed, ` +
    `${failed.length} failed, ${warned.length} warnings, ${skipped.length} skipped`,
);

if (failed.length > 0) {
  console.log(`Blocked gates: ${failed.map((r) => r.name).join(", ")}`);
  if (warnOnly) {
    console.log("--warn-only set: exiting 0 despite blocked gates.");
  } else {
    process.exit(1);
  }
}
