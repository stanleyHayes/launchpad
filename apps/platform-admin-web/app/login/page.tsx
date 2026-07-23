"use client";

import { useState, useTransition, type SyntheticEvent } from "react";
import { useRouter } from "next/navigation";
import { ApiError } from "@launchpad/api-client";
import { AuthShell, Button } from "@launchpad/ui";
import { getClient } from "@/lib/api";
import { clearSession, saveSession } from "@/lib/session";

function formString(form: FormData, key: string): string {
  const value = form.get(key);
  return typeof value === "string" ? value.trim() : "";
}

export default function LoginPage() {
  const router = useRouter();
  const [pending, startTransition] = useTransition();
  const [error, setError] = useState<string | null>(null);

  function onSubmit(event: SyntheticEvent<HTMLFormElement>) {
    event.preventDefault();
    setError(null);

    const form = new FormData(event.currentTarget);
    const email = formString(form, "email");
    const password = formString(form, "password");

    startTransition(() => {
      void (async () => {
        try {
          const client = getClient();
          const result = await client.login({ email, password });
          saveSession(result.tokens.accessToken, result.tokens.refreshToken);

          const profile = await client.me();
          if (!profile.roleCode.startsWith("platform_")) {
            clearSession();
            setError("Platform staff access required");
            return;
          }

          router.replace("/");
        } catch (err) {
          clearSession();
          if (err instanceof ApiError) {
            setError(err.message);
            return;
          }
          setError("Unable to sign in. Try again.");
        }
      })();
    });
  }

  return (
    <AuthShell
      headline="Operate LaunchPad across every customer organization."
      support="Sign in with platform staff credentials to review organizations, leads, and tenant health."
    >
      <div className="lp-card rounded-[28px] p-5 shadow-[0_28px_80px_rgba(6,22,49,0.12)] sm:p-7">
        <p className="text-sm font-semibold uppercase tracking-[0.2em] text-[var(--lp-accent)]">
          LaunchPad
        </p>
        <h1
          className="mt-3 text-3xl font-semibold tracking-tight"
          style={{ fontFamily: "var(--lp-font-display)" }}
        >
          Platform sign-in
        </h1>
        <p className="mt-2 text-sm text-[var(--lp-ink-muted)]">
          Internal control plane access only
        </p>

        <form onSubmit={onSubmit} className="mt-7 space-y-4">
          <label className="block text-sm font-semibold">
            Work email
            <input
              className="lp-input mt-1.5"
              name="email"
              type="email"
              required
              autoComplete="username"
            />
          </label>
          <label className="block text-sm font-semibold">
            Password
            <input
              className="lp-input mt-1.5"
              name="password"
              type="password"
              required
              minLength={10}
              autoComplete="current-password"
            />
          </label>

          {error ? (
            <p
              className="rounded-[var(--lp-radius)] bg-[var(--lp-danger)]/10 px-3 py-2 text-sm text-[var(--lp-danger)]"
              role="alert"
            >
              {error}
            </p>
          ) : null}

          <Button type="submit" disabled={pending} className="w-full">
            {pending ? "Signing in…" : "Sign in"}
          </Button>
        </form>
      </div>
    </AuthShell>
  );
}
