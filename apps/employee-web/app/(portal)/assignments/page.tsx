"use client";

import Link from "next/link";
import { useEffect, useState, useTransition } from "react";
import { useRouter } from "next/navigation";
import type { JourneyAssignment } from "@launchpad/api-client";
import { ApiError } from "@launchpad/api-client";
import { EmptyState, PageHeader, Reveal, Surface } from "@launchpad/ui";
import { getClient } from "@/lib/api";
import { clearSession, getAccessToken } from "@/lib/session";

function formatStatus(status: string): string {
  return status.replace(/_/g, " ");
}

export default function AssignmentsPage() {
  const router = useRouter();
  const [pending, startTransition] = useTransition();
  const [assignments, setAssignments] = useState<JourneyAssignment[]>([]);
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
          const assignmentItems = await getClient().listMyAssignments();
          if (stale) return;
          setAssignments(assignmentItems);
        } catch (err) {
          if (stale) return;
          if (err instanceof ApiError && err.status === 401) {
            clearSession();
            router.replace("/login");
            return;
          }
          setError(err instanceof ApiError ? err.message : "Unable to load assignments");
        }
      })();
    });
    return () => {
      stale = true;
    };
  }, [router]);

  return (
          <div className="space-y-8">
        <Reveal>
          <Surface className="overflow-hidden">
            <PageHeader
              eyebrow="My journey"
              title="Assignments"
              description="Every onboarding journey assigned to you, with progress and due dates."
            />
          </Surface>
        </Reveal>

        {error ? (
          <p className="text-[var(--lp-danger)]" role="alert">
            {error}
          </p>
        ) : null}

        <Reveal delay={1}>
          <Surface>
            <p className="text-sm text-[var(--lp-ink-muted)]">
              {pending ? "Loading…" : `${assignments.length} assignments`}
            </p>

            {assignments.length === 0 && !pending ? (
              <div className="mt-4">
                <EmptyState
                  title="No assignments yet"
                  description="Your manager will assign onboarding journeys here when you're ready to begin."
                />
              </div>
            ) : (
              <ul className="mt-4 divide-y divide-[var(--lp-border)]">
                {assignments.map((assignment) => (
                  <li key={assignment.id}>
                    <Link
                      href={`/assignments/${assignment.id}`}
                      className="flex flex-wrap items-center justify-between gap-3 py-4 transition hover:text-[var(--lp-accent)]"
                    >
                      <span>
                        <span className="block font-medium capitalize">
                          Journey · {formatStatus(assignment.status)}
                        </span>
                        <span className="text-sm text-[var(--lp-ink-muted)]">
                          {Math.round(assignment.progressPercent)}% complete
                          {assignment.startsAt
                            ? ` · Started ${new Date(assignment.startsAt).toLocaleDateString()}`
                            : ""}
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
      );
}
