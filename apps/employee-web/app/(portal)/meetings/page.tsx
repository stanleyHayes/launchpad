"use client";

import { useEffect, useState, useTransition } from "react";
import { useRouter } from "next/navigation";
import type { Meeting } from "@launchpad/api-client";
import { ApiError } from "@launchpad/api-client";
import { EmptyState, PageHeader, Reveal, Surface } from "@launchpad/ui";
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

const statusBadgeClass: Record<string, string> = {
  scheduled:
    "rounded-full bg-[var(--lp-accent)]/10 px-3 py-1 text-xs font-semibold text-[var(--lp-accent)]",
  completed:
    "rounded-full bg-[var(--lp-success)]/10 px-3 py-1 text-xs font-semibold text-[var(--lp-success)]",
  no_show: "rounded-full px-3 py-1 text-xs font-semibold text-[var(--lp-danger)]",
  cancelled: "rounded-full px-3 py-1 text-xs font-semibold text-[var(--lp-ink-muted)]",
};

export default function MeetingsPage() {
  const router = useRouter();
  const [pending, startTransition] = useTransition();
  const [meetings, setMeetings] = useState<Meeting[]>([]);
  const [loaded, setLoaded] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [message, setMessage] = useState<string | null>(null);

  function reload(isStale?: () => boolean) {
    startTransition(() => {
      void (async () => {
        try {
          const items = await getClient().listMyMeetings();
          if (isStale?.()) return;
          setMeetings(items);
          setLoaded(true);
          setError(null);
        } catch (err) {
          if (isStale?.()) return;
          if (err instanceof ApiError && err.status === 401) {
            clearSession();
            router.replace("/login");
            return;
          }
          setError(err instanceof ApiError ? err.message : "Unable to load your meetings");
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

  function onCancel(meetingId: string) {
    setError(null);
    setMessage(null);
    startTransition(() => {
      void (async () => {
        try {
          await getClient().cancelMyMeeting(meetingId);
          setMessage("Meeting cancelled");
          reload();
        } catch (err) {
          setError(err instanceof ApiError ? err.message : "Unable to cancel the meeting");
        }
      })();
    });
  }

  const upcoming = meetings.filter((meeting) => meeting.status === "scheduled");
  const past = meetings.filter((meeting) => meeting.status !== "scheduled");

  return (
    <div className="space-y-8">
      <Reveal>
        <PageHeader
          eyebrow="Onboarding"
          title="Meetings"
          description="Your scheduled introductions, check-ins, and reviews. Schedule new ones from the meeting steps on your journey."
        />
      </Reveal>

      {error ? (
        <p className="text-[var(--lp-danger)]" role="alert">
          {error}
        </p>
      ) : null}
      {message ? <p className="text-[var(--lp-success)]">{message}</p> : null}

      <Reveal delay={1}>
        <Surface className="overflow-hidden p-0">
          <div className="border-b border-[var(--lp-border)] px-5 py-4">
            <h2 className="text-lg font-semibold">Upcoming</h2>
            <p className="text-sm text-[var(--lp-ink-muted)]">
              {pending && !loaded ? "Loading…" : `${upcoming.length} scheduled`}
            </p>
          </div>
          {loaded && upcoming.length === 0 ? (
            <div className="p-5">
              <EmptyState
                dense
                title="No upcoming meetings"
                description="Meetings you schedule from journey steps will appear here."
              />
            </div>
          ) : (
            <ul className="divide-y divide-[var(--lp-border)]">
              {upcoming.map((meeting) => (
                <li key={meeting.id} className="px-5 py-4">
                  <div className="flex flex-wrap items-start justify-between gap-3">
                    <div>
                      <p className="font-medium">{meeting.title}</p>
                      <p className="text-sm text-[var(--lp-ink-muted)]">
                        {typeLabels[meeting.type] ?? meeting.type} ·{" "}
                        {new Date(meeting.startsAt).toLocaleString()} · {meeting.durationMin} min
                      </p>
                      {meeting.location ? (
                        <p className="mt-1 text-sm text-[var(--lp-ink-muted)]">
                          {meeting.location}
                        </p>
                      ) : null}
                    </div>
                    <div className="flex items-center gap-3">
                      <span className={statusBadgeClass[meeting.status] ?? statusBadgeClass.scheduled}>
                        {meeting.status}
                      </span>
                      <button
                        type="button"
                        disabled={pending}
                        onClick={() => {
                          onCancel(meeting.id);
                        }}
                        className="text-sm font-semibold text-[var(--lp-danger)] disabled:opacity-60"
                      >
                        Cancel
                      </button>
                    </div>
                  </div>
                </li>
              ))}
            </ul>
          )}
        </Surface>
      </Reveal>

      {past.length > 0 ? (
        <Reveal delay={2}>
          <Surface className="overflow-hidden p-0">
            <div className="border-b border-[var(--lp-border)] px-5 py-4">
              <h2 className="text-lg font-semibold">Past</h2>
            </div>
            <ul className="divide-y divide-[var(--lp-border)]">
              {past.map((meeting) => (
                <li key={meeting.id} className="px-5 py-4">
                  <div className="flex flex-wrap items-start justify-between gap-3">
                    <div>
                      <p className="font-medium">{meeting.title}</p>
                      <p className="text-sm text-[var(--lp-ink-muted)]">
                        {typeLabels[meeting.type] ?? meeting.type} ·{" "}
                        {new Date(meeting.startsAt).toLocaleString()}
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
                    <span className={statusBadgeClass[meeting.status] ?? statusBadgeClass.cancelled}>
                      {meeting.status.replace("_", " ")}
                    </span>
                  </div>
                </li>
              ))}
            </ul>
          </Surface>
        </Reveal>
      ) : null}
    </div>
  );
}
