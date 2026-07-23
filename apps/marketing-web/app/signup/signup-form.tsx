"use client";

import Link from "next/link";
import { useState, useTransition, type SyntheticEvent } from "react";
import { ApiError, createLaunchPadClient } from "@launchpad/api-client";
import { Container } from "@launchpad/ui";

const apiBaseUrl =
  process.env.NEXT_PUBLIC_API_BASE_URL ?? "http://localhost:8080";
const orgAdminUrl =
  process.env.NEXT_PUBLIC_ORG_ADMIN_URL ?? "http://localhost:3002";

function formString(form: FormData, key: string): string {
  const value = form.get(key);
  return typeof value === "string" ? value.trim() : "";
}

function Field({
  label,
  name,
  type,
  required,
  minLength,
}: {
  label: string;
  name: string;
  type: string;
  required?: boolean;
  minLength?: number;
}) {
  return (
    <label className="block text-sm text-[var(--lp-ink)]">
      {label}
      <input
        className="mt-1 w-full rounded-[var(--lp-radius)] border border-[var(--lp-border)] bg-white px-3 py-2"
        name={name}
        type={type}
        required={required}
        minLength={minLength}
        autoComplete={type === "password" ? "new-password" : "on"}
      />
    </label>
  );
}

export function SignupForm() {
  const [pending, startTransition] = useTransition();
  const [error, setError] = useState<string | null>(null);

  function onSubmit(event: SyntheticEvent<HTMLFormElement>) {
    event.preventDefault();
    setError(null);

    const form = new FormData(event.currentTarget);
    const email = formString(form, "email");
    const password = formString(form, "password");
    const displayName = formString(form, "displayName");
    const organizationName = formString(form, "organizationName");
    const organizationSlug = formString(form, "organizationSlug");

    startTransition(() => {
      void (async () => {
        try {
          const client = createLaunchPadClient({ baseUrl: apiBaseUrl });
          const result = await client.register({
            email,
            password,
            displayName,
            organizationName,
            organizationSlug: organizationSlug === "" ? undefined : organizationSlug,
            timezone: Intl.DateTimeFormat().resolvedOptions().timeZone,
          });

          const hash = new URLSearchParams({
            accessToken: result.tokens.accessToken,
            refreshToken: result.tokens.refreshToken,
          }).toString();
          window.location.assign(`${orgAdminUrl}/auth/callback#${hash}`);
        } catch (err) {
          if (err instanceof ApiError) {
            setError(err.message);
            return;
          }
          setError("Unable to create your account. Try again.");
        }
      })();
    });
  }

  return (
    <main className="min-h-screen py-24">
      <Container className="max-w-lg">
        <p className="text-sm font-semibold uppercase tracking-[0.2em] text-[var(--lp-accent)]">
          LaunchPad
        </p>
        <h1
          className="mt-4 text-4xl font-semibold tracking-tight"
          style={{ fontFamily: "var(--lp-font-display)" }}
        >
          Start your free trial
        </h1>
        <p className="mt-3 text-[var(--lp-ink-muted)]">
          Create your organization and jump into the admin portal.
        </p>

        <form onSubmit={onSubmit} className="mt-10 space-y-4">
          <Field label="Work email" name="email" type="email" required />
          <Field label="Your name" name="displayName" type="text" required />
          <Field
            label="Organization name"
            name="organizationName"
            type="text"
            required
          />
          <Field
            label="Organization slug (optional)"
            name="organizationSlug"
            type="text"
          />
          <Field
            label="Password"
            name="password"
            type="password"
            minLength={10}
            required
          />

          {error ? (
            <p className="text-sm text-[var(--lp-danger)]" role="alert">
              {error}
            </p>
          ) : null}

          <button
            type="submit"
            disabled={pending}
            className="w-full rounded-[var(--lp-radius)] bg-[var(--lp-accent)] px-6 py-3 text-sm font-semibold text-white transition hover:bg-[var(--lp-accent-hover)] disabled:opacity-60"
          >
            {pending ? "Creating account…" : "Create account"}
          </button>
        </form>

        <p className="mt-6 text-sm text-[var(--lp-ink-muted)]">
          Already registered?{" "}
          <Link href={`${orgAdminUrl}/login`} className="text-[var(--lp-accent)]">
            Sign in
          </Link>
        </p>
      </Container>
    </main>
  );
}
