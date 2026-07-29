"use client";

import Link from "next/link";
import { useEffect, useState, useTransition, type SyntheticEvent } from "react";
import { useRouter } from "next/navigation";
import type { Assessment } from "@launchpad/api-client";
import { ApiError } from "@launchpad/api-client";
import { EmptyState, PageHeader, Reveal, Surface } from "@launchpad/ui";
import { getClient } from "@/lib/api";
import { clearSession, getAccessToken } from "@/lib/session";

const statusBadgeClass: Record<string, string> = {
  draft: "rounded-full px-3 py-1 text-xs font-semibold text-[var(--lp-ink-muted)]",
  published:
    "rounded-full bg-[var(--lp-success)]/10 px-3 py-1 text-xs font-semibold text-[var(--lp-success)]",
  archived:
    "rounded-full px-3 py-1 text-xs font-semibold text-[var(--lp-ink-muted)] line-through",
};

function formString(form: FormData, key: string): string {
  const value = form.get(key);
  return typeof value === "string" ? value.trim() : "";
}

export default function AssessmentsPage() {
  const router = useRouter();
  const [pending, startTransition] = useTransition();
  const [assessments, setAssessments] = useState<Assessment[]>([]);
  const [loaded, setLoaded] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [message, setMessage] = useState<string | null>(null);
  const [busyId, setBusyId] = useState<string | null>(null);

  function reload(isStale?: () => boolean) {
    startTransition(() => {
      void (async () => {
        try {
          const items = await getClient().listAssessments();
          if (isStale?.()) return;
          setAssessments(items);
          setLoaded(true);
          setError(null);
        } catch (err) {
          if (isStale?.()) return;
          if (err instanceof ApiError && err.status === 401) {
            clearSession();
            router.replace("/login");
            return;
          }
          setError(err instanceof ApiError ? err.message : "Unable to load assessments");
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

  function onCreate(event: SyntheticEvent<HTMLFormElement>) {
    event.preventDefault();
    setError(null);
    setMessage(null);
    const formEl = event.currentTarget;
    const form = new FormData(formEl);

    const questions = [
      {
        type: "single_choice" as const,
        text: "Sample question — edit me",
        options: ["Option A", "Option B"],
        correctOptions: [0],
      },
    ];

    startTransition(() => {
      void (async () => {
        try {
          const created = await getClient().createAssessment({
            title: formString(form, "title"),
            description: formString(form, "description") || undefined,
            questions,
            passingScore: Number(formString(form, "passingScore") || "70"),
            maxAttempts: Number(formString(form, "maxAttempts") || "0"),
            randomize: form.get("randomize") === "on",
          });
          router.push(`/assessments/${created.id}`);
        } catch (err) {
          if (err instanceof ApiError && err.status === 401) {
            clearSession();
            router.replace("/login");
            return;
          }
          setError(err instanceof ApiError ? err.message : "Unable to create assessment");
        }
      })();
    });
  }

  function onArchive(assessment: Assessment) {
    setError(null);
    setMessage(null);
    setBusyId(assessment.id);

    void (async () => {
      try {
        await getClient().archiveAssessment(assessment.id);
        setMessage(`"${assessment.title}" archived`);
        reload();
      } catch (err) {
        setError(err instanceof ApiError ? err.message : `Unable to archive "${assessment.title}"`);
      } finally {
        setBusyId(null);
      }
    })();
  }

  return (
    <div className="space-y-8">
      <Reveal>
        <PageHeader
          eyebrow="Operations"
          title="Assessments"
          description="Build scored assessments, publish them, and review attempts. Passing employees earn a certificate."
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
          <h2 className="text-lg font-semibold">New assessment</h2>
          <p className="mt-1 text-sm text-[var(--lp-ink-muted)]">
            Creates a draft with one starter question — open it to build the full question set.
          </p>
          <form className="mt-4 grid gap-3 sm:grid-cols-2" onSubmit={onCreate}>
            <input className="lp-input" name="title" placeholder="Title" required />
            <input
              className="lp-input"
              name="description"
              placeholder="Description (optional)"
            />
            <label className="block text-sm font-semibold">
              Passing score (%)
              <input
                className="lp-input mt-1.5"
                name="passingScore"
                type="number"
                min={0}
                max={100}
                defaultValue={70}
                required
              />
            </label>
            <label className="block text-sm font-semibold">
              Max attempts (0 = unlimited)
              <input
                className="lp-input mt-1.5"
                name="maxAttempts"
                type="number"
                min={0}
                defaultValue={0}
              />
            </label>
            <label className="flex items-center gap-2 text-sm font-semibold">
              <input type="checkbox" name="randomize" />
              Randomize question order per attempt
            </label>
            <div>
              <button
                type="submit"
                disabled={pending}
                className="rounded-[var(--lp-radius)] bg-[var(--lp-accent)] px-4 py-2.5 text-sm font-semibold text-white disabled:opacity-60"
              >
                Create draft
              </button>
            </div>
          </form>
        </Surface>
      </Reveal>

      <Reveal delay={2}>
        <Surface className="overflow-hidden p-0">
          <div className="border-b border-[var(--lp-border)] px-5 py-4">
            <h2 className="text-lg font-semibold">Assessments</h2>
            <p className="text-sm text-[var(--lp-ink-muted)]">
              {pending && !loaded ? "Loading…" : `${assessments.length} assessments`}
            </p>
          </div>
          {loaded && assessments.length === 0 ? (
            <div className="p-5">
              <EmptyState
                dense
                title="No assessments yet"
                description="Create a draft, add questions, then publish it into a journey assessment step."
              />
            </div>
          ) : (
            <ul className="divide-y divide-[var(--lp-border)]">
              {assessments.map((assessment) => (
                <li key={assessment.id} className="px-5 py-4">
                  <div className="flex flex-wrap items-start justify-between gap-3">
                    <div>
                      <Link
                        href={`/assessments/${assessment.id}`}
                        className="font-medium text-[var(--lp-brand)] underline-offset-2 hover:underline"
                      >
                        {assessment.title}
                      </Link>
                      <p className="text-sm text-[var(--lp-ink-muted)]">
                        {assessment.questions.length} questions · pass at {assessment.passingScore}%
                        {" · "}
                        {assessment.maxAttempts === 0
                          ? "unlimited attempts"
                          : `${assessment.maxAttempts} attempts`}
                        {assessment.randomize ? " · randomized" : ""}
                      </p>
                    </div>
                    <div className="flex flex-wrap items-center gap-2">
                      <span className={statusBadgeClass[assessment.status] ?? statusBadgeClass.draft}>
                        {assessment.status}
                      </span>
                      {assessment.status !== "archived" ? (
                        <button
                          type="button"
                          disabled={busyId === assessment.id}
                          className="lp-btn lp-btn--ghost"
                          onClick={() => {
                            onArchive(assessment);
                          }}
                        >
                          Archive
                        </button>
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
