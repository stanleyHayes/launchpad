"use client";

import Link from "next/link";
import { useEffect, useState, useTransition } from "react";
import { useRouter } from "next/navigation";
import type { EmployeeContact, JourneyAssignment, MeResponse, StepAssignment } from "@launchpad/api-client";
import { ApiError } from "@launchpad/api-client";
import {
  EmptyState,
  MetricCard,
  PageHeader,
  Reveal,
  Surface,
} from "@launchpad/ui";
import { getClient } from "@/lib/api";
import { partitionStepsByDue } from "@/lib/due";
import { clearSession, getAccessToken } from "@/lib/session";

function formatStatus(status: string): string {
  return status.replace(/_/g, " ");
}

export default function HomePage() {
  const router = useRouter();
  const [pending, startTransition] = useTransition();
  const [me, setMe] = useState<MeResponse | null>(null);
  const [assignments, setAssignments] = useState<JourneyAssignment[]>([]);
  const [steps, setSteps] = useState<StepAssignment[]>([]);
  const [contacts, setContacts] = useState<EmployeeContact[]>([]);
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
          const [profile, items, contactItems] = await Promise.all([
            client.me(),
            client.listMyAssignments(),
            client.listMyContacts(),
          ]);
          const stepGroups = await Promise.all(
            items
              .filter((item) => item.status !== "completed")
              .map((item) => client.listAssignmentSteps(item.id)),
          );
          setMe(profile);
          setAssignments(items);
          setContacts(contactItems);
          setSteps(stepGroups.flat());
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
  const dueGroups = partitionStepsByDue(steps, new Date());

  function renderDueSection(title: string, items: StepAssignment[]) {
    return (
      <Surface>
        <h2
          className="text-xl font-semibold"
          style={{ fontFamily: "var(--lp-font-display)" }}
        >
          {title}
        </h2>
        {items.length === 0 ? (
          <p className="mt-3 text-sm text-[var(--lp-ink-muted)]">
            {pending && steps.length === 0 ? "Loading…" : "Nothing here — you're on track."}
          </p>
        ) : (
          <ul className="mt-3 divide-y divide-[var(--lp-border)]">
            {items.map((step) => (
              <li key={step.id}>
                <Link
                  href={`/assignments/${step.journeyAssignmentId}`}
                  className="flex flex-wrap items-center justify-between gap-3 py-3 transition hover:text-[var(--lp-accent)]"
                >
                  <span>
                    <span className="block font-medium">{step.title}</span>
                    <span className="text-sm text-[var(--lp-ink-muted)]">
                      Due {step.dueAt ? new Date(step.dueAt).toLocaleDateString() : ""}
                    </span>
                  </span>
                  <span aria-hidden="true">→</span>
                </Link>
              </li>
            ))}
          </ul>
        )}
      </Surface>
    );
  }

  return (
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
              icon="workflow"
              accent="#3b82f6"
              label="My assignments"
              value={assignments.length || (pending ? "…" : "0")}
            />
            <MetricCard
              icon="clock"
              accent="#f59e0b"
              label="In progress"
              value={pending && assignments.length === 0 ? "…" : inProgressCount}
              hint="Active or scheduled journeys"
            />
            <MetricCard
              icon="check"
              accent="#10b981"
              label="Completed"
              value={pending && assignments.length === 0 ? "…" : completedCount}
            />
          </section>
        </Reveal>

        <Reveal delay={2}>
          <section className="grid gap-4 lg:grid-cols-2">
            {renderDueSection("Due today", dueGroups.dueToday)}
            {renderDueSection("Overdue", dueGroups.overdue)}
          </section>
        </Reveal>

        <Reveal delay={2}>
          <Surface>
            <h2 className="text-xl font-semibold" style={{ fontFamily: "var(--lp-font-display)" }}>Your onboarding contacts</h2>
            {contacts.length ? (
              <ul className="mt-3 grid gap-3 sm:grid-cols-2">
                {contacts.map((contact) => (
                  <li key={contact.id} className="rounded-[var(--lp-radius)] border border-[var(--lp-border)] p-4">
                    <p className="text-xs font-semibold uppercase tracking-wide text-[var(--lp-ink-muted)]">{contact.kind}</p>
                    <p className="mt-1 font-semibold">{contact.name}</p>
                    <a className="text-sm text-[var(--lp-accent)]" href={`mailto:${contact.workEmail}`}>{contact.workEmail}</a>
                    {contact.team || contact.location ? <p className="mt-1 text-xs text-[var(--lp-ink-muted)]">{[contact.team, contact.location].filter(Boolean).join(" · ")}</p> : null}
                  </li>
                ))}
              </ul>
            ) : <p className="mt-3 text-sm text-[var(--lp-ink-muted)]">Your manager and buddy will appear here when assigned.</p>}
          </Surface>
        </Reveal>

        <Reveal delay={3}>
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
      );
}
