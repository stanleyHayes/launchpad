"use client";

import Link from "next/link";
import { useEffect, useState, useTransition, type SyntheticEvent } from "react";
import { useParams, useRouter } from "next/navigation";
import type { JourneyStep, JourneyTemplate } from "@launchpad/api-client";
import { ApiError } from "@launchpad/api-client";
import { EmptyState, PageHeader, Reveal, Surface } from "@launchpad/ui";
import { AdminShell } from "@/components/admin-shell";
import { getClient } from "@/lib/api";
import { clearSession, getAccessToken } from "@/lib/session";

function formString(form: FormData, key: string): string {
  const value = form.get(key);
  return typeof value === "string" ? value.trim() : "";
}

export default function JourneyDetailPage() {
  const router = useRouter();
  const params = useParams<{ journeyID: string }>();
  const journeyId = params.journeyID;
  const [pending, startTransition] = useTransition();
  const [journey, setJourney] = useState<JourneyTemplate | null>(null);
  const [steps, setSteps] = useState<JourneyStep[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [message, setMessage] = useState<string | null>(null);

  function reload() {
    startTransition(() => {
      void (async () => {
        try {
          const client = getClient();
          const [template, stepItems] = await Promise.all([
            client.getJourney(journeyId),
            client.listJourneySteps(journeyId),
          ]);
          setJourney(template);
          setSteps(stepItems);
        } catch (err) {
          if (err instanceof ApiError && err.status === 401) {
            clearSession();
            router.replace("/login");
            return;
          }
          setError(err instanceof ApiError ? err.message : "Unable to load journey");
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
    // eslint-disable-next-line react-hooks/exhaustive-deps -- initial load only
  }, [router, journeyId]);

  function onAddStep(event: SyntheticEvent<HTMLFormElement>) {
    event.preventDefault();
    setError(null);
    setMessage(null);

    const form = new FormData(event.currentTarget);
    startTransition(() => {
      void (async () => {
        try {
          await getClient().addJourneyStep(journeyId, {
            stepType: formString(form, "stepType"),
            title: formString(form, "title"),
            instructions: formString(form, "instructions"),
            dueOffsetDays: Number(formString(form, "dueOffsetDays") || "0"),
          });
          event.currentTarget.reset();
          setMessage("Step added");
          reload();
        } catch (err) {
          setError(err instanceof ApiError ? err.message : "Unable to add step");
        }
      })();
    });
  }

  function onPublish() {
    setError(null);
    setMessage(null);
    startTransition(() => {
      void (async () => {
        try {
          await getClient().publishJourney(journeyId);
          setMessage("Journey published");
          reload();
        } catch (err) {
          setError(err instanceof ApiError ? err.message : "Unable to publish journey");
        }
      })();
    });
  }

  return (
    <AdminShell>
      <div className="space-y-8">
        <Reveal>
          <PageHeader
            eyebrow="Journey detail"
            title={journey?.name ?? "Journey"}
            description={
              journey
                ? `${journey.status} · version ${String(journey.currentVersion)}`
                : "Loading…"
            }
            actions={
              <Link href="/journeys" className="text-sm font-semibold text-[var(--lp-accent)]">
                ← All journeys
              </Link>
            }
          />
        </Reveal>

        {error ? (
          <p className="text-[var(--lp-danger)]" role="alert">
            {error}
          </p>
        ) : null}
        {message ? <p className="text-[var(--lp-success)]">{message}</p> : null}

        {journey?.status === "draft" ? (
          <Reveal delay={1}>
            <Surface>
              <h2 className="text-lg font-semibold">Add step</h2>
              <p className="mt-1 text-sm text-[var(--lp-ink-muted)]">
                Document, quiz, task, or approval.
              </p>
              <form onSubmit={onAddStep} className="mt-4 grid gap-3 md:grid-cols-2">
                <select className="lp-input" name="stepType" defaultValue="document" required>
                  <option value="document">Document</option>
                  <option value="quiz">Quiz</option>
                  <option value="task">Task</option>
                  <option value="approval">Approval</option>
                </select>
                <input className="lp-input" name="title" placeholder="Step title" required />
                <input
                  className="lp-input md:col-span-2"
                  name="instructions"
                  placeholder="Instructions"
                />
                <input
                  className="lp-input"
                  name="dueOffsetDays"
                  type="number"
                  min={0}
                  defaultValue={0}
                  placeholder="Due offset days"
                />
                <button
                  type="submit"
                  disabled={pending}
                  className="rounded-[var(--lp-radius)] bg-[var(--lp-accent)] px-4 py-2.5 text-sm font-semibold text-white disabled:opacity-60"
                >
                  Add step
                </button>
              </form>
              <button
                type="button"
                disabled={pending || steps.length === 0}
                onClick={onPublish}
                className="mt-4 rounded-[var(--lp-radius)] border border-[var(--lp-border)] px-4 py-2.5 text-sm font-semibold disabled:opacity-60"
              >
                Publish journey
              </button>
            </Surface>
          </Reveal>
        ) : null}

        <Reveal delay={2}>
          <Surface className="overflow-hidden p-0">
            <div className="border-b border-[var(--lp-border)] px-5 py-4">
              <h2 className="text-lg font-semibold">Steps</h2>
              <p className="text-sm text-[var(--lp-ink-muted)]">{steps.length} steps</p>
            </div>
            {steps.length === 0 ? (
              <div className="p-5">
                <EmptyState dense title="No steps yet" description="Add at least one step before publishing." />
              </div>
            ) : (
              <ol className="divide-y divide-[var(--lp-border)]">
                {steps.map((step) => (
                  <li key={step.id} className="px-5 py-4">
                    <p className="font-medium">
                      {step.position}. {step.title}
                    </p>
                    <p className="text-sm text-[var(--lp-ink-muted)]">
                      {step.stepType}
                      {step.instructions ? ` · ${step.instructions}` : ""}
                    </p>
                  </li>
                ))}
              </ol>
            )}
          </Surface>
        </Reveal>
      </div>
    </AdminShell>
  );
}
