"use client";

import Link from "next/link";
import { useState, useTransition, type SyntheticEvent } from "react";
import { useRouter } from "next/navigation";
import { ApiError } from "@launchpad/api-client";
import { AuthShell, Button, FormField, LogoTile } from "@launchpad/ui";
import { getClient } from "@/lib/api";
import { marketingUrl } from "@/lib/env";
import { clearSession, saveSession } from "@/lib/session";

function formString(form: FormData, key: string): string {
  const value = form.get(key);
  return typeof value === "string" ? value.trim() : "";
}

export default function LoginPage() {
  const router = useRouter();
  const [pending, startTransition] = useTransition();
  const [error, setError] = useState<string | null>(null);
  const [mfaTicket, setMFATicket] = useState<string | null>(null);

  // finishSignIn runs the membership gate and lands on the portal.
  async function finishSignIn(): Promise<void> {
    const profile = await getClient().me();
    if (!profile.organization) {
      clearSession();
      setError("This account is not a member of an organization");
      return;
    }

    saveSession();
    router.replace("/");
  }

  function onSubmit(event: SyntheticEvent<HTMLFormElement>) {
    event.preventDefault();
    setError(null);

    const form = new FormData(event.currentTarget);
    const email = formString(form, "email");
    const password = formString(form, "password");

    startTransition(() => {
      void (async () => {
        try {
          const result = await getClient().login({ email, password });
          if ("mfaRequired" in result) {
            setMFATicket(result.mfaTicket);
            return;
          }

          await finishSignIn();
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

  function onMFASubmit(event: SyntheticEvent<HTMLFormElement>) {
    event.preventDefault();
    setError(null);

    const code = formString(new FormData(event.currentTarget), "code");
    const ticket = mfaTicket;
    if (!ticket) {
      return;
    }

    startTransition(() => {
      void (async () => {
        try {
          await getClient().loginMFA({ ticket, code });
          await finishSignIn();
        } catch (err) {
          clearSession();
          if (err instanceof ApiError) {
            setError(err.message);
            return;
          }
          setError("Unable to verify the code. Try again.");
        }
      })();
    });
  }

  return (
    <AuthShell
      headline="Your onboarding journey, step by step."
      support="Sign in to view assignments, complete steps, and track your progress."
    >
      <div className="lp-card rounded-[28px] p-5 shadow-[0_28px_80px_rgba(6,22,49,0.12)] sm:p-7">
        <LogoTile size={36} />
        <h1
          className="mt-3 text-3xl font-semibold tracking-tight"
          style={{ fontFamily: "var(--lp-font-display)" }}
        >
          Welcome back
        </h1>
        <p className="mt-2 text-sm text-[var(--lp-ink-muted)]">
          Employee sign-in
        </p>

        {mfaTicket ? (
          <form onSubmit={onMFASubmit} className="mt-7 space-y-4">
            <p className="text-sm text-[var(--lp-ink-muted)]">
              Two-factor authentication is enabled on this account. Enter the 6-digit code from your
              authenticator app, or one of your backup codes.
            </p>
            <FormField
              label="Authentication code"
              name="code"
              type="text"
              required
              startIcon="shield"
              autoComplete="one-time-code"
              placeholder="123456"
            />

            {error ? (
              <p
                className="rounded-[var(--lp-radius)] bg-[var(--lp-danger)]/10 px-3 py-2 text-sm text-[var(--lp-danger)]"
                role="alert"
              >
                {error}
              </p>
            ) : null}

            <Button type="submit" disabled={pending} className="w-full">
              {pending ? "Verifying…" : "Verify and sign in"}
            </Button>
            <button
              type="button"
              onClick={() => {
                setMFATicket(null);
                setError(null);
              }}
              className="w-full text-sm text-[var(--lp-ink-muted)] hover:text-[var(--lp-ink)]"
            >
              Back to sign-in
            </button>
          </form>
        ) : (
          <form onSubmit={onSubmit} className="mt-7 space-y-4">
            <FormField
              label="Work email"
              name="email"
              type="email"
              required
              startIcon="mail"
              autoComplete="username"
              placeholder="you@company.com"
            />
            <FormField
              label="Password"
              name="password"
              type="password"
              required
              minLength={10}
              startIcon="lock"
              autoComplete="current-password"
              placeholder="Your password"
            />
            <div className="text-right">
              <Link href="/forgot-password" className="text-sm text-[var(--lp-accent)]">Forgot password?</Link>
            </div>

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
        )}

        <p className="mt-6 text-sm text-[var(--lp-ink-muted)]">
          New to LaunchPad?{" "}
          <Link href={`${marketingUrl}/signup`} className="text-[var(--lp-accent)]">
            Start a free trial
          </Link>
        </p>
      </div>
    </AuthShell>
  );
}
