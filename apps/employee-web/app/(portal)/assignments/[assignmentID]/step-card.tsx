"use client";

import { useState, type SyntheticEvent } from "react";
import type { BlockerCategory, StepAssignment } from "@launchpad/api-client";
import { Button, cn } from "@launchpad/ui";
import { formatStatus } from "./status";

const PASSING_SCORE = 70;

const blockerCategories: { value: BlockerCategory; label: string }[] = [
  { value: "hr", label: "HR" },
  { value: "it", label: "IT" },
  { value: "manager", label: "Manager" },
  { value: "other", label: "Other" },
];

// Request-step item options (PRD §5.3.8); completing the step raises the
// corresponding equipment/access request.
const requestStepItems: Record<string, { value: string; label: string }[]> = {
  equipment_request: [
    { value: "laptop", label: "Laptop" },
    { value: "mobile", label: "Mobile device" },
    { value: "badge", label: "Access badge" },
    { value: "desk_equipment", label: "Desk equipment" },
    { value: "other", label: "Other" },
  ],
  access_request: [
    { value: "vpn", label: "VPN" },
    { value: "email", label: "Email" },
    { value: "software", label: "Software" },
    { value: "github_repo", label: "GitHub repository" },
    { value: "jira_project", label: "Jira project" },
    { value: "other", label: "Other" },
  ],
};

// Meeting-step type options (PRD §5.3.7); completing the step schedules the
// meeting with the submitted time and location.
const meetingStepTypes: { value: string; label: string }[] = [
  { value: "manager_intro", label: "Manager introduction" },
  { value: "hr_orientation", label: "HR orientation" },
  { value: "team_intro", label: "Team introduction" },
  { value: "buddy_checkin", label: "Buddy check-in" },
  { value: "architecture_walkthrough", label: "Architecture walkthrough" },
  { value: "role_coaching", label: "Role coaching" },
  { value: "first_week_review", label: "First-week review" },
];

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

