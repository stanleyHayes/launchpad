"use client";

import Link from "next/link";
import { useState, useTransition, type SyntheticEvent } from "react";
import { ApiError, createLaunchPadClient } from "@launchpad/api-client";
import { Container, LogoTile } from "@launchpad/ui";
import { SiteFooter } from "../site-footer";
import { SiteHeader } from "../site-header";
import { apiBaseUrl, orgAdminUrl } from "../env";
import { FormField } from "../form-field";

function formString(form: FormData, key: string): string {
  const value = form.get(key);
  return typeof value === "string" ? value.trim() : "";
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
    <main className="relative min-h-screen">
      <SiteHeader variant="light" />
      <section className="pb-16 pt-36">
      <Container className="max-w-lg">
        <LogoTile size={40} />
        <p className="lp-eyebrow mt-5">
          Start free
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

        <form onSubmit={onSubmit} className="mt-10 space-y-5">
          <FormField label="Work email" name="email" type="email" required startIcon="mail" autoComplete="email" placeholder="you@company.com" />
          <FormField label="Your name" name="displayName" type="text" required startIcon="user" autoComplete="name" placeholder="Priya Shah" />
          <FormField
            label="Organization name"
            name="organizationName"
            type="text"
            required
            startIcon="building"
            autoComplete="organization"
            placeholder="Northwind Labs"
          />
          <FormField
            label="Organization slug (optional)"
            name="organizationSlug"
            type="text"
            startIcon="at-sign"
            placeholder="northwind"
          />
          <FormField
            label="Password"
            name="password"
            type="password"
            minLength={10}
            required
            startIcon="lock"
            placeholder="At least 10 characters"
          />

          {error ? (
            <p className="text-sm text-[var(--lp-danger)]" role="alert">
              {error}
            </p>
          ) : null}

          <button
            type="submit"
            disabled={pending}
            className="lp-btn lp-btn--primary w-full"
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
      </section>

      <SiteFooter />
    </main>
  );
}
