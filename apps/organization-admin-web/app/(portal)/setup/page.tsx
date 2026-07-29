"use client";

import { useEffect, useState, useTransition, type SyntheticEvent } from "react";
import { useRouter } from "next/navigation";
import type { Organization } from "@launchpad/api-client";
import { ApiError } from "@launchpad/api-client";
import { PageHeader, Reveal, Surface } from "@launchpad/ui";
import { getClient } from "@/lib/api";
import { clearSession, getAccessToken } from "@/lib/session";

const steps = [
  { title: "Organization profile", detail: "Confirm your workspace name and timezone.", href: "/settings" },
  { title: "Brand and domain", detail: "Add your logo, colors, and customer-facing domain.", href: "/settings" },
  { title: "Departments and teams", detail: "Create the structure employees will join.", href: "/employees" },
  { title: "Job roles", detail: "Define the roles used by assignment rules.", href: "/employees" },
  { title: "Administrators", detail: "Invite owners and HR administrators.", href: "/members" },
  { title: "SSO and identity", detail: "Configure SSO, SCIM, and identity policies.", href: "/integrations" },
  { title: "Integrations", detail: "Connect HRIS, GitHub, Jira, chat, and calendars.", href: "/integrations" },
  { title: "Onboarding template", detail: "Choose a marketplace template or build a journey.", href: "/marketplace" },
  { title: "Notifications", detail: "Connect notification channels and delivery preferences.", href: "/settings" },
  { title: "Launch readiness", detail: "Review the workspace and finish setup.", href: "/dashboard" },
] as const;

export default function SetupPage() {
  const router = useRouter();
  const [pending, startTransition] = useTransition();
  const [organization, setOrganization] = useState<Organization | null>(null);
  const [selectedStep, setSelectedStep] = useState(1);
  const [error, setError] = useState<string | null>(null);
  const [message, setMessage] = useState<string | null>(null);

  useEffect(() => {
    if (!getAccessToken()) {
      router.replace("/login");
      return;
    }
    void getClient().getCurrentOrganization().then((org) => {
      setOrganization(org);
      setSelectedStep(Math.min(10, Math.max(1, (org.setupStep ?? 0) + (org.setupCompletedAt ? 0 : 1))));
    }).catch((err) => {
      if (err instanceof ApiError && err.status === 401) {
        clearSession();
        router.replace("/login");
        return;
      }
      setError(err instanceof ApiError ? err.message : "Unable to load setup");
    });
  }, [router]);

  function saveProfile(event: SyntheticEvent<HTMLFormElement>) {
    event.preventDefault();
    const form = new FormData(event.currentTarget);
    startTransition(() => {
      void (async () => {
        try {
          const updated = await getClient().updateCurrentOrganization({
            name: String(form.get("name") ?? "").trim(),
            timezone: String(form.get("timezone") ?? "").trim(),
          });
          setOrganization(updated);
          setMessage("Organization profile saved");
        } catch (err) {
          setError(err instanceof ApiError ? err.message : "Unable to save profile");
        }
      })();
    });
  }

  function advance() {
    const completed = selectedStep === 10;
    startTransition(() => {
      void (async () => {
        try {
          const updated = await getClient().updateOrganizationSetup(selectedStep, completed);
          setOrganization(updated);
          if (completed) {
            setMessage("Workspace setup complete");
          } else {
            setSelectedStep((step) => Math.min(10, step + 1));
            setMessage(`Step ${selectedStep} completed`);
          }
        } catch (err) {
          setError(err instanceof ApiError ? err.message : "Unable to update setup progress");
        }
      })();
    });
  }

  const current = steps[selectedStep - 1];
  const progress = organization?.setupCompletedAt ? 100 : ((organization?.setupStep ?? 0) / steps.length) * 100;

  return (
    <div className="space-y-6">
      <Reveal>
        <PageHeader
          eyebrow="Workspace launch"
          title="Organization setup"
          description="A guided path from workspace profile to launch readiness. Progress is saved for every administrator."
        />
      </Reveal>
      {error ? <p role="alert" className="text-sm text-[var(--lp-danger)]">{error}</p> : null}
      {message ? <p className="text-sm text-[var(--lp-success)]">{message}</p> : null}

      <Reveal delay={1}>
        <Surface>
          <div className="flex items-center justify-between text-sm">
            <span className="font-semibold">{organization?.setupCompletedAt ? "Setup complete" : `Step ${selectedStep} of ${steps.length}`}</span>
            <span className="text-[var(--lp-ink-muted)]">{Math.round(progress)}%</span>
          </div>
          <div className="mt-3 h-2 overflow-hidden rounded-full bg-[var(--lp-border)]">
            <div className="h-full bg-[var(--lp-accent)] transition-[width]" style={{ width: `${progress}%` }} />
          </div>
        </Surface>
      </Reveal>

      <div className="grid gap-5 lg:grid-cols-[18rem_minmax(0,1fr)]">
        <Reveal delay={2}>
          <Surface className="p-2">
            <ol>
              {steps.map((step, index) => {
                const number = index + 1;
                const done = number <= (organization?.setupStep ?? 0);
                return (
                  <li key={step.title}>
                    <button
                      type="button"
                      onClick={() => setSelectedStep(number)}
                      className={`flex w-full items-center gap-3 rounded-[var(--lp-radius)] px-3 py-2.5 text-left text-sm ${
                        selectedStep === number ? "bg-[var(--lp-brand-soft)] text-[var(--lp-brand)]" : ""
                      }`}
                    >
                      <span className="grid h-6 w-6 place-items-center rounded-full border border-current text-xs font-bold">
                        {done ? "✓" : number}
                      </span>
                      <span>{step.title}</span>
                    </button>
                  </li>
                );
              })}
            </ol>
          </Surface>
        </Reveal>

        <Reveal delay={3}>
          <Surface>
            <p className="text-xs font-semibold uppercase tracking-wider text-[var(--lp-accent)]">Step {selectedStep}</p>
            <h2 className="mt-2 text-2xl font-semibold">{current.title}</h2>
            <p className="mt-2 text-sm text-[var(--lp-ink-muted)]">{current.detail}</p>

            {selectedStep === 1 ? (
              <form onSubmit={saveProfile} className="mt-6 grid gap-4 sm:grid-cols-2">
                <label className="text-sm font-medium">
                  Organization name
                  <input className="lp-input mt-1.5" name="name" defaultValue={organization?.name} required />
                </label>
                <label className="text-sm font-medium">
                  Timezone
                  <input className="lp-input mt-1.5" name="timezone" defaultValue={organization?.timezone} required />
                </label>
                <button className="lp-btn lp-btn--secondary sm:col-span-2 sm:w-fit" disabled={pending}>
                  Save profile
                </button>
              </form>
            ) : (
              <button
                type="button"
                onClick={() => router.push(current.href)}
                className="lp-btn lp-btn--secondary mt-6"
              >
                Open {current.title.toLowerCase()}
              </button>
            )}

            <div className="mt-8 flex flex-wrap justify-between gap-3 border-t border-[var(--lp-border)] pt-5">
              <button
                type="button"
                onClick={() => setSelectedStep((step) => Math.max(1, step - 1))}
                disabled={pending || selectedStep === 1}
                className="lp-btn lp-btn--secondary"
              >
                Previous
              </button>
              <button type="button" onClick={advance} disabled={pending} className="lp-btn lp-btn--primary">
                {selectedStep === 10 ? "Complete setup" : "Mark complete & continue"}
              </button>
            </div>
          </Surface>
        </Reveal>
      </div>
    </div>
  );
}
