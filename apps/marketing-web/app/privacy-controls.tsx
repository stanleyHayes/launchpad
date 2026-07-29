"use client";

import { useEffect, useState } from "react";
import { useSearchParams } from "next/navigation";

const consentKey = "launchpad_cookie_consent";
const campaignKey = "launchpad_campaign";

export function PrivacyControls() {
  const params = useSearchParams();
  const [consent, setConsent] = useState<string | null>(null);
  const [chatOpen, setChatOpen] = useState(false);

  useEffect(() => {
    setConsent(window.localStorage.getItem(consentKey));
    const campaign = {
      utmSource: params.get("utm_source") ?? "",
      utmMedium: params.get("utm_medium") ?? "",
      utmCampaign: params.get("utm_campaign") ?? "",
    };
    if (Object.values(campaign).some(Boolean)) {
      window.sessionStorage.setItem(campaignKey, JSON.stringify(campaign));
    }
  }, [params]);

  function choose(value: "essential" | "analytics") {
    window.localStorage.setItem(consentKey, value);
    setConsent(value);
    window.dispatchEvent(new CustomEvent("launchpad:consent", { detail: value }));
  }

  return (
    <>
      {!consent ? (
        <aside
          className="fixed inset-x-4 bottom-4 z-50 mx-auto max-w-3xl rounded-2xl border border-[var(--lp-border)] bg-[var(--lp-paper-elevated)] p-5 shadow-2xl"
          aria-label="Cookie preferences"
        >
          <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
            <p className="text-sm leading-6 text-[var(--lp-ink-muted)]">
              We use essential storage to run this site. Optional analytics helps us understand which
              onboarding resources are useful. No advertising cookies.
            </p>
            <div className="flex shrink-0 gap-2">
              <button className="lp-btn lp-btn--ghost" type="button" onClick={() => choose("essential")}>
                Essential only
              </button>
              <button className="lp-btn lp-btn--primary" type="button" onClick={() => choose("analytics")}>
                Allow analytics
              </button>
            </div>
          </div>
        </aside>
      ) : null}

      <div className="fixed bottom-5 right-5 z-40">
        {chatOpen ? (
          <div className="mb-3 w-80 rounded-2xl border border-[var(--lp-border)] bg-[var(--lp-paper-elevated)] p-5 shadow-2xl">
            <p className="font-semibold text-[var(--lp-ink)]">Talk to LaunchPad</p>
            <p className="mt-2 text-sm leading-6 text-[var(--lp-ink-muted)]">
              Our onboarding specialists reply during Ghana business hours.
            </p>
            <a className="lp-btn lp-btn--primary mt-4 w-full" href="/contact">
              Start a conversation
            </a>
          </div>
        ) : null}
        <button
          type="button"
          className="lp-btn lp-btn--primary ml-auto shadow-xl"
          aria-expanded={chatOpen}
          onClick={() => setChatOpen((open) => !open)}
        >
          {chatOpen ? "Close" : "Chat with us"}
        </button>
      </div>
    </>
  );
}
