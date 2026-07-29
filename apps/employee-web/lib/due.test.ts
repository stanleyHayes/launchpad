import { describe, expect, it } from "vitest";
import type { StepAssignment } from "@launchpad/api-client";
import { partitionStepsByDue } from "./due";

const now = new Date("2026-07-28T14:00:00");

function step(id: string, dueAt: string | null, status = "in_progress"): StepAssignment {
  return {
    id,
    organizationId: "org-1",
    journeyAssignmentId: "asg-1",
    journeyStepId: `js-${id}`,
    employeeId: "emp-1",
    stepType: "document",
    title: `Step ${id}`,
    instructions: "",
    position: 1,
    status,
    dueAt,
    createdAt: "2026-07-01T00:00:00",
  };
}

describe("partitionStepsByDue", () => {
  it("splits steps into due-today and overdue", () => {
    const steps = [
      step("today-early", "2026-07-28T08:00:00"),
      step("today-late", "2026-07-28T23:30:00"),
      step("yesterday", "2026-07-27T10:00:00"),
      step("last-week", "2026-07-20T10:00:00"),
      step("tomorrow", "2026-07-29T09:00:00"),
    ];

    const groups = partitionStepsByDue(steps, now);

    expect(groups.dueToday.map((s) => s.id)).toEqual(["today-early", "today-late"]);
    expect(groups.overdue.map((s) => s.id)).toEqual(["last-week", "yesterday"]);
  });

  it("excludes completed steps and steps without a due date", () => {
    const steps = [
      step("done-overdue", "2026-07-27T10:00:00", "completed"),
      step("done-today", "2026-07-28T09:00:00", "completed"),
      step("no-due", null),
    ];

    const groups = partitionStepsByDue(steps, now);

    expect(groups.dueToday).toEqual([]);
    expect(groups.overdue).toEqual([]);
  });

  it("a step due earlier today is due-today, not overdue", () => {
    const groups = partitionStepsByDue([step("morning", "2026-07-28T06:00:00")], now);

    expect(groups.dueToday.map((s) => s.id)).toEqual(["morning"]);
    expect(groups.overdue).toEqual([]);
  });
});
