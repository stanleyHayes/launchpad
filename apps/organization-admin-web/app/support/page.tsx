"use client";

import { useEffect, useState, useTransition, type SyntheticEvent } from "react";
import { useRouter } from "next/navigation";
import type { SupportTicket } from "@launchpad/api-client";
import { ApiError } from "@launchpad/api-client";
import { EmptyState, PageHeader, Reveal, Surface } from "@launchpad/ui";
import { AdminShell } from "@/components/admin-shell";
import { getClient } from "@/lib/api";
import { clearSession, getAccessToken } from "@/lib/session";

function formString(form: FormData, key: string): string {
  const value = form.get(key);
  return typeof value === "string" ? value.trim() : "";
}

export default function SupportPage() {
  const router = useRouter();
  const [pending, startTransition] = useTransition();
  const [tickets, setTickets] = useState<SupportTicket[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [message, setMessage] = useState<string | null>(null);

  function reload() {
    startTransition(() => {
      void (async () => {
        try {
          setTickets(await getClient().listSupportTickets());
        } catch (err) {
          if (err instanceof ApiError && err.status === 401) {
            clearSession();
            router.replace("/login");
            return;
          }
          setError(err instanceof ApiError ? err.message : "Unable to load support tickets");
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

  function onCreateTicket(event: SyntheticEvent<HTMLFormElement>) {
    event.preventDefault();
    setError(null);
    setMessage(null);
    const form = new FormData(event.currentTarget);

    startTransition(() => {
      void (async () => {
        try {
          await getClient().createSupportTicket({
            subject: formString(form, "subject"),
            body: formString(form, "body"),
            priority: formString(form, "priority") || undefined,
          });
          event.currentTarget.reset();
          setMessage("Support ticket created");
          reload();
        } catch (err) {
          setError(err instanceof ApiError ? err.message : "Unable to create support ticket");
        }
      })();
    });
  }

  return (
    <AdminShell>
      <div className="space-y-8">
        <Reveal>
          <PageHeader
            eyebrow="Account"
            title="Support"
            description="Open tickets with the LaunchPad team and track responses."
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
            <h2 className="text-lg font-semibold">Create ticket</h2>
            <form className="mt-4 grid gap-3" onSubmit={onCreateTicket}>
              <input className="lp-input" name="subject" placeholder="Subject" required />
              <textarea
                className="lp-input min-h-24 resize-y"
                name="body"
                placeholder="Describe your issue"
                required
              />
              <select className="lp-input" name="priority" defaultValue="normal">
                <option value="low">Low priority</option>
                <option value="normal">Normal priority</option>
                <option value="high">High priority</option>
                <option value="urgent">Urgent</option>
              </select>
              <div>
                <button
                  type="submit"
                  disabled={pending}
                  className="rounded-[var(--lp-radius)] bg-[var(--lp-accent)] px-4 py-2.5 text-sm font-semibold text-white disabled:opacity-60"
                >
                  Submit ticket
                </button>
              </div>
            </form>
          </Surface>
        </Reveal>

        <Reveal delay={2}>
          <Surface className="overflow-hidden p-0">
            <div className="border-b border-[var(--lp-border)] px-5 py-4">
              <h2 className="text-lg font-semibold">Your tickets</h2>
              <p className="text-sm text-[var(--lp-ink-muted)]">
                {pending && tickets.length === 0 ? "Loading…" : `${tickets.length} tickets`}
              </p>
            </div>
            {tickets.length === 0 ? (
              <div className="p-5">
                <EmptyState
                  dense
                  title="No tickets yet"
                  description="Submit a ticket when you need help from the platform team."
                />
              </div>
            ) : (
              <ul className="divide-y divide-[var(--lp-border)]">
                {tickets.map((ticket) => (
                  <li key={ticket.id} className="px-5 py-4">
                    <div className="flex flex-wrap items-start justify-between gap-3">
                      <div>
                        <p className="font-medium">{ticket.subject}</p>
                        <p className="text-sm text-[var(--lp-ink-muted)]">
                          {ticket.priority} · {ticket.status.replace("_", " ")}
                        </p>
                        <p className="mt-2 text-sm text-[var(--lp-ink-muted)]">{ticket.body}</p>
                      </div>
                      <time className="text-sm text-[var(--lp-ink-muted)]">
                        {new Date(ticket.createdAt).toLocaleString()}
                      </time>
                    </div>
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
