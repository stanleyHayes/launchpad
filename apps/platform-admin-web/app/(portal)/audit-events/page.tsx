"use client";

import { useEffect, useState, useTransition } from "react";
import { useRouter } from "next/navigation";
import type { AuditEvent } from "@launchpad/api-client";
import { ApiError } from "@launchpad/api-client";
import { Select, EmptyState, PageHeader, Reveal, Surface } from "@launchpad/ui";
import { getClient } from "@/lib/api";
import { clearSession, getAccessToken } from "@/lib/session";

const limitOptions = [20, 50, 100, 200];

export default function AuditEventsPage() {
  const router = useRouter();
  const [pending, startTransition] = useTransition();
  const [events, setEvents] = useState<AuditEvent[]>([]);
  const [limit, setLimit] = useState(50);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!getAccessToken()) {
      router.replace("/login");
      return;
    }

    let stale = false;
    startTransition(() => {
      void (async () => {
        try {
          const items = await getClient().listPlatformAuditEvents(limit);
          if (stale) return;
          setEvents(items);
        } catch (err) {
          if (stale) return;
          if (err instanceof ApiError && err.status === 401) {
            clearSession();
            router.replace("/login");
            return;
          }
          setError(err instanceof ApiError ? err.message : "Unable to load audit events");
        }
      })();
    });
    return () => {
      stale = true;
    };
  }, [router, limit]);

  return (
          <div className="space-y-8">
        <Reveal>
          <PageHeader
            eyebrow="Operations"
            title="Audit events"
            description="Recent audit activity across all organizations and platform actions."
          />
        </Reveal>

        {error ? (
          <p className="text-[var(--lp-danger)]" role="alert">
            {error}
          </p>
        ) : null}

        <Reveal delay={1}>
          <Surface className="overflow-hidden p-0">
            <div className="flex flex-wrap items-center justify-between gap-3 border-b border-[var(--lp-border)] px-5 py-4">
              <div>
                <h2 className="text-lg font-semibold">Recent events</h2>
                <p className="text-sm text-[var(--lp-ink-muted)]">
                  {pending ? "Loading…" : `${events.length} events`}
                </p>
              </div>
              <label className="flex items-center gap-2 text-sm text-[var(--lp-ink-muted)]">
                Show
                <Select
                  className="lp-input"
                  value={limit}
                  onChange={(event) => {
                    setLimit(Number(event.target.value));
                  }}
                >
                  {limitOptions.map((option) => (
                    <option key={option} value={option}>
                      {option}
                    </option>
                  ))}
                </Select>
              </label>
            </div>
            {events.length === 0 ? (
              <div className="p-5">
                <EmptyState
                  dense
                  title="No audit events yet"
                  description="Events appear here as organizations and platform staff act."
                />
              </div>
            ) : (
              <div className="overflow-x-auto">
                <table className="w-full text-left text-sm">
                  <thead>
                    <tr className="border-b border-[var(--lp-border)] text-xs uppercase tracking-wide text-[var(--lp-ink-muted)]">
                      <th className="px-5 py-3 font-semibold">Time</th>
                      <th className="px-5 py-3 font-semibold">Organization</th>
                      <th className="px-5 py-3 font-semibold">Actor</th>
                      <th className="px-5 py-3 font-semibold">Action</th>
                      <th className="px-5 py-3 font-semibold">Resource</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-[var(--lp-border)]">
                    {events.map((event) => (
                      <tr key={event.id}>
                        <td className="whitespace-nowrap px-5 py-3 text-[var(--lp-ink-muted)]">
                          {new Date(event.createdAt).toLocaleString()}
                        </td>
                        <td className="px-5 py-3">{event.organizationId ?? "platform"}</td>
                        <td className="px-5 py-3">{event.actorUserId}</td>
                        <td className="px-5 py-3 font-medium">{event.action}</td>
                        <td className="px-5 py-3 text-[var(--lp-ink-muted)]">
                          {event.resourceType}
                          {event.resourceId ? ` · ${event.resourceId}` : ""}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </Surface>
        </Reveal>
      </div>
      );
}
