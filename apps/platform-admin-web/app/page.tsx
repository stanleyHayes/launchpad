"use client";

import Link from "next/link";
import { useEffect, useState, useTransition } from "react";
import { useRouter } from "next/navigation";
import type { MeResponse, PlatformOverview } from "@launchpad/api-client";
import { ApiError } from "@launchpad/api-client";
import {
  MetricCard,
  PageHeader,
  Reveal,
  Surface,
} from "@launchpad/ui";
import { PlatformShell } from "@/components/platform-shell";
import { getClient } from "@/lib/api";
import { clearSession, getAccessToken } from "@/lib/session";

export default function DashboardPage() {
  const router = useRouter();
  const [pending, startTransition] = useTransition();
  const [me, setMe] = useState<MeResponse | null>(null);
  const [overview, setOverview] = useState<PlatformOverview | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!getAccessToken()) {
      router.replace("/login");
      return;
    }

    startTransition(() => {
      void (async () => {
        try {
          const client = getClient();
          const [profile, metrics] = await Promise.all([
            client.me(),
            client.platformOverview(),
          ]);
          setMe(profile);
          setOverview(metrics);
        } catch (err) {
          if (err instanceof ApiError && err.status === 401) {
            clearSession();
            router.replace("/login");
            return;
          }
          setError(err instanceof ApiError ? err.message : "Unable to load overview");
        }
      })();
    });
  }, [router]);

  return (
    <PlatformShell>
      <div className="space-y-8">
        <Reveal>
          <Surface className="overflow-hidden">
            <PageHeader
              eyebrow="Platform control plane"
              title={me ? `Welcome, ${me.user.displayName}` : "Platform overview"}
              description="Monitor tenant health, inbound leads, and organization lifecycle from one workspace."
            />
          </Surface>
        </Reveal>

        {error ? (
          <p className="text-[var(--lp-danger)]" role="alert">
            {error}
          </p>
        ) : null}

        <Reveal delay={1}>
          <section className="grid gap-4 sm:grid-cols-2 xl:grid-cols-6">
            <MetricCard
              label="Total organizations"
              value={overview?.totalOrgs ?? (pending ? "…" : "—")}
            />
            <MetricCard
              label="Trial organizations"
              value={overview?.trialOrgs ?? (pending ? "…" : "—")}
            />
            <MetricCard
              label="Active organizations"
              value={overview?.activeOrgs ?? (pending ? "…" : "—")}
            />
            <MetricCard
              label="Suspended organizations"
              value={overview?.suspendedOrgs ?? (pending ? "…" : "—")}
            />
            <MetricCard
              label="Inbound leads"
              value={overview?.totalLeads ?? (pending ? "…" : "—")}
            />
            <MetricCard
              label="Open support tickets"
              value={overview?.openTicketCount ?? (pending ? "…" : "—")}
              hint="Needs platform attention"
            />
          </section>
        </Reveal>

        <Reveal delay={2}>
          <Surface>
            <h2
              className="text-xl font-semibold"
              style={{ fontFamily: "var(--lp-font-display)" }}
            >
              Quick links
            </h2>
            <ul className="mt-4 divide-y divide-[var(--lp-border)]">
              {[
                {
                  href: "/organizations",
                  label: "Organizations",
                  detail: "Review tenants, suspend, or reactivate",
                },
                {
                  href: "/leads",
                  label: "Leads",
                  detail: "Inbound demo and trial interest",
                },
                {
                  href: "/support",
                  label: "Support",
                  detail: "Customer tickets and workflow",
                },
              ].map((link) => (
                <li key={link.href}>
                  <Link
                    href={link.href}
                    className="flex items-center justify-between gap-3 py-3 transition hover:text-[var(--lp-accent)]"
                  >
                    <span>
                      <span className="block font-medium">{link.label}</span>
                      <span className="text-sm text-[var(--lp-ink-muted)]">{link.detail}</span>
                    </span>
                    <span aria-hidden="true">→</span>
                  </Link>
                </li>
              ))}
            </ul>
          </Surface>
        </Reveal>
      </div>
    </PlatformShell>
  );
}
