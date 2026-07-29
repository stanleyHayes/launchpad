"use client";

import { useState, useTransition, type SyntheticEvent } from "react";
import { createLaunchPadClient } from "@launchpad/api-client";
import { apiBaseUrl } from "./env";

export function NewsletterForm() {
  const [pending, startTransition] = useTransition();
  const [message, setMessage] = useState("");

  function submit(event: SyntheticEvent<HTMLFormElement>) {
    event.preventDefault();
    const form = event.currentTarget;
    const email = String(new FormData(form).get("email") ?? "").trim();
    startTransition(() => {
      void createLaunchPadClient({ baseUrl: apiBaseUrl }).createLead({
        name: "Newsletter subscriber",
        email,
        source: "marketing_newsletter",
      }).then(() => {
        form.reset();
        setMessage("You’re on the list.");
      }).catch(() => setMessage("We couldn’t subscribe you. Please try again."));
    });
  }

  return (
    <form onSubmit={submit} className="mt-5" aria-label="Newsletter signup">
      <label className="lp-eyebrow" htmlFor="newsletter-email">Onboarding field notes</label>
      <div className="mt-3 flex gap-2">
        <input
          id="newsletter-email"
          className="lp-input min-w-0"
          name="email"
          type="email"
          required
          placeholder="you@company.com"
          aria-describedby="newsletter-message"
        />
        <button className="lp-btn lp-btn--primary" disabled={pending} type="submit">
          {pending ? "Joining…" : "Join"}
        </button>
      </div>
      <p id="newsletter-message" className="mt-2 text-xs text-[var(--lp-ink-muted)]" aria-live="polite">
        {message || "One practical note a month. Unsubscribe anytime."}
      </p>
    </form>
  );
}
