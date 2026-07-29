"use client";

import { useCallback, useEffect, useState, useTransition } from "react";
import { useRouter } from "next/navigation";
import type {
  AssistantReport,
  FunnelReport,
  OnboardingBreakdown,
  OnboardingBreakdownGroupBy,
  OnboardingSummary,
} from "@launchpad/api-client";
import { ApiError } from "@launchpad/api-client";
import { EmptyState, MetricCard, PageHeader, Reveal, Surface } from "@launchpad/ui";
import { getClient } from "@/lib/api";
import { clearSession, getAccessToken } from "@/lib/session";

const breakdownTabs: { key: OnboardingBreakdownGroupBy; label: string }[] = [
  { key: "department", label: "Department" },
  { key: "jobRole", label: "Job role" },
];

export default function AnalyticsPage() {
  const router = useRouter();
  const [pending, startTransition] = useTransition();
  const [summary, setSummary] = useState<OnboardingSummary | null>(null);
  const [report, setReport] = useState<AssistantReport | null>(null);
  const [breakdown, setBreakdown] = useState<OnboardingBreakdown | null>(null);
  const [funnel, setFunnel] = useState<FunnelReport | null>(null);
  const [breakdownBy, setBreakdownBy] = useState<OnboardingBreakdownGroupBy>("department");
  const [error, setError] = useState<string | null>(null);

  const handleLoadError = useCallback(
    (err: unknown) => {
      if (err instanceof ApiError && err.status === 401) {
        clearSession();
        router.replace("/login");
        return;
      }
      setError(err instanceof ApiError ? err.message : "Unable to load analytics");
    },
    [router],
  );

  useEffect(() => {
    if (!getAccessToken()) {
      router.replace("/login");
      return;
    }

    let stale = false;
    startTransition(() => {
      void (async () => {
        try {
          const [summaryItem, reportItem, funnelItem] = await Promise.all([
            getClient().getOnboardingAnalytics(),
            getClient().getAssistantReport(),
            getClient().getOnboardingFunnel(),
          ]);
          if (stale) return;
          setSummary(summaryItem);
          setReport(reportItem);
          setFunnel(funnelItem);
        } catch (err) {
          if (stale) return;
          handleLoadError(err);
        }
      })();
    });
    return () => {
      stale = true;
    };
  }, [router, handleLoadError]);

  useEffect(() => {
    if (!getAccessToken()) {
      return;
    }

    let stale = false;
    startTransition(() => {
      void (async () => {
        try {
          const breakdownItem = await getClient().getOnboardingBreakdown(breakdownBy);
          if (stale) return;
          setBreakdown(breakdownItem);
        } catch (err) {
          if (stale) return;
          handleLoadError(err);
        }
      })();
    });
    return () => {
      stale = true;
    };
  }, [breakdownBy, handleLoadError]);

  return (
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
          <section className="grid gap-4 sm:grid-cols-2 xl:grid-cols-5">
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
            <MetricCard
              label="Overdue rate"
              value={
                summary ? `${Math.round(summary.overdueRate * 100)}%` : pending ? "…" : "—"
              }
              hint={summary ? `${summary.overdueStepCount} overdue steps` : undefined}
            />
          </section>
        </Reveal>

        <Reveal delay={2}>
          <Surface>
            <h2 className="text-lg font-semibold">Quality signals</h2>
            <dl className="mt-4 grid gap-4 sm:grid-cols-4">
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
              <div>
                <dt className="text-sm text-[var(--lp-ink-muted)]">Incomplete steps</dt>
                <dd className="text-2xl font-semibold">
                  {summary?.incompleteStepCount ?? (pending ? "…" : "—")}
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

        <Reveal delay={3}>
          <Surface>
            <h2 className="text-lg font-semibold">Journey milestones and drop-off</h2>
            <p className="mt-1 text-sm text-[var(--lp-ink-muted)]">Completion at each step and the first open step where active journeys currently stop.</p>
            {!funnel?.milestones.length ? <p className="mt-4 text-sm text-[var(--lp-ink-muted)]">No funnel data yet.</p> : (
              <div className="mt-4 grid gap-5 lg:grid-cols-2">
                <ol className="space-y-3">
                  {funnel.milestones.map((milestone) => (
                    <li key={milestone.position}>
                      <div className="flex justify-between text-sm"><span>{milestone.position}. {milestone.stepTitle}</span><strong>{Math.round(milestone.rate * 100)}%</strong></div>
                      <div className="mt-1 h-2 overflow-hidden rounded-full bg-[var(--lp-border)]"><div className="h-full bg-[var(--lp-success)]" style={{ width: `${milestone.rate * 100}%` }} /></div>
                    </li>
                  ))}
                </ol>
                <div>
                  <h3 className="text-sm font-semibold">Top drop-off points</h3>
                  <ul className="mt-2 space-y-2">
                    {funnel.dropOff.slice(0, 8).map((item) => <li key={item.position} className="flex justify-between text-sm"><span>{item.stepTitle}</span><strong>{item.count}</strong></li>)}
                  </ul>
                </div>
              </div>
            )}
          </Surface>
        </Reveal>

        <Reveal delay={3}>
          <Surface className="overflow-hidden p-0">
            <div className="flex flex-wrap items-center justify-between gap-3 border-b border-[var(--lp-border)] px-5 py-4">
              <div>
                <h2 className="text-lg font-semibold">Completion breakdown</h2>
                <p className="text-sm text-[var(--lp-ink-muted)]">
                  Rates for employees currently on your roster.
                </p>
              </div>
              <div className="flex gap-2" role="tablist" aria-label="Group by">
                {breakdownTabs.map((tab) => (
                  <button
                    key={tab.key}
                    type="button"
                    role="tab"
                    aria-selected={breakdownBy === tab.key}
                    onClick={() => setBreakdownBy(tab.key)}
                    className={
                      breakdownBy === tab.key
                        ? "rounded-full bg-[var(--lp-accent)]/10 px-3 py-1 text-xs font-semibold text-[var(--lp-accent)]"
                        : "rounded-full px-3 py-1 text-xs font-semibold text-[var(--lp-ink-muted)]"
                    }
                  >
                    {tab.label}
                  </button>
                ))}
              </div>
            </div>
            {!breakdown || breakdown.rows.length === 0 ? (
              <div className="p-5">
                <EmptyState
                  dense
                  title="No breakdown data"
                  description="Employees with department or job-role assignments will appear here."
                />
              </div>
            ) : (
              <table className="w-full text-sm">
                <thead>
                  <tr className="border-b border-[var(--lp-border)] text-left text-[var(--lp-ink-muted)]">
                    <th className="px-5 py-3 font-medium">Name</th>
                    <th className="px-5 py-3 font-medium">Employees</th>
                    <th className="px-5 py-3 font-medium">Assignments</th>
                    <th className="px-5 py-3 font-medium">Completed</th>
                    <th className="px-5 py-3 font-medium">Completion rate</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-[var(--lp-border)]">
                  {breakdown.rows.map((row) => (
                    <tr key={row.id || "unassigned"}>
                      <td className="px-5 py-3 font-medium">{row.name}</td>
                      <td className="px-5 py-3">{row.employeeCount}</td>
                      <td className="px-5 py-3">{row.assignmentCount}</td>
                      <td className="px-5 py-3">{row.completedAssignmentCount}</td>
                      <td className="px-5 py-3 font-semibold">
                        {Math.round(row.completionRate * 100)}%
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            )}
          </Surface>
        </Reveal>

        <Reveal delay={2}>
          <Surface className="overflow-hidden p-0">
            <div className="border-b border-[var(--lp-border)] px-5 py-4">
              <h2 className="text-lg font-semibold">Assistant report</h2>
              <p className="text-sm text-[var(--lp-ink-muted)]">
                Question volume, refusals, and feedback for the AI assistant.
              </p>
            </div>
            <dl className="grid gap-4 px-5 py-4 sm:grid-cols-3">
              <div>
                <dt className="text-sm text-[var(--lp-ink-muted)]">Total questions</dt>
                <dd className="text-2xl font-semibold">
                  {report?.totalQuestions ?? (pending ? "…" : "—")}
                </dd>
              </div>
              <div>
                <dt className="text-sm text-[var(--lp-ink-muted)]">Refusal rate</dt>
                <dd className="text-2xl font-semibold">
                  {report ? `${Math.round(report.refusalRate * 100)}%` : pending ? "…" : "—"}
                </dd>
              </div>
              <div>
                <dt className="text-sm text-[var(--lp-ink-muted)]">Helpful rate</dt>
                <dd className="text-2xl font-semibold">
                  {report ? `${Math.round(report.helpfulRate * 100)}%` : pending ? "…" : "—"}
                </dd>
              </div>
            </dl>
            <div className="border-t border-[var(--lp-border)] px-5 py-4">
              <h3 className="text-sm font-semibold">Top refused questions</h3>
              {!report || report.topRefusedQuestions.length === 0 ? (
                <p className="mt-2 text-sm text-[var(--lp-ink-muted)]">
                  No refused questions yet.
                </p>
              ) : (
                <ul className="mt-2 space-y-2">
                  {report.topRefusedQuestions.map((stat) => (
                    <li
                      key={stat.question}
                      className="flex items-center justify-between gap-3 text-sm"
                    >
                      <span className="truncate">{stat.question}</span>
                      <span className="rounded-full bg-[var(--lp-accent)]/10 px-3 py-1 text-xs font-semibold text-[var(--lp-accent)]">
                        {stat.count}×
                      </span>
                    </li>
                  ))}
                </ul>
              )}
            </div>
          </Surface>
        </Reveal>
      </div>
      );
}
