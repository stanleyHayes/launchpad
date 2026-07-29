"use client";

import Link from "next/link";
import { useEffect, useState, useTransition } from "react";
import { useParams, useRouter } from "next/navigation";
import type { BlockerCategory, JourneyAssignment, StepAssignment } from "@launchpad/api-client";
import { ApiError } from "@launchpad/api-client";
import { EmptyState, PageHeader, Reveal, Surface } from "@launchpad/ui";
import { getClient } from "@/lib/api";
import { clearSession, getAccessToken } from "@/lib/session";
import { StepCard } from "./step-card";
import { formatStatus } from "./status";

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

  function reload(isStale?: () => boolean) {
    startTransition(() => {
      void (async () => {
        try {
          const client = getClient();
          const [item, stepItems] = await Promise.all([
            client.getAssignment(assignmentId),
            client.listAssignmentSteps(assignmentId),
          ]);
          if (isStale?.()) return;
          setAssignment(item);
          setSteps(stepItems.sort((a, b) => a.position - b.position));
        } catch (err) {
          if (isStale?.()) return;
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
    let stale = false;
    reload(() => stale);
    return () => {
      stale = true;
    };
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

  async function reportBlocker(
    stepId: string,
    payload: { category: BlockerCategory; message: string },
  ) {
    try {
      await getClient().reportBlocker({ stepAssignmentId: stepId, ...payload });
    } catch (err) {
      if (err instanceof ApiError && err.status === 401) {
        clearSession();
        router.replace("/login");
        return;
      }
      throw err instanceof ApiError ? err : new Error("Unable to report blocker");
    }
  }

  return (
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
                    onReportBlocker={reportBlocker}
                  />
                ))}
              </ul>
            )}
          </Surface>
        </Reveal>
      </div>
      );
}
