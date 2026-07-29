"use client";

import { useEffect, useState, useTransition, type FormEvent } from "react";
import type { CMSPage } from "@launchpad/api-client";
import { ApiError } from "@launchpad/api-client";
import { Surface } from "@launchpad/ui";
import { getClient } from "@/lib/api";

interface SiteInformation {
  salesEmail: string;
  supportEmail: string;
  securityEmail: string;
  privacyEmail: string;
  legalEmail: string;
  responseTime: string;
  securityEffectiveDate: string;
  termsEffectiveDate: string;
  privacyEffectiveDate: string;
  dpaEffectiveDate: string;
}

const defaults: SiteInformation = {
  salesEmail: "sales@launchpad.example",
  supportEmail: "support@launchpad.example",
  securityEmail: "security@launchpad.example",
  privacyEmail: "privacy@launchpad.example",
  legalEmail: "legal@launchpad.example",
  responseTime: "One business day",
  securityEffectiveDate: "July 28, 2026",
  termsEffectiveDate: "July 28, 2026",
  privacyEffectiveDate: "July 28, 2026",
  dpaEffectiveDate: "July 29, 2026",
};

const fields: { key: keyof SiteInformation; label: string; type?: string }[] = [
  { key: "salesEmail", label: "Sales email", type: "email" },
  { key: "supportEmail", label: "Support email", type: "email" },
  { key: "securityEmail", label: "Security email", type: "email" },
  { key: "privacyEmail", label: "Privacy email", type: "email" },
  { key: "legalEmail", label: "Legal email", type: "email" },
  { key: "responseTime", label: "Typical response time" },
  { key: "securityEffectiveDate", label: "Security page effective date" },
  { key: "termsEffectiveDate", label: "Terms effective date" },
  { key: "privacyEffectiveDate", label: "Privacy policy effective date" },
  { key: "dpaEffectiveDate", label: "DPA effective date" },
];

function parseInformation(page: CMSPage | undefined): SiteInformation {
  if (!page) return defaults;
  try {
    return { ...defaults, ...(JSON.parse(page.body) as Partial<SiteInformation>) };
  } catch {
    return defaults;
  }
}

export function SiteInformationEditor({
  pages,
  onSaved,
}: {
  pages: CMSPage[];
  onSaved: () => void;
}) {
  const [pending, startTransition] = useTransition();
  const [message, setMessage] = useState<string | null>(null);
  const settingsPage = pages.find((page) => page.slug === "site-information");
  const [information, setInformation] = useState<SiteInformation>(() =>
    parseInformation(settingsPage),
  );

  useEffect(() => {
    setInformation(parseInformation(settingsPage));
  }, [settingsPage]);

  function updateField(key: keyof SiteInformation, value: string) {
    setInformation((current) => ({ ...current, [key]: value }));
  }

  function onSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    startTransition(() => {
      void (async () => {
        try {
          const body = JSON.stringify(information);
          if (!settingsPage) {
            const created = await getClient().createPlatformCMSPage({
              slug: "site-information",
              title: "Public site information",
              summary: "Contact details and effective dates used by public trust pages.",
              body,
              contentType: "settings",
            });
            await getClient().publishPlatformCMSPage(created.id);
          } else {
            const updated = await getClient().updatePlatformCMSPage(settingsPage.id, { body });
            if (updated.status === "draft") {
              await getClient().publishPlatformCMSPage(updated.id);
            }
          }
          setMessage("Site information saved and published.");
          onSaved();
        } catch (error) {
          setMessage(error instanceof ApiError ? error.message : "Unable to save site information");
        }
      })();
    });
  }

  return (
    <Surface>
      <div className="flex flex-wrap items-start justify-between gap-4">
        <div>
          <p className="text-xs font-semibold uppercase tracking-[0.14em] text-[var(--lp-accent)]">
            Public trust details
          </p>
          <h2 className="mt-2 text-xl font-semibold">Site information</h2>
          <p className="mt-1 max-w-2xl text-sm leading-6 text-[var(--lp-ink-muted)]">
            These values appear on Contact, Security, Terms, Privacy, and DPA pages. Saving updates
            the published site information immediately.
          </p>
        </div>
        <span className="rounded-full bg-[var(--lp-brand-soft)] px-3 py-1 text-xs font-semibold text-[var(--lp-accent)]">
          {settingsPage?.status === "published" ? "Published" : "Ready to publish"}
        </span>
      </div>

      <form className="mt-6 grid gap-4 md:grid-cols-2" onSubmit={onSubmit}>
        {fields.map((field) => (
          <label key={field.key} className="grid gap-1.5 text-sm font-medium">
            {field.label}
            <input
              className="lp-input"
              type={field.type ?? "text"}
              value={information[field.key]}
              onChange={(event) => updateField(field.key, event.target.value)}
              required
            />
          </label>
        ))}
        <div className="flex flex-wrap items-center gap-3 md:col-span-2">
          <button
            type="submit"
            disabled={pending}
            className="lp-btn lp-btn--primary disabled:opacity-60"
          >
            {pending ? "Publishing…" : "Save and publish"}
          </button>
          {message ? (
            <p className="text-sm text-[var(--lp-ink-muted)]" role="status">
              {message}
            </p>
          ) : null}
        </div>
      </form>
    </Surface>
  );
}
