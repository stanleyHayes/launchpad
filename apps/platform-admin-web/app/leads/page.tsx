"use client";

import { useEffect, useState, useTransition } from "react";
import { useRouter } from "next/navigation";
import type { Lead } from "@launchpad/api-client";
import { ApiError } from "@launchpad/api-client";
import { EmptyState, PageHeader, Reveal, Surface } from "@launchpad/ui";
import { PlatformShell } from "@/components/platform-shell";
import { getClient } from "@/lib/api";
import { clearSession, getAccessToken } from "@/lib/session";

export default function LeadsPage() {
  const router = useRouter();
  const [pending, startTransition] = useTransition();
  const [leads, setLeads] = useState<Lead[]>([]);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!getAccessToken()) {
      router.replace("/login");
      return;
    }

    startTransition(() => {
      void (async () => {
        try {
          setLeads(await getClient().listPlatformLeads());
        } catch (err) {
          if (err instanceof ApiError && err.status === 401) {
            clearSession();
            router.replace("/login");
            return;
          }
          setError(err instanceof ApiError ? err.message : "Unable to load leads");
        }
      })();
    });
  }, [router]);

  return (
    <PlatformShell>
      <div className="space-y-8">
        <Reveal>
          <PageHeader
            eyebrow="Operations"
            title="Leads"
            description="Inbound demo requests and marketing interest captured from the public site."
          />
        </Reveal>

        {error ? (
          <p className="text-[var(--lp-danger)]" role="alert">
            {error}
          </p>
        ) : null}

        <Reveal delay={1}>
          <Surface className="overflow-hidden p-0">
            <div className="border-b border-[var(--lp-border)] px-5 py-4">
              <h2 className="text-lg font-semibold">All leads</h2>
              <p className="text-sm text-[var(--lp-ink-muted)]">
                {pending && leads.length === 0 ? "Loading…" : `${leads.length} leads`}
              </p>
            </div>
            {leads.length === 0 ? (
              <div className="p-5">
                <EmptyState
                  dense
                  title="No leads yet"
                  description="Demo and contact submissions will appear here."
                />
              </div>
            ) : (
              <ul className="divide-y divide-[var(--lp-border)]">
                {leads.map((lead) => (
                  <li key={lead.id} className="px-5 py-4">
                    <div className="flex flex-wrap items-start justify-between gap-3">
                      <div>
                        <p className="font-medium">{lead.name}</p>
                        <p className="text-sm text-[var(--lp-ink-muted)]">
                          {lead.email}
                          {lead.company ? ` · ${lead.company}` : ""}
                        </p>
                      </div>
                      <div className="text-right text-sm text-[var(--lp-ink-muted)]">
                        <p>{lead.status}</p>
                        <time>{new Date(lead.createdAt).toLocaleString()}</time>
                      </div>
                    </div>
                    {lead.message ? (
                      <p className="mt-2 text-sm text-[var(--lp-ink-muted)]">{lead.message}</p>
                    ) : null}
                    <p className="mt-1 text-xs uppercase tracking-wide text-[var(--lp-ink-muted)]">
                      Source: {lead.source}
                    </p>
                  </li>
                ))}
              </ul>
            )}
          </Surface>
        </Reveal>
      </div>
    </PlatformShell>
  );
}
