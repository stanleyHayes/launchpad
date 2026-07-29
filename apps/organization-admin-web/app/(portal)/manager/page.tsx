"use client";

import { useEffect, useState, useTransition } from "react";
import { useRouter } from "next/navigation";
import type { Blocker, ManagerTeamReport } from "@launchpad/api-client";
import { ApiError } from "@launchpad/api-client";
import { EmptyState, PageHeader, Reveal, Surface } from "@launchpad/ui";
import { getClient } from "@/lib/api";
import { clearSession, getAccessToken } from "@/lib/session";

const categoryLabels: Record<string, string> = {
  hr: "HR",
  it: "IT",
  manager: "Manager",
  other: "Other",
};

export default function ManagerPage() {
  const router = useRouter();
  const [pending, startTransition] = useTransition();
  const [team, setTeam] = useState<ManagerTeamReport[]>([]);
  const [blockers, setBlockers] = useState<Blocker[]>([]);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!getAccessToken()) {
      router.replace("/login");
      return;
    }

    let stale = false;

    startTransition(() => {
      void (async () => {
        try {
          const client = getClient();
          const [teamItems, blockerItems] = await Promise.all([
            client.getManagerTeam(),
            client.listManagerBlockers(),
          ]);
          if (stale) return;
          setTeam(teamItems);
          setBlockers(blockerItems);
        } catch (err) {
          if (stale) return;
          if (err instanceof ApiError && err.status === 401) {
            clearSession();
            router.replace("/login");
            return;
          }
          setError(err instanceof ApiError ? err.message : "Unable to load team overview");
        }
      })();
    });

    return () => {
      stale = true;
    };
  }, [router]);

  const nameFor = (employeeId: string) =>
    team.find((report) => report.employeeId === employeeId)?.name ?? employeeId;

  return (
    <div className="space-y-8">
      <Reveal>
        <PageHeader
          eyebrow="My team"
          title="Team overview"
          description="Direct reports' onboarding progress and the blockers they have raised."
        />
      </Reveal>

      {error ? (
        <p className="text-[var(--lp-danger)]" role="alert">
          {error}
        </p>
      ) : null}

      <Reveal delay={1}>
        <div>
          <h2 className="text-lg font-semibold">Direct reports</h2>
          {pending && team.length === 0 ? (
            <p className="mt-4 text-sm text-[var(--lp-ink-muted)]">Loading team…</p>
          ) : null}
          {!pending && team.length === 0 ? (
            <div className="mt-4">
              <Surface>
                <EmptyState
                  dense
                  title="No direct reports"
                  description="Employees who list you as their manager will appear here."
                />
              </Surface>
            </div>
          ) : (
            <ul className="mt-4 grid gap-4 sm:grid-cols-2 xl:grid-cols-3">
              {team.map((report) => (
                <li key={report.employeeId} className="lp-card rounded-[var(--lp-radius)] p-5">
                  <p className="font-semibold">{report.name}</p>
                  <dl className="mt-3 grid grid-cols-2 gap-2 text-sm">
                    <div>
                      <dt className="text-[var(--lp-ink-muted)]">Active</dt>
                      <dd className="font-semibold">{report.activeAssignments}</dd>
                    </div>
                    <div>
                      <dt className="text-[var(--lp-ink-muted)]">Completed</dt>
                      <dd className="font-semibold">{report.completedAssignments}</dd>
                    </div>
                    <div>
                      <dt className="text-[var(--lp-ink-muted)]">Overdue steps</dt>
                      <dd
                        className={
                          report.overdueSteps > 0
                            ? "font-semibold text-[var(--lp-danger)]"
                            : "font-semibold"
                        }
                      >
                        {report.overdueSteps}
                      </dd>
                    </div>
                    <div>
                      <dt className="text-[var(--lp-ink-muted)]">Pending approvals</dt>
                      <dd className="font-semibold">{report.pendingApprovals}</dd>
                    </div>
                  </dl>
                </li>
              ))}
            </ul>
          )}
        </div>
      </Reveal>

      <Reveal delay={2}>
        <Surface className="overflow-hidden p-0">
          <div className="border-b border-[var(--lp-border)] px-5 py-4">
            <h2 className="text-lg font-semibold">Blockers</h2>
            <p className="text-sm text-[var(--lp-ink-muted)]">{blockers.length} reported</p>
          </div>
          {blockers.length === 0 ? (
            <div className="p-5">
              <EmptyState
                dense
                title="No blockers"
                description="Blockers your reports raise from their journey steps appear here."
              />
            </div>
          ) : (
            <ul className="divide-y divide-[var(--lp-border)]">
              {blockers.map((blocker) => (
                <li key={blocker.id} className="px-5 py-4">
                  <div className="flex flex-wrap items-center justify-between gap-2">
                    <p className="font-medium">{nameFor(blocker.employeeId)}</p>
                    <span className="rounded-full bg-[var(--lp-accent)]/10 px-3 py-1 text-xs font-semibold text-[var(--lp-accent)]">
                      {categoryLabels[blocker.category] ?? blocker.category}
                    </span>
                  </div>
                  <p className="mt-1 text-sm text-[var(--lp-ink-muted)]">{blocker.message}</p>
                  <p className="mt-1 text-xs text-[var(--lp-ink-muted)]">
                    {new Date(blocker.createdAt).toLocaleString()}
                  </p>
                </li>
              ))}
            </ul>
          )}
        </Surface>
      </Reveal>
    </div>
  );
}
