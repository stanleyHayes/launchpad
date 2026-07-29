import type { StepAssignment } from "@launchpad/api-client";

export type DueGroups = {
  dueToday: StepAssignment[];
  overdue: StepAssignment[];
};

function startOfDay(date: Date): Date {
  const start = new Date(date);
  start.setHours(0, 0, 0, 0);
  return start;
}

function endOfDay(date: Date): Date {
  const end = startOfDay(date);
  end.setDate(end.getDate() + 1);
  return end;
}

// partitionStepsByDue splits open steps into "due today" (due date falls on
// the current local calendar day) and "overdue" (due date passed before
// today). Completed steps never appear; steps without a due date are ignored.
// The two groups are disjoint: a step due earlier today is "due today", not
// overdue, until the day rolls over.
export function partitionStepsByDue(steps: StepAssignment[], now: Date): DueGroups {
  const dayStart = startOfDay(now);
  const dayEnd = endOfDay(now);

  const groups: DueGroups = { dueToday: [], overdue: [] };

  for (const step of steps) {
    if (!step.dueAt || step.status === "completed") continue;

    const dueAt = new Date(step.dueAt);

    if (dueAt < dayStart) {
      groups.overdue.push(step);
    } else if (dueAt < dayEnd) {
      groups.dueToday.push(step);
    }
  }

  const byDueAt = (a: StepAssignment, b: StepAssignment) =>
    new Date(a.dueAt ?? 0).getTime() - new Date(b.dueAt ?? 0).getTime();

  groups.overdue.sort(byDueAt);
  groups.dueToday.sort(byDueAt);

  return groups;
}
