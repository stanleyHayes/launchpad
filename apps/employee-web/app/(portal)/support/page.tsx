"use client";

import { useEffect, useState, useTransition, type SyntheticEvent } from "react";
import { useRouter } from "next/navigation";
import type { SupportTicket } from "@launchpad/api-client";
import { ApiError } from "@launchpad/api-client";
import { Select, EmptyState, PageHeader, Reveal, Surface } from "@launchpad/ui";
import { getClient } from "@/lib/api";
import { clearSession, getAccessToken } from "@/lib/session";

const categoryLabels: Record<string, string> = {
  hr: "HR & benefits",
  it: "IT & equipment",
  manager: "Manager",
  other: "Other",
};

const statusBadgeClass: Record<string, string> = {
  open: "rounded-full bg-[var(--lp-accent)]/10 px-3 py-1 text-xs font-semibold text-[var(--lp-accent)]",
  in_progress: "rounded-full bg-[var(--lp-accent)]/10 px-3 py-1 text-xs font-semibold text-[var(--lp-accent)]",
  waiting: "rounded-full px-3 py-1 text-xs font-semibold text-[var(--lp-ink-muted)]",
  resolved: "rounded-full bg-[var(--lp-success)]/10 px-3 py-1 text-xs font-semibold text-[var(--lp-success)]",
  closed: "rounded-full px-3 py-1 text-xs font-semibold text-[var(--lp-ink-muted)]",
};

function formString(form: FormData, key: string): string {
  const value = form.get(key);
  return typeof value === "string" ? value.trim() : "";
}

export default function SupportPage() {
  const router = useRouter();
  const [pending, startTransition] = useTransition();
  const [tickets, setTickets] = useState<SupportTicket[]>([]);
  const [loaded, setLoaded] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [message, setMessage] = useState<string | null>(null);
  const [openId, setOpenId] = useState<string | null>(null);

  function reload(isStale?: () => boolean) {
    startTransition(() => {
      void (async () => {
        try {
          const client = getClient();
          // The org list is org-wide; employees only see their own tickets here.
          const [profile, items] = await Promise.all([client.me(), client.listSupportTickets()]);
          if (isStale?.()) return;
          setTickets(items.filter((ticket) => ticket.createdByUserId === profile.user.id));
          setLoaded(true);
          setError(null);
        } catch (err) {
          if (isStale?.()) return;
          if (err instanceof ApiError && err.status === 401) {
            clearSession();
            router.replace("/login");
            return;
          }
          setError(err instanceof ApiError ? err.message : "Unable to load your tickets");
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

    startTransition(() => {
      void (async () => {
        try {
          await getClient().createSupportTicket({
            subject: formString(form, "subject"),
            body: formString(form, "body"),
            category: (formString(form, "category") || "other") as "hr" | "it" | "manager" | "other",
            priority: formString(form, "priority") || undefined,
          });
          formEl.reset();
          setMessage("Ticket submitted — your team will follow up");
          reload();
        } catch (err) {
          if (err instanceof ApiError && err.status === 401) {
            clearSession();
            router.replace("/login");
            return;
          }
          setError(err instanceof ApiError ? err.message : "Unable to submit your ticket");
        }
      })();
    });
  }

  return (
    <div className="space-y-8">
      <Reveal>
        <PageHeader
          eyebrow="Help"
          title="Support"
          description="Raise a ticket with your HR or IT team and track its status."
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
          <h2 className="text-lg font-semibold">New ticket</h2>
          <form className="mt-4 grid gap-3" onSubmit={onCreate}>
            <input className="lp-input" name="subject" placeholder="Subject" required />
            <div className="grid gap-3 sm:grid-cols-2">
              <Select className="lp-input" name="category" defaultValue="other">
                <option value="hr">HR & benefits</option>
                <option value="it">IT & equipment</option>
                <option value="manager">Manager</option>
                <option value="other">Other</option>
              </Select>
              <Select className="lp-input" name="priority" defaultValue="normal">
                <option value="low">Low priority</option>
                <option value="normal">Normal priority</option>
                <option value="high">High priority</option>
                <option value="urgent">Urgent</option>
              </Select>
            </div>
            <textarea
              className="lp-input min-h-24 resize-y"
              name="body"
              placeholder="Describe what you need help with"
              required
            />
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
            <h2 className="text-lg font-semibold">My tickets</h2>
            <p className="text-sm text-[var(--lp-ink-muted)]">
              {pending && !loaded ? "Loading…" : `${tickets.length} tickets`}
            </p>
          </div>
          {loaded && tickets.length === 0 ? (
            <div className="p-5">
              <EmptyState
                dense
                title="No tickets yet"
                description="Submit a ticket when something blocks your onboarding."
              />
            </div>
          ) : (
            <ul className="divide-y divide-[var(--lp-border)]">
              {tickets.map((ticket) => (
                <li key={ticket.id} className="px-5 py-4">
                  <button
                    type="button"
                    className="flex w-full flex-wrap items-start justify-between gap-3 text-left"
                    onClick={() => {
                      setOpenId(openId === ticket.id ? null : ticket.id);
                    }}
                  >
                    <div>
                      <p className="font-medium">{ticket.subject}</p>
                      <p className="text-sm text-[var(--lp-ink-muted)]">
                        {ticket.category ? `${categoryLabels[ticket.category] ?? ticket.category} · ` : ""}
                        {ticket.priority} priority
                      </p>
                    </div>
                    <span className={statusBadgeClass[ticket.status] ?? statusBadgeClass.waiting}>
                      {ticket.status.replace("_", " ")}
                    </span>
                  </button>
                  {openId === ticket.id ? (
                    <div className="mt-3 border-t border-[var(--lp-border)] pt-3">
                      <p className="whitespace-pre-wrap text-sm text-[var(--lp-ink-muted)]">
                        {ticket.body}
                      </p>
                      <p className="mt-2 text-xs text-[var(--lp-ink-muted)]">
                        Opened {new Date(ticket.createdAt).toLocaleString()} · Last update{" "}
                        {new Date(ticket.updatedAt).toLocaleString()}
                      </p>
                    </div>
                  ) : null}
                </li>
              ))}
            </ul>
          )}
        </Surface>
      </Reveal>
    </div>
  );
}
