"use client";

import { useEffect, useState, useTransition, type SyntheticEvent } from "react";
import { useRouter } from "next/navigation";
import type { Employee, Meeting, MeetingStatus, MeetingType } from "@launchpad/api-client";
import { ApiError } from "@launchpad/api-client";
import { Select, EmptyState, PageHeader, Reveal, Surface } from "@launchpad/ui";
import { getClient } from "@/lib/api";
import { clearSession, getAccessToken } from "@/lib/session";

const typeLabels: Record<string, string> = {
  manager_intro: "Manager introduction",
  hr_orientation: "HR orientation",
  team_intro: "Team introduction",
  buddy_checkin: "Buddy check-in",
  architecture_walkthrough: "Architecture walkthrough",
  role_coaching: "Role coaching",
  first_week_review: "First-week review",
};

const meetingTypes = Object.entries(typeLabels).map(([value, label]) => ({ value, label }));

const statusBadgeClass: Record<string, string> = {
  scheduled:
    "rounded-full bg-[var(--lp-accent)]/10 px-3 py-1 text-xs font-semibold text-[var(--lp-accent)]",
  completed:
    "rounded-full bg-[var(--lp-success)]/10 px-3 py-1 text-xs font-semibold text-[var(--lp-success)]",
  no_show: "rounded-full px-3 py-1 text-xs font-semibold text-[var(--lp-danger)]",
  cancelled: "rounded-full px-3 py-1 text-xs font-semibold text-[var(--lp-ink-muted)]",
};

const statusFilters: { value: "" | MeetingStatus; label: string }[] = [
  { value: "", label: "All statuses" },
  { value: "scheduled", label: "Scheduled" },
  { value: "completed", label: "Completed" },
  { value: "no_show", label: "No-show" },
  { value: "cancelled", label: "Cancelled" },
];

function formString(form: FormData, key: string): string {
  const value = form.get(key);
  return typeof value === "string" ? value.trim() : "";
}

