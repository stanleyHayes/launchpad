"use client";

import Link from "next/link";
import { useEffect, useState, useTransition } from "react";
import { useRouter } from "next/navigation";
import type { JourneyAssignment, MeResponse } from "@launchpad/api-client";
import { ApiError } from "@launchpad/api-client";
import {
  EmptyState,
  MetricCard,
  PageHeader,
  Reveal,
  Surface,
} from "@launchpad/ui";
import { EmployeeShell } from "@/components/employee-shell";
import { getClient } from "@/lib/api";
import { clearSession, getAccessToken } from "@/lib/session";

function formatStatus(status: string): string {
  return status.replace(/_/g, " ");
}

export default function HomePage() {
  const router = useRouter();
  const [pending, startTransition] = useTransition();
  const [me, setMe] = useState<MeResponse | null>(null);
  const [assignments, setAssignments] = useState<JourneyAssignment[]>([]);
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
          const [profile, items] = await Promise.all([
            client.me(),
            client.listMyAssignments(),
          ]);
          setMe(profile);
          setAssignments(items);
        } catch (err) {
          if (err instanceof ApiError && err.status === 401) {
            clearSession();
            router.replace("/login");
            return;
          }
          setError(err instanceof ApiError ? err.message : "Unable to load home");
        }
      })();
    });
  }, [router]);

  const inProgressCount = assignments.filter(
    (item) => item.status === "in_progress" || item.status === "scheduled",
  ).length;
  const completedCount = assignments.filter((item) => item.status === "completed").length;

  return (
    <EmployeeShell>
      <div className="space-y-8">
        <Reveal>
          <Surface className="overflow-hidden">
            <PageHeader
              eyebrow="Employee workspace"
              title={me ? `Welcome, ${me.user.displayName}` : "Welcome"}
              description={
                me
                  ? `Track your onboarding journeys at ${me.organization?.name ?? "your organization"}.`
                  : "Loading your profile…"
              }
              actions={
                <Link
                  href="/assignments"
                  className="rounded-[var(--lp-radius)] bg-[var(--lp-accent)] px-4 py-2.5 text-sm font-semibold text-white"
                >
                  View my journeys
                </Link>
              }
            />
          </Surface>
        </Reveal>

        {error ? (
          <p className="text-[var(--lp-danger)]" role="alert">
            {error}
          </p>
        ) : null}

        <Reveal delay={1}>
          <section className="grid gap-4 sm:grid-cols-2 xl:grid-cols-3">
            <MetricCard
              label="My assignments"
              value={assignments.length || (pending ? "…" : "0")}
            />
            <MetricCard
              label="In progress"
              value={pending && assignments.length === 0 ? "…" : inProgressCount}
              hint="Active or scheduled journeys"
            />
            <MetricCard
              label="Completed"
              value={pending && assignments.length === 0 ? "…" : completedCount}
            />
          </section>
        </Reveal>

        <Reveal delay={2}>
          <Surface>
            <div className="flex flex-wrap items-center justify-between gap-3">
              <h2
                className="text-xl font-semibold"
                style={{ fontFamily: "var(--lp-font-display)" }}
              >
                My assignments
              </h2>
              <Link
                href="/assignments"
                className="text-sm font-semibold text-[var(--lp-accent)]"
              >
                View all
              </Link>
            </div>

            {assignments.length === 0 ? (
              <div className="mt-4">
                <EmptyState
                  dense
                  title="No assignments yet"
                  description="When your manager assigns a journey, it will appear here."
                />
              </div>
            ) : (
              <ul className="mt-4 divide-y divide-[var(--lp-border)]">
                {assignments.map((assignment) => (
                  <li key={assignment.id}>
                    <Link
                      href={`/assignments/${assignment.id}`}
                      className="flex flex-wrap items-center justify-between gap-3 py-3 transition hover:text-[var(--lp-accent)]"
                    >
                      <span>
                        <span className="block font-medium capitalize">
                          {formatStatus(assignment.status)}
                        </span>
                        <span className="text-sm text-[var(--lp-ink-muted)]">
                          {Math.round(assignment.progressPercent)}% complete
                          {assignment.dueAt
                            ? ` · Due ${new Date(assignment.dueAt).toLocaleDateString()}`
                            : ""}
                        </span>
                      </span>
                      <span aria-hidden="true">→</span>
                    </Link>
                  </li>
                ))}
              </ul>
            )}
          </Surface>
        </Reveal>
      </div>
    </EmployeeShell>
  );
}
