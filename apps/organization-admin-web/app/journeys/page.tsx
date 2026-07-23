"use client";

import Link from "next/link";
import { useEffect, useState, useTransition, type SyntheticEvent } from "react";
import { useRouter } from "next/navigation";
import type { JourneyTemplate } from "@launchpad/api-client";
import { ApiError } from "@launchpad/api-client";
import { EmptyState, PageHeader, Reveal, Surface } from "@launchpad/ui";
import { AdminShell } from "@/components/admin-shell";
import { getClient } from "@/lib/api";
import { clearSession, getAccessToken } from "@/lib/session";

function formString(form: FormData, key: string): string {
  const value = form.get(key);
  return typeof value === "string" ? value.trim() : "";
}

export default function JourneysPage() {
  const router = useRouter();
  const [pending, startTransition] = useTransition();
  const [journeys, setJourneys] = useState<JourneyTemplate[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [message, setMessage] = useState<string | null>(null);

  function reload() {
    startTransition(() => {
      void (async () => {
        try {
          setJourneys(await getClient().listJourneys());
        } catch (err) {
          if (err instanceof ApiError && err.status === 401) {
            clearSession();
            router.replace("/login");
            return;
          }
          setError(err instanceof ApiError ? err.message : "Unable to load journeys");
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
  }, [router]);

  function onCreate(event: SyntheticEvent<HTMLFormElement>) {
    event.preventDefault();
    setError(null);
    setMessage(null);

    const form = new FormData(event.currentTarget);
    startTransition(() => {
      void (async () => {
        try {
          const created = await getClient().createJourney({
            name: formString(form, "name"),
            description: formString(form, "description"),
          });
          event.currentTarget.reset();
          setMessage("Draft journey created");
          router.push(`/journeys/${created.id}`);
        } catch (err) {
          setError(err instanceof ApiError ? err.message : "Unable to create journey");
        }
      })();
    });
  }

  return (
    <AdminShell>
      <div className="space-y-8">
        <Reveal>
          <PageHeader
            eyebrow="Journeys"
            title="Journey builder"
            description="Draft steps, publish templates, then assign them to employees."
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
            <h2 className="text-lg font-semibold">New journey</h2>
            <p className="mt-1 text-sm text-[var(--lp-ink-muted)]">
              Starts as a draft. Add steps, then publish.
            </p>
            <form onSubmit={onCreate} className="mt-4 grid gap-3 md:grid-cols-2">
              <input className="lp-input" name="name" placeholder="Engineering onboarding" required />
              <input className="lp-input" name="description" placeholder="Short description" />
              <button
                type="submit"
                disabled={pending}
                className="rounded-[var(--lp-radius)] bg-[var(--lp-accent)] px-4 py-2.5 text-sm font-semibold text-white disabled:opacity-60 md:col-span-2"
              >
                Create draft
              </button>
            </form>
          </Surface>
        </Reveal>

        <Reveal delay={2}>
          <Surface className="p-0 overflow-hidden">
            <div className="border-b border-[var(--lp-border)] px-5 py-4">
              <h2 className="text-lg font-semibold">Templates</h2>
              <p className="text-sm text-[var(--lp-ink-muted)]">{journeys.length} journeys</p>
            </div>
            {journeys.length === 0 ? (
              <div className="p-5">
                <EmptyState
                  dense
                  title="No journeys yet"
                  description="Create a draft to start your first onboarding path."
                />
              </div>
            ) : (
              <ul className="divide-y divide-[var(--lp-border)]">
                {journeys.map((journey) => (
                  <li key={journey.id} className="flex flex-wrap items-center justify-between gap-2 px-5 py-4">
                    <div>
                      <Link
                        href={`/journeys/${journey.id}`}
                        className="font-medium text-[var(--lp-accent)]"
                      >
                        {journey.name}
                      </Link>
                      <p className="text-sm text-[var(--lp-ink-muted)]">
                        {journey.description || "—"}
                      </p>
                    </div>
                    <p className="text-sm text-[var(--lp-ink-muted)]">
                      {journey.status} · v{journey.currentVersion}
                    </p>
                  </li>
                ))}
              </ul>
            )}
          </Surface>
        </Reveal>
      </div>
    </AdminShell>
  );
}