export default function MeetingsPage() {
  const router = useRouter();
  const [pending, startTransition] = useTransition();
  const [meetings, setMeetings] = useState<Meeting[]>([]);
  const [employees, setEmployees] = useState<Record<string, string>>({});
  const [employeeOptions, setEmployeeOptions] = useState<Employee[]>([]);
  const [statusFilter, setStatusFilter] = useState<"" | MeetingStatus>("");
  const [loaded, setLoaded] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [message, setMessage] = useState<string | null>(null);

  function reload(isStale?: () => boolean, status: "" | MeetingStatus = statusFilter) {
    startTransition(() => {
      void (async () => {
        try {
          const client = getClient();
          const [items, employeeItems] = await Promise.all([
            client.listOrgMeetings(status || undefined),
            client.listEmployees(200),
          ]);
          if (isStale?.()) return;
          setMeetings(items);
          setEmployees(
            Object.fromEntries(
              employeeItems.map((employee: Employee) => [
                employee.id,
                `${employee.firstName} ${employee.lastName}`.trim(),
              ]),
            ),
          );
          setEmployeeOptions(employeeItems);
          setLoaded(true);
          setError(null);
        } catch (err) {
          if (isStale?.()) return;
          if (err instanceof ApiError && err.status === 401) {
            clearSession();
            router.replace("/login");
            return;
          }
          setError(err instanceof ApiError ? err.message : "Unable to load meetings");
          setLoaded(true);
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
    // eslint-disable-next-line react-hooks/exhaustive-deps -- initial load only
  }, [router]);

  function act(action: () => Promise<unknown>, success: string) {
    setError(null);
    setMessage(null);
    startTransition(() => {
      void (async () => {
        try {
          await action();
          setMessage(success);
          reload();
        } catch (err) {
          if (err instanceof ApiError && err.status === 401) {
            clearSession();
            router.replace("/login");
            return;
          }
          setError(err instanceof ApiError ? err.message : "Unable to update the meeting");
        }
      })();
    });
  }

  function onSchedule(event: SyntheticEvent<HTMLFormElement>) {
    event.preventDefault();
    const formEl = event.currentTarget;
    const form = new FormData(formEl);

    const startsAtLocal = formString(form, "startsAt");
    if (!startsAtLocal) return;

    act(
      () =>
        getClient().createOrgMeeting({
          title: formString(form, "title"),
          type: formString(form, "type") as MeetingType,
          attendeeEmployeeId: formString(form, "attendeeEmployeeId"),
          startsAt: new Date(startsAtLocal).toISOString(),
          durationMin: Number(formString(form, "durationMin")) || undefined,
          location: formString(form, "location") || undefined,
        }),
      "Meeting scheduled",
    );
    formEl.reset();
  }

  function complete(meeting: Meeting, noShow: boolean) {
    const notesLink = noShow
      ? undefined
      : (window.prompt("Link to meeting notes or recording (optional)") ?? "").trim() || undefined;
    act(
      () => getClient().completeOrgMeeting(meeting.id, { noShow, notesLink }),
      noShow ? "Marked as no-show" : "Meeting marked completed",
    );
  }

  function reschedule(meeting: Meeting) {
    const startsAt = window.prompt("New start time (ISO 8601)", meeting.startsAt)?.trim();
    if (!startsAt || Number.isNaN(new Date(startsAt).getTime())) return;
    const durationText = window.prompt("Duration in minutes", String(meeting.durationMin))?.trim();
    const durationMin = Number(durationText);
    if (!Number.isInteger(durationMin) || durationMin < 5 || durationMin > 480) return;
    const location = window.prompt("Location or meeting URL", meeting.location ?? "")?.trim();
    if (location === undefined) return;
    act(
      () => getClient().rescheduleOrgMeeting(meeting.id, {
        startsAt: new Date(startsAt).toISOString(), durationMin, location,
      }),
      "Meeting rescheduled",
    );
  }

  return (
    <div className="space-y-8">
      <Reveal>
        <PageHeader
          eyebrow="Operations"
          title="Meetings"
          description="Onboarding meetings for your team. Schedule new ones, then record attendance when they happen."
        />
      </Reveal>

      {error ? (
        <p className="text-[var(--lp-danger)]" role="alert">
          {error}
        </p>
      ) : null}
      {message ? <p className="text-[var(--lp-success)]">{message}</p> : null}

      <Reveal delay={1}>
        <Surface>
          <h2 className="text-lg font-semibold">Schedule a meeting</h2>
          <form className="mt-4 grid gap-3" onSubmit={onSchedule}>
            <div className="grid gap-3 sm:grid-cols-2">
              <input className="lp-input" name="title" placeholder="Meeting title" required />
              <Select className="lp-input" name="attendeeEmployeeId" required>
                {employeeOptions.map((employee) => (
                  <option key={employee.id} value={employee.id}>
                    {`${employee.firstName} ${employee.lastName}`.trim()}
                  </option>
                ))}
              </Select>
            </div>
            <div className="grid gap-3 sm:grid-cols-3">
              <Select className="lp-input" name="type">
                {meetingTypes.map((meetingType) => (
                  <option key={meetingType.value} value={meetingType.value}>
                    {meetingType.label}
                  </option>
                ))}
              </Select>
              <input className="lp-input" name="startsAt" type="datetime-local" required />
              <input
                className="lp-input"
                name="durationMin"
                type="number"
                min={5}
                max={480}
                step={5}
                defaultValue={30}
                required
              />
            </div>
            <input
              className="lp-input"
              name="location"
              placeholder="Meeting link or location (optional)"
            />
            <div>
              <button
                type="submit"
                disabled={pending}
                className="rounded-[var(--lp-radius)] bg-[var(--lp-accent)] px-4 py-2.5 text-sm font-semibold text-white disabled:opacity-60"
              >
                Schedule meeting
              </button>
            </div>
          </form>
        </Surface>
      </Reveal>

      <Reveal delay={2}>
        <Surface className="overflow-hidden p-0">
          <div className="flex flex-wrap items-center justify-between gap-3 border-b border-[var(--lp-border)] px-5 py-4">
            <div>
              <h2 className="text-lg font-semibold">Team meetings</h2>
              <p className="text-sm text-[var(--lp-ink-muted)]">
                {pending && !loaded ? "Loading…" : `${meetings.length} meetings`}
              </p>
            </div>
            <Select
              className="lp-input w-auto"
              value={statusFilter}
              onChange={(event) => {
                const status = event.target.value as "" | MeetingStatus;
                setStatusFilter(status);
                reload(undefined, status);
              }}
            >
              {statusFilters.map((filter) => (
                <option key={filter.value} value={filter.value}>
                  {filter.label}
                </option>
              ))}
            </Select>
          </div>
          {loaded && meetings.length === 0 ? (
            <div className="p-5">
              <EmptyState
                dense
                title="No meetings yet"
                description="Schedule one above, or add meeting steps to a journey so employees pick their own times."
              />
            </div>
          ) : (
            <ul className="divide-y divide-[var(--lp-border)]">
              {meetings.map((meeting) => (
                <li key={meeting.id} className="px-5 py-4">
                  <div className="flex flex-wrap items-start justify-between gap-3">
                    <div>
                      <p className="font-medium">
                        {meeting.title}
                        <span className="ml-2 text-sm font-normal text-[var(--lp-ink-muted)]">
                          {employees[meeting.attendeeEmployeeId] ?? meeting.attendeeEmployeeId}
                        </span>
                      </p>
                      <p className="text-sm text-[var(--lp-ink-muted)]">
                        {typeLabels[meeting.type] ?? meeting.type} ·{" "}
                        {new Date(meeting.startsAt).toLocaleString()} · {meeting.durationMin} min
                        {meeting.location ? ` · ${meeting.location}` : ""}
                      </p>
                      {meeting.notesLink ? (
                        <a
                          href={meeting.notesLink}
                          target="_blank"
                          rel="noreferrer"
                          className="mt-1 inline-block text-sm font-semibold text-[var(--lp-accent)]"
                        >
                          Meeting notes
                        </a>
                      ) : null}
                    </div>
                    <div className="flex items-center gap-3">
                      <span className={statusBadgeClass[meeting.status] ?? statusBadgeClass.scheduled}>
                        {meeting.status.replace("_", " ")}
                      </span>
                      {meeting.status === "scheduled" ? (
                        <>
                          <button
                            type="button"
                            disabled={pending}
                            onClick={() => reschedule(meeting)}
                            className="text-sm font-semibold text-[var(--lp-brand)] disabled:opacity-60"
                          >
                            Reschedule
                          </button>
                          <button
                            type="button"
                            disabled={pending}
                            onClick={() => {
                              complete(meeting, false);
                            }}
                            className="text-sm font-semibold text-[var(--lp-success)] disabled:opacity-60"
                          >
                            Complete
                          </button>
                          <button
                            type="button"
                            disabled={pending}
                            onClick={() => {
                              complete(meeting, true);
                            }}
                            className="text-sm font-semibold text-[var(--lp-ink-muted)] disabled:opacity-60"
                          >
                            No-show
                          </button>
                          <button
                            type="button"
                            disabled={pending}
                            onClick={() => {
                              act(
                                () => getClient().cancelOrgMeeting(meeting.id),
                                "Meeting cancelled",
                              );
                            }}
                            className="text-sm font-semibold text-[var(--lp-danger)] disabled:opacity-60"
                          >
                            Cancel
                          </button>
                        </>
                      ) : null}
                    </div>
                  </div>
                </li>
              ))}
            </ul>
          )}
        </Surface>
      </Reveal>
    </div>
  );
}
