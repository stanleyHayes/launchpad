"use client";

import { useEffect, useState, useTransition } from "react";
import { useRouter } from "next/navigation";
import type { SupportTicket } from "@launchpad/api-client";
import { ApiError } from "@launchpad/api-client";
import { EmptyState, PageHeader, Reveal, Surface } from "@launchpad/ui";
import { PlatformShell } from "@/components/platform-shell";
import { getClient } from "@/lib/api";
import { clearSession, getAccessToken } from "@/lib/session";

const STATUS_ACTIONS = [
  { status: "open", label: "Open" },
  { status: "in_progress", label: "In progress" },
  { status: "resolved", label: "Resolved" },
  { status: "closed", label: "Closed" },
] as const;

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
          setTickets(await getClient().listPlatformSupportTickets());
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

  function updateStatus(ticketId: string, status: string) {
    setError(null);
    setMessage(null);
    startTransition(() => {
      void (async () => {
        try {
          await getClient().updatePlatformSupportTicketStatus(ticketId, { status });
          setMessage(`Ticket marked ${status.replace("_", " ")}`);
          reload();
        } catch (err) {
          setError(err instanceof ApiError ? err.message : "Unable to update ticket status");
        }
      })();
    });
  }

  return (
    <PlatformShell>
      <div className="space-y-8">
        <Reveal>
          <PageHeader
            eyebrow="Business"
            title="Support"
            description="Review customer tickets and move them through the support workflow."
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
              <h2 className="text-lg font-semibold">All tickets</h2>
              <p className="text-sm text-[var(--lp-ink-muted)]">
                {pending && tickets.length === 0 ? "Loading…" : `${tickets.length} tickets`}
              </p>
            </div>
            {tickets.length === 0 ? (
              <div className="p-5">
                <EmptyState
                  dense
                  title="No support tickets"
                  description="Customer requests will appear here."
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
                          Org {ticket.organizationId} · {ticket.priority} · {ticket.status}
                        </p>
                        <p className="mt-2 text-sm text-[var(--lp-ink-muted)]">{ticket.body}</p>
                        <time className="mt-1 block text-xs text-[var(--lp-ink-muted)]">
                          {new Date(ticket.createdAt).toLocaleString()}
                        </time>
                      </div>
                      <div className="flex flex-wrap gap-2">
                        {STATUS_ACTIONS.map((action) => (
                          <button
                            key={action.status}
                            type="button"
                            disabled={pending || ticket.status === action.status}
                            onClick={() => {
                              updateStatus(ticket.id, action.status);
                            }}
                            className="rounded-[var(--lp-radius)] border border-[var(--lp-border)] px-3 py-2 text-sm font-semibold disabled:opacity-60"
                          >
                            {action.label}
                          </button>
                        ))}
                      </div>
                    </div>
                  </li>
                ))}
              </ul>
            )}
          </Surface>
        </Reveal>
      </div>
    </PlatformShell>
  );
}
