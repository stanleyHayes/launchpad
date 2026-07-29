"use client";

import { useEffect, useState, type SyntheticEvent } from "react";
import { useRouter } from "next/navigation";
import type { JourneyAssignment, StepAssignment } from "@launchpad/api-client";
import { ApiError } from "@launchpad/api-client";
import { EmptyState, PageHeader, Surface } from "@launchpad/ui";
import { getClient } from "@/lib/api";
import { clearSession, getAccessToken } from "@/lib/session";

export default function AssignmentsPage() {
  const router = useRouter();
  const [assignments, setAssignments] = useState<JourneyAssignment[]>([]);
  const [steps, setSteps] = useState<Record<string, StepAssignment[]>>({});
  const [selected, setSelected] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!getAccessToken()) {
      router.replace("/login");
      return;
    }
    void getClient().listAssignments().then(setAssignments).catch((err: unknown) => {
      if (err instanceof ApiError && err.status === 401) {
        clearSession();
        router.replace("/login");
      } else {
        setError(err instanceof Error ? err.message : "Unable to load assignments");
      }
    });
  }, [router]);

  async function selectAssignment(id: string) {
    setSelected(id);
    if (!steps[id]) {
      try {
        const items = await getClient().listAssignmentSteps(id);
        setSteps((current) => ({ ...current, [id]: items }));
      } catch (err) {
        setError(err instanceof Error ? err.message : "Unable to load assignment steps");
      }
    }
  }

  async function override(event: SyntheticEvent<HTMLFormElement>, step: StepAssignment) {
    event.preventDefault();
    const form = new FormData(event.currentTarget);
    const action = String(form.get("action")) as "complete" | "skip" | "reopen";
    const reason = String(form.get("reason") ?? "").trim();
    if (!reason) {
      setError("An override reason is required.");
      return;
    }
    try {
      await getClient().overrideStep(step.id, { action, reason });
      setSteps((current) => ({
        ...current,
        [step.journeyAssignmentId]: current[step.journeyAssignmentId].map((item) =>
          item.id === step.id ? { ...item, status: action === "reopen" ? "in_progress" : action === "skip" ? "skipped" : "completed" } : item,
        ),
      }));
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Unable to override step");
    }
  }

  return (
    <div className="space-y-8">
      <PageHeader eyebrow="Operations" title="Assignments" description="Inspect active journeys and resolve exceptional step states with an audited reason." />
      {error ? <p className="text-[var(--lp-danger)]" role="alert">{error}</p> : null}
      <div className="grid gap-6 lg:grid-cols-[0.8fr_1.2fr]">
        <Surface className="p-0">
          {assignments.length === 0 ? (
            <div className="p-5"><EmptyState dense title="No assignments" description="Assign a published journey to an employee to begin." /></div>
          ) : (
            <ul className="divide-y divide-[var(--lp-border)]">
              {assignments.map((assignment) => (
                <li key={assignment.id}>
                  <button type="button" onClick={() => void selectAssignment(assignment.id)} className="w-full px-5 py-4 text-left hover:bg-[var(--lp-surface)]">
                    <span className="font-medium">Assignment {assignment.id.slice(0, 8)}</span>
                    <span className="mt-1 block text-sm text-[var(--lp-ink-muted)]">{assignment.status} · {Math.round(assignment.progressPercent)}%</span>
                  </button>
                </li>
              ))}
            </ul>
          )}
        </Surface>
        <Surface>
          {!selected ? <EmptyState dense title="Select an assignment" description="Its stages, attempts, and override controls will appear here." /> : (
            <ul className="space-y-4">
              {(steps[selected] ?? []).map((step) => (
                <li className="rounded-xl border border-[var(--lp-border)] p-4" key={step.id}>
                  <div className="flex items-center justify-between gap-3">
                    <div><p className="font-medium">{step.title}</p><p className="text-xs text-[var(--lp-ink-muted)]">{step.stage || "Journey"} · {step.status}{step.maxAttempts ? ` · ${step.attemptCount ?? 0}/${step.maxAttempts} attempts` : ""}</p></div>
                  </div>
                  <form className="mt-3 grid gap-2 sm:grid-cols-[8rem_1fr_auto]" onSubmit={(event) => void override(event, step)}>
                    <select className="lp-input" name="action" defaultValue={step.status === "blocked" ? "reopen" : "complete"}>
                      <option value="complete">Complete</option><option value="skip">Skip</option><option value="reopen">Reopen</option>
                    </select>
                    <input className="lp-input" name="reason" placeholder="Required audit reason" required />
                    <button className="lp-btn lp-btn--secondary" type="submit">Apply</button>
                  </form>
                </li>
              ))}
            </ul>
          )}
        </Surface>
      </div>
    </div>
  );
}
