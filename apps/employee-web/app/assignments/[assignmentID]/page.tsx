"use client";

import Link from "next/link";
import { useEffect, useState, useTransition, type SyntheticEvent } from "react";
import { useParams, useRouter } from "next/navigation";
import type { JourneyAssignment, StepAssignment } from "@launchpad/api-client";
import { ApiError } from "@launchpad/api-client";
import { Button, EmptyState, PageHeader, Reveal, Surface, cn } from "@launchpad/ui";
import { EmployeeShell } from "@/components/employee-shell";
import { getClient } from "@/lib/api";
import { clearSession, getAccessToken } from "@/lib/session";

const PASSING_SCORE = 70;

function formatStatus(status: string): string {
  return status.replace(/_/g, " ");
}

function statusTone(status: string): string {
  switch (status) {
    case "completed":
      return "bg-[var(--lp-accent)]/10 text-[var(--lp-accent)]";
    case "awaiting_approval":
      return "bg-amber-500/10 text-amber-700";
    case "in_progress":
      return "bg-blue-500/10 text-blue-700";
    default:
      return "bg-[var(--lp-border)] text-[var(--lp-ink-muted)]";
  }
}

function StepCard({
  step,
  onComplete,
  completing,
}: {
  step: StepAssignment;
  onComplete: (stepId: string, payload: { submission?: Record<string, unknown>; score?: number }) => void;
  completing: string | null;
}) {
  const isDone = step.status === "completed";
  const isAwaiting = step.status === "awaiting_approval";
  const canAct = step.status === "pending" || step.status === "in_progress";

  function onSubmit(event: SyntheticEvent<HTMLFormElement>) {
    event.preventDefault();
    const form = new FormData(event.currentTarget);

    if (step.stepType === "quiz") {
      const rawScore = form.get("score");
      const score = typeof rawScore === "string" ? Number.parseFloat(rawScore) : NaN;
      if (Number.isNaN(score)) {
        return;
      }
      onComplete(step.id, { score });
      return;
    }

    const notes = form.get("notes");
    const submission =
      typeof notes === "string" && notes.trim()
        ? { notes: notes.trim() }
        : undefined;
    onComplete(step.id, { submission });
  }

  return (
    <li className="lp-card rounded-[var(--lp-radius)] p-5">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div>
          <p className="text-xs font-semibold uppercase tracking-[0.15em] text-[var(--lp-ink-muted)]">
            Step {step.position} · {step.stepType}
          </p>
          <h3
            className="mt-1 text-lg font-semibold"
            style={{ fontFamily: "var(--lp-font-display)" }}
          >
            {step.title}
          </h3>
        </div>
        <span
          className={cn(
            "rounded-full px-3 py-1 text-xs font-semibold capitalize",
            statusTone(step.status),
          )}
        >
          {formatStatus(step.status)}
        </span>
      </div>

      {step.instructions ? (
        <p className="mt-3 text-sm text-[var(--lp-ink-muted)]">{step.instructions}</p>
      ) : null}

      {step.dueAt ? (
        <p className="mt-2 text-sm text-[var(--lp-ink-muted)]">
          Due {new Date(step.dueAt).toLocaleDateString()}
        </p>
      ) : null}

      {isDone ? (
        <div className="mt-4 text-sm text-[var(--lp-ink-muted)]">
          {step.completedAt
            ? `Completed ${new Date(step.completedAt).toLocaleString()}`
            : "Completed"}
          {step.score != null ? ` · Score ${step.score}%` : null}
        </div>
      ) : null}

      {isAwaiting ? (
        <div className="mt-4 rounded-[var(--lp-radius)] bg-amber-500/10 px-3 py-2 text-sm text-amber-800">
          Awaiting manager approval before this step can be marked complete.
        </div>
      ) : null}

      {canAct && step.stepType === "approval" ? (
        <div className="mt-4">
          <Button
            type="button"
            disabled={completing === step.id}
            onClick={() => {
              onComplete(step.id, {});
            }}
          >
            {completing === step.id ? "Submitting…" : "Submit for approval"}
          </Button>
        </div>
      ) : null}

      {canAct && (step.stepType === "document" || step.stepType === "task") ? (
        <form onSubmit={onSubmit} className="mt-4 space-y-3">
          <label className="block text-sm font-semibold">
            Notes (optional)
            <textarea
              className="lp-input mt-1.5 min-h-24 resize-y"
              name="notes"
              defaultValue={
                typeof step.submission?.notes === "string" ? step.submission.notes : ""
              }
              placeholder="Add any notes about completing this step…"
            />
          </label>
          <Button type="submit" disabled={completing === step.id}>
            {completing === step.id ? "Saving…" : "Mark complete"}
          </Button>
        </form>
      ) : null}

      {canAct && step.stepType === "quiz" ? (
        <form onSubmit={onSubmit} className="mt-4 space-y-3">
          <label className="block text-sm font-semibold">
            Quiz score (%)
            <input
              className="lp-input mt-1.5"
              name="score"
              type="number"
              min={0}
              max={100}
              step={1}
              required
              defaultValue={step.score ?? undefined}
              placeholder={`Minimum ${PASSING_SCORE}% to pass`}
            />
          </label>
          <p className="text-sm text-[var(--lp-ink-muted)]">
            Enter your quiz score. You need at least {PASSING_SCORE}% to complete this step.
          </p>
          <Button type="submit" disabled={completing === step.id}>
            {completing === step.id ? "Submitting…" : "Submit quiz"}
          </Button>
        </form>
      ) : null}
    </li>
  );
}

