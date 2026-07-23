"use client";

import Link from "next/link";
import { useState, useTransition, type SyntheticEvent } from "react";
import { ApiError, createLaunchPadClient } from "@launchpad/api-client";
import { Container } from "@launchpad/ui";

const apiBaseUrl =
  process.env.NEXT_PUBLIC_API_BASE_URL ?? "http://localhost:8080";

function formString(form: FormData, key: string): string {
  const value = form.get(key);
  return typeof value === "string" ? value.trim() : "";
}

function Field({
  label,
  name,
  type = "text",
  required,
}: {
  label: string;
  name: string;
  type?: string;
  required?: boolean;
}) {
  return (
    <label className="block text-sm text-[var(--lp-ink)]">
      {label}
      <input
        className="mt-1 w-full rounded-[var(--lp-radius)] border border-[var(--lp-border)] bg-white px-3 py-2"
        name={name}
        type={type}
        required={required}
      />
    </label>
  );
}

export default function DemoPage() {
  const [pending, startTransition] = useTransition();
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState<string | null>(null);

  function onSubmit(event: SyntheticEvent<HTMLFormElement>) {
    event.preventDefault();
    setError(null);
    setSuccess(null);

    const form = new FormData(event.currentTarget);

    startTransition(() => {
      void (async () => {
        try {
          const client = createLaunchPadClient({ baseUrl: apiBaseUrl });
          const lead = await client.createLead({
            name: formString(form, "name"),
            email: formString(form, "email"),
            company: formString(form, "company") || undefined,
            message: formString(form, "message") || undefined,
            source: "marketing_demo",
          });
          event.currentTarget.reset();
          setSuccess(`Thanks, ${lead.name}! We received your request and will be in touch soon.`);
        } catch (err) {
          if (err instanceof ApiError) {
            setError(err.message);
            return;
          }
          setError("Unable to submit your request. Try again.");
        }
      })();
    });
  }

  return (
    <main className="min-h-screen">
      <header className="border-b border-[var(--lp-border)] bg-white">
        <Container className="flex items-center justify-between py-5">
          <Link href="/" className="text-lg font-semibold tracking-tight text-[var(--lp-ink)]">
            LaunchPad
          </Link>
          <nav className="flex items-center gap-5 text-sm">
            <Link href="/" className="text-[var(--lp-ink-muted)] hover:text-[var(--lp-ink)]">
              Home
            </Link>
            <Link
              href="/signup"
              className="rounded-[var(--lp-radius)] bg-[var(--lp-accent)] px-4 py-2 font-semibold text-white"
            >
              Start free trial
            </Link>
          </nav>
        </Container>
      </header>

      <section className="py-16">
        <Container className="max-w-lg">
          <p className="text-sm font-semibold uppercase tracking-[0.2em] text-[var(--lp-accent)]">
            Book a demo
          </p>
          <h1
            className="mt-4 text-4xl font-semibold tracking-tight"
            style={{ fontFamily: "var(--lp-font-display)" }}
          >
            See LaunchPad in action
          </h1>
          <p className="mt-3 text-[var(--lp-ink-muted)]">
            Tell us about your team and we will follow up with a tailored walkthrough.
          </p>

          <form onSubmit={onSubmit} className="mt-10 space-y-4">
            <Field label="Your name" name="name" required />
            <Field label="Work email" name="email" type="email" required />
            <Field label="Company" name="company" />
            <label className="block text-sm text-[var(--lp-ink)]">
              What would you like to explore?
              <textarea
                className="mt-1 min-h-28 w-full rounded-[var(--lp-radius)] border border-[var(--lp-border)] bg-white px-3 py-2"
                name="message"
              />
            </label>

            {error ? (
              <p className="text-sm text-[var(--lp-danger)]" role="alert">
                {error}
              </p>
            ) : null}
            {success ? (
              <p className="rounded-[var(--lp-radius)] bg-[var(--lp-success)]/10 px-3 py-2 text-sm text-[var(--lp-success)]">
                {success}
              </p>
            ) : null}

            <button
              type="submit"
              disabled={pending}
              className="w-full rounded-[var(--lp-radius)] bg-[var(--lp-accent)] px-6 py-3 text-sm font-semibold text-white transition hover:bg-[var(--lp-accent-hover)] disabled:opacity-60"
            >
              {pending ? "Submitting…" : "Request demo"}
            </button>
          </form>
        </Container>
      </section>
    </main>
  );
}