export function StepCard({
  step,
  onComplete,
  completing,
  onReportBlocker,
}: {
  step: StepAssignment;
  onComplete: (stepId: string, payload: { submission?: Record<string, unknown>; score?: number }) => void;
  completing: string | null;
  onReportBlocker: (stepId: string, payload: { category: BlockerCategory; message: string }) => Promise<void>;
}) {
  const isDone = step.status === "completed";
  const isAwaiting = step.status === "awaiting_approval";
  const canAct = step.status === "pending" || step.status === "in_progress";
  const [blockerOpen, setBlockerOpen] = useState(false);
  const [blockerSending, setBlockerSending] = useState(false);
  const [blockerSent, setBlockerSent] = useState(false);
  const [blockerError, setBlockerError] = useState<string | null>(null);

  function onSubmit(event: SyntheticEvent<HTMLFormElement>) {
    event.preventDefault();
    const form = new FormData(event.currentTarget);

    if (step.stepType === "quiz") {
      const answers: Record<string, number> = {};
      for (const question of step.quizQuestions ?? []) {
        const value = form.get(`answer-${question.id}`);
        if (typeof value === "string") {
          answers[question.id] = Number(value);
        }
      }
      onComplete(step.id, { submission: { answers } });
      return;
    }

    if (step.stepType === "equipment_request" || step.stepType === "access_request") {
      const item = form.get("item");
      const notes = form.get("notes");
      const submission: Record<string, unknown> = {};
      if (typeof item === "string" && item) {
        submission.item = item;
      }
      if (typeof notes === "string" && notes.trim()) {
        submission.notes = notes.trim();
      }
      onComplete(step.id, { submission });
      return;
    }

    if (step.stepType === "meeting") {
      const meetingType = form.get("meetingType");
      const startsAtLocal = form.get("startsAt");
      const durationMin = form.get("durationMin");
      const location = form.get("location");
      const submission: Record<string, unknown> = {};
      if (typeof meetingType === "string" && meetingType) {
        submission.meetingType = meetingType;
      }
      if (typeof startsAtLocal === "string" && startsAtLocal) {
        // datetime-local carries no zone; interpret it in the employee's
        // local timezone and send an absolute timestamp.
        submission.startsAt = new Date(startsAtLocal).toISOString();
      }
      if (typeof durationMin === "string" && durationMin) {
        submission.durationMin = Number(durationMin);
      }
      if (typeof location === "string" && location.trim()) {
        submission.location = location.trim();
      }
      onComplete(step.id, { submission });
      return;
    }

    const notes = form.get("notes");
    const submission =
      typeof notes === "string" && notes.trim()
        ? { notes: notes.trim() }
        : undefined;
    onComplete(step.id, { submission });
  }

  function onBlockerSubmit(event: SyntheticEvent<HTMLFormElement>) {
    event.preventDefault();
    const form = new FormData(event.currentTarget);
    const category = String(form.get("category") ?? "other") as BlockerCategory;
    const message = String(form.get("message") ?? "").trim();

    if (!message) {
      setBlockerError("Describe what is blocking you.");
      return;
    }

    setBlockerSending(true);
    setBlockerError(null);

    void (async () => {
      try {
        await onReportBlocker(step.id, { category, message });
        setBlockerSent(true);
        setBlockerOpen(false);
      } catch (err) {
        setBlockerError(err instanceof Error ? err.message : "Unable to report blocker");
      } finally {
        setBlockerSending(false);
      }
    })();
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

      {canAct && (step.stepType === "equipment_request" || step.stepType === "access_request") ? (
        <form onSubmit={onSubmit} className="mt-4 space-y-3">
          <label className="block text-sm font-semibold">
            What do you need?
            <select className="lp-input mt-1.5" name="item">
              {(requestStepItems[step.stepType] ?? []).map((item) => (
                <option key={item.value} value={item.value}>
                  {item.label}
                </option>
              ))}
            </select>
          </label>
          <label className="block text-sm font-semibold">
            Notes (optional)
            <textarea
              className="lp-input mt-1.5 min-h-24 resize-y"
              name="notes"
              placeholder="Model, access level, project, …"
            />
          </label>
          <Button type="submit" disabled={completing === step.id}>
            {completing === step.id ? "Requesting…" : "Submit request"}
          </Button>
        </form>
      ) : null}

      {canAct && step.stepType === "meeting" ? (
        <form onSubmit={onSubmit} className="mt-4 space-y-3">
          <label className="block text-sm font-semibold">
            Meeting type
            <select className="lp-input mt-1.5" name="meetingType">
              {meetingStepTypes.map((meetingType) => (
                <option key={meetingType.value} value={meetingType.value}>
                  {meetingType.label}
                </option>
              ))}
            </select>
          </label>
          <div className="grid gap-3 sm:grid-cols-2">
            <label className="block text-sm font-semibold">
              Date and time
              <input className="lp-input mt-1.5" name="startsAt" type="datetime-local" required />
            </label>
            <label className="block text-sm font-semibold">
              Duration (minutes)
              <input
                className="lp-input mt-1.5"
                name="durationMin"
                type="number"
                min={5}
                max={480}
                step={5}
                defaultValue={30}
                required
              />
            </label>
          </div>
          <label className="block text-sm font-semibold">
            Location or meeting link (optional)
            <input
              className="lp-input mt-1.5"
              name="location"
              type="text"
              placeholder="https://meet.example.com/… or a room"
            />
          </label>
          <Button type="submit" disabled={completing === step.id}>
            {completing === step.id ? "Scheduling…" : "Schedule meeting"}
          </Button>
        </form>
      ) : null}

      {canAct && step.stepType === "quiz" ? (
        (step.quizQuestions?.length ?? 0) > 0 ? (
          <form onSubmit={onSubmit} className="mt-4 space-y-4">
            {step.score != null ? (
              <p className="rounded-[var(--lp-radius)] bg-amber-500/10 px-3 py-2 text-sm text-amber-800">
                Last score {step.score}% — you need at least {PASSING_SCORE}% to pass. Review the
                material and try again.
              </p>
            ) : null}
            {step.quizQuestions?.map((question, questionIndex) => (
              <fieldset key={question.id} className="space-y-2">
                <p className="text-sm font-semibold">
                  {questionIndex + 1}. {question.text}
                </p>
                {question.options.map((option, optionIndex) => (
                  <label
                    key={optionIndex}
                    className="flex items-center gap-2 text-sm text-[var(--lp-ink-muted)]"
                  >
                    <input
                      type="radio"
                      name={`answer-${question.id}`}
                      value={optionIndex}
                      required
                    />
                    {option}
                  </label>
                ))}
              </fieldset>
            ))}
            <p className="text-sm text-[var(--lp-ink-muted)]">
              Your answers are graded when you submit. You need at least {PASSING_SCORE}% to
              complete this step.
            </p>
            <Button type="submit" disabled={completing === step.id}>
              {completing === step.id ? "Submitting…" : "Submit quiz"}
            </Button>
          </form>
        ) : (
          <p className="mt-4 text-sm text-[var(--lp-ink-muted)]">
            This quiz has no questions yet. Contact your manager.
          </p>
        )
      ) : null}

      <div className="mt-4 border-t border-[var(--lp-border)] pt-3">
        {blockerSent ? (
          <p className="text-sm text-[var(--lp-success)]" role="status">
            Blocker reported — your manager and the support team have been notified.
          </p>
        ) : null}

        {!blockerSent && !blockerOpen ? (
          <button
            type="button"
            onClick={() => {
              setBlockerOpen(true);
            }}
            className="text-sm font-semibold text-[var(--lp-accent)]"
          >
            Report a blocker
          </button>
        ) : null}

        {!blockerSent && blockerOpen ? (
          <form onSubmit={onBlockerSubmit} className="mt-2 space-y-3">
            <label className="block text-sm font-semibold">
              Category
              <select name="category" className="lp-input mt-1.5" defaultValue="hr">
                {blockerCategories.map((category) => (
                  <option key={category.value} value={category.value}>
                    {category.label}
                  </option>
                ))}
              </select>
            </label>
            <label className="block text-sm font-semibold">
              What is blocking you?
              <textarea
                className="lp-input mt-1.5 min-h-20 resize-y"
                name="message"
                placeholder="Describe what you need to move forward…"
                required
              />
            </label>
            {blockerError ? (
              <p className="text-sm text-[var(--lp-danger)]" role="alert">
                {blockerError}
              </p>
            ) : null}
            <div className="flex gap-2">
              <Button type="submit" disabled={blockerSending}>
                {blockerSending ? "Sending…" : "Send blocker"}
              </Button>
              <Button
                type="button"
                variant="secondary"
                disabled={blockerSending}
                onClick={() => {
                  setBlockerOpen(false);
                  setBlockerError(null);
                }}
              >
                Cancel
              </Button>
            </div>
          </form>
        ) : null}
      </div>
    </li>
  );
}