export default function AssignmentDetailPage() {
  const router = useRouter();
  const params = useParams<{ assignmentID: string }>();
  const assignmentId = params.assignmentID;
  const [pending, startTransition] = useTransition();
  const [completing, setCompleting] = useState<string | null>(null);
  const [assignment, setAssignment] = useState<JourneyAssignment | null>(null);
  const [steps, setSteps] = useState<StepAssignment[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [message, setMessage] = useState<string | null>(null);

  function reload() {
    startTransition(() => {
      void (async () => {
        try {
          const client = getClient();
          const [item, stepItems] = await Promise.all([
            client.getAssignment(assignmentId),
            client.listAssignmentSteps(assignmentId),
          ]);
          setAssignment(item);
          setSteps(stepItems.sort((a, b) => a.position - b.position));
        } catch (err) {
          if (err instanceof ApiError && err.status === 401) {
            clearSession();
            router.replace("/login");
            return;
          }
          setError(err instanceof ApiError ? err.message : "Unable to load assignment");
        }
      })();
    });
  }

  useEffect(() => {
    if (!getAccessToken()) {
      router.replace("/login");
      return;
    }
    reload();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [router, assignmentId]);

  function completeStep(
    stepId: string,
    payload: { submission?: Record<string, unknown>; score?: number },
  ) {
    setError(null);
    setMessage(null);
    setCompleting(stepId);

    startTransition(() => {
      void (async () => {
        try {
          await getClient().completeStep(stepId, payload);
          setMessage("Step updated.");
          reload();
        } catch (err) {
          if (err instanceof ApiError && err.status === 401) {
            clearSession();
            router.replace("/login");
            return;
          }
          setError(err instanceof ApiError ? err.message : "Unable to complete step");
        } finally {
          setCompleting(null);
        }
      })();
    });
  }

  return (
    <EmployeeShell>
      <div className="space-y-8">
        <Reveal>
          <Surface className="overflow-hidden">
            <PageHeader
              eyebrow="My journey"
              title={
                assignment
                  ? `Assignment · ${formatStatus(assignment.status)}`
                  : "Assignment"
              }
              description={
                assignment
                  ? `${Math.round(assignment.progressPercent)}% complete${
                      assignment.dueAt
                        ? ` · Due ${new Date(assignment.dueAt).toLocaleDateString()}`
                        : ""
                    }`
                  : "Loading assignment details…"
              }
              actions={
                <Link
                  href="/assignments"
                  className="rounded-[var(--lp-radius)] border border-[var(--lp-border)] px-4 py-2.5 text-sm font-semibold"
                >
                  Back to list
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

        {message ? (
          <p className="text-sm text-[var(--lp-accent)]" role="status">
            {message}
          </p>
        ) : null}

        <Reveal delay={1}>
          <Surface>
            <h2
              className="text-xl font-semibold"
              style={{ fontFamily: "var(--lp-font-display)" }}
            >
              Steps
            </h2>

            {pending && steps.length === 0 ? (
              <p className="mt-4 text-sm text-[var(--lp-ink-muted)]">Loading steps…</p>
            ) : null}

            {!pending && steps.length === 0 ? (
              <div className="mt-4">
                <EmptyState
                  dense
                  title="No steps yet"
                  description="Steps for this journey will appear here once they are provisioned."
                />
              </div>
            ) : (
              <ul className="mt-4 space-y-4">
                {steps.map((step) => (
                  <StepCard
                    key={step.id}
                    step={step}
                    completing={completing}
                    onComplete={completeStep}
                  />
                ))}
              </ul>
            )}
          </Surface>
        </Reveal>
      </div>
    </EmployeeShell>
  );
}
