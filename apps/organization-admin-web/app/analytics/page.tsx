"use client";

import { useEffect, useState, useTransition } from "react";
import { useRouter } from "next/navigation";
import type { OnboardingSummary } from "@launchpad/api-client";
import { ApiError } from "@launchpad/api-client";
import { MetricCard, PageHeader, Reveal, Surface } from "@launchpad/ui";
import { AdminShell } from "@/components/admin-shell";
import { getClient } from "@/lib/api";
import { clearSession, getAccessToken } from "@/lib/session";

export default function AnalyticsPage() {
  const router = useRouter();
  const [pending, startTransition] = useTransition();
  const [summary, setSummary] = useState<OnboardingSummary | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!getAccessToken()) {
      router.replace("/login");
      return;
    }

    startTransition(() => {
      void (async () => {
        try {
          setSummary(await getClient().getOnboardingAnalytics());
        } catch (err) {
          if (err instanceof ApiError && err.status === 401) {
            clearSession();
            router.replace("/login");
            return;
          }
          setError(err instanceof ApiError ? err.message : "Unable to load analytics");
        }
      })();
    });
  }, [router]);

  return (
    <AdminShell>
      <div className="space-y-8">
        <Reveal>
          <PageHeader
            eyebrow="Insights"
            title="Onboarding analytics"
            description="Completion rates, active journeys, and approval backlog for your organization."
          />
        </Reveal>

        {error ? (
          <p className="text-[var(--lp-danger)]" role="alert">
            {error}
          </p>
        ) : null}

        <Reveal delay={1}>
          <section className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
            <MetricCard
              label="Employees"
              value={summary?.employeeCount ?? (pending ? "…" : "—")}
            />
            <MetricCard
              label="Active assignments"
              value={summary?.activeAssignmentCount ?? (pending ? "…" : "—")}
            />
            <MetricCard
              label="Completed"
              value={summary?.completedAssignmentCount ?? (pending ? "…" : "—")}
            />
            <MetricCard
              label="Pending approvals"
              value={summary?.pendingApprovalCount ?? (pending ? "…" : "—")}
            />
          </section>
        </Reveal>

        <Reveal delay={2}>
          <Surface>
            <h2 className="text-lg font-semibold">Quality signals</h2>
            <dl className="mt-4 grid gap-4 sm:grid-cols-3">
              <div>
                <dt className="text-sm text-[var(--lp-ink-muted)]">Completion rate</dt>
                <dd className="text-2xl font-semibold">
                  {summary ? `${Math.round(summary.completionRate * 100)}%` : pending ? "…" : "—"}
                </dd>
              </div>
              <div>
                <dt className="text-sm text-[var(--lp-ink-muted)]">Avg. days to complete</dt>
                <dd className="text-2xl font-semibold">
                  {summary?.averageDaysToComplete ?? (pending ? "…" : "—")}
                </dd>
              </div>
              <div>
                <dt className="text-sm text-[var(--lp-ink-muted)]">Scheduled</dt>
                <dd className="text-2xl font-semibold">
                  {summary?.scheduledAssignmentCount ?? (pending ? "…" : "—")}
                </dd>
              </div>
            </dl>
            {summary ? (
              <p className="mt-4 text-xs text-[var(--lp-ink-muted)]">
                Generated {new Date(summary.generatedAt).toLocaleString()}
              </p>
            ) : null}
          </Surface>
        </Reveal>
      </div>
    </AdminShell>
  );
}
