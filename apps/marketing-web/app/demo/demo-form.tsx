"use client";

import { useState, useTransition, type SyntheticEvent } from "react";
import { ApiError, createLaunchPadClient } from "@launchpad/api-client";
import { Container } from "@launchpad/ui";
import { SiteFooter } from "../site-footer";
import { SiteHeader } from "../site-header";
import { apiBaseUrl } from "../env";
import { FormField } from "../form-field";
import { FormTextArea } from "../form-textarea";
import { Icon } from "../ui-icon";

function formString(form: FormData, key: string): string {
  const value = form.get(key);
  return typeof value === "string" ? value.trim() : "";
}

const emailPattern = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;

interface FieldErrors {
  name?: string;
  email?: string;
  consent?: string;
}

function validate(form: FormData): FieldErrors {
  const errors: FieldErrors = {};
  if (!formString(form, "name")) {
    errors.name = "Please tell us your name.";
  }
  const email = formString(form, "email");
  if (!email) {
    errors.email = "Please enter your work email.";
  } else if (!emailPattern.test(email)) {
    errors.email = "That email address doesn't look right.";
  }
  if (form.get("consent") === null) {
    errors.consent = "Please confirm we can contact you about your request.";
  }
  return errors;
}

const successMessage = (name: string) =>
  `Thanks${name ? `, ${name}` : ""}! We received your request and will be in touch soon.`;

export function DemoForm() {
  const [pending, startTransition] = useTransition();
  const [error, setError] = useState<string | null>(null);
  const [fieldErrors, setFieldErrors] = useState<FieldErrors>({});
  const [success, setSuccess] = useState<string | null>(null);

  function onSubmit(event: SyntheticEvent<HTMLFormElement>) {
    event.preventDefault();
    setError(null);
    setSuccess(null);

    const formEl = event.currentTarget;
    const form = new FormData(formEl);

    // Honeypot: real users never see or fill this field, so a value means a
    // bot. Pretend success without touching the API.
    if (formString(form, "website")) {
      formEl.reset();
      setSuccess(successMessage(""));
      return;
    }

    const errors = validate(form);
    setFieldErrors(errors);
    if (Object.keys(errors).length > 0) {
      return;
    }

    startTransition(() => {
      void (async () => {
        try {
          const client = createLaunchPadClient({ baseUrl: apiBaseUrl });
          let campaign: { utmSource?: string; utmMedium?: string; utmCampaign?: string } = {};
          try {
            campaign = JSON.parse(window.sessionStorage.getItem("launchpad_campaign") ?? "{}");
          } catch {
            campaign = {};
          }
          const preferredTime = formString(form, "scheduledFor");
          const lead = await client.createLead({
            name: formString(form, "name"),
            email: formString(form, "email"),
            company: formString(form, "company") || undefined,
            message: formString(form, "message") || undefined,
            source: "marketing_demo",
            scheduledFor: preferredTime ? new Date(preferredTime).toISOString() : undefined,
            ...campaign,
          });
          formEl.reset();
          setSuccess(successMessage(lead.name));
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
    <main className="relative min-h-screen">
      <SiteHeader variant="light" />

      <section className="pb-16 pt-36">
        <Container className="max-w-lg">
          <p className="lp-eyebrow">
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

          {success ? (
            <div className="lp-card mt-10 p-8 text-center">
              <span className="mx-auto grid h-14 w-14 place-items-center rounded-full bg-[var(--lp-success)]/12 text-[var(--lp-success)]">
                <Icon name="check" className="h-7 w-7" />
              </span>
              <h2
                className="mt-5 text-2xl font-semibold tracking-tight text-[var(--lp-ink)]"
                style={{ fontFamily: "var(--lp-font-display)" }}
              >
                Request received
              </h2>
              <p className="mt-3 leading-7 text-[var(--lp-ink-muted)]">{success}</p>
            </div>
          ) : (
            <form onSubmit={onSubmit} noValidate className="mt-10 space-y-5">
              {/* Honeypot: visually hidden from humans, irresistible to bots. */}
              <div
                aria-hidden="true"
                className="pointer-events-none absolute -left-[9999px] top-auto h-px w-px overflow-hidden"
              >
                <label>
                  Website
                  <input type="text" name="website" tabIndex={-1} autoComplete="off" />
                </label>
              </div>

              <div>
                <FormField label="Your name" name="name" required startIcon="user" autoComplete="name" placeholder="Priya Shah" />
                {fieldErrors.name ? (
                  <p className="mt-1.5 text-sm text-[var(--lp-danger)]" role="alert">
                    {fieldErrors.name}
                  </p>
                ) : null}
              </div>
              <div>
                <FormField label="Work email" name="email" type="email" required startIcon="mail" autoComplete="email" placeholder="you@company.com" />
                {fieldErrors.email ? (
                  <p className="mt-1.5 text-sm text-[var(--lp-danger)]" role="alert">
                    {fieldErrors.email}
                  </p>
                ) : null}
              </div>
              <FormField label="Company" name="company" startIcon="building" autoComplete="organization" placeholder="Northwind Labs" />
              <FormField
                label="Preferred demo time"
                name="scheduledFor"
                type="datetime-local"
                required
              />
              <FormTextArea
                label="What would you like to explore?"
                name="message"
                startIcon="message"
                placeholder="Journeys, approvals, the AI assistant…"
              />

              <div>
                <label className="flex items-start gap-3 text-sm leading-6 text-[var(--lp-ink-muted)]">
                  <input
                    type="checkbox"
                    name="consent"
                    required
                    className="mt-1 h-4 w-4 shrink-0 accent-[var(--lp-brand)]"
                  />
                  <span>
                    I agree to be contacted about my demo request and understand my
                    details will be used to follow up.
                  </span>
                </label>
                {fieldErrors.consent ? (
                  <p className="mt-1.5 text-sm text-[var(--lp-danger)]" role="alert">
                    {fieldErrors.consent}
                  </p>
                ) : null}
              </div>

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
                {pending ? "Submitting…" : "Request demo"}
              </button>
            </form>
          )}
        </Container>
      </section>

      <SiteFooter />
    </main>
  );
}
