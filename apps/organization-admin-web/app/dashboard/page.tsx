"use client";

import Link from "next/link";
import { useEffect, useState, useTransition } from "react";
import { useRouter } from "next/navigation";
import type { AuditEvent, MeResponse, Notification } from "@launchpad/api-client";
import { ApiError } from "@launchpad/api-client";
import {
  EmptyState,
  MetricCard,
  PageHeader,
  Reveal,
  Surface,
} from "@launchpad/ui";
import { AdminShell } from "@/components/admin-shell";
import { getClient } from "@/lib/api";
import { clearSession, getAccessToken } from "@/lib/session";

export default function DashboardPage() {
  const router = useRouter();
  const [pending, startTransition] = useTransition();
  const [me, setMe] = useState<MeResponse | null>(null);
  const [events, setEvents] = useState<AuditEvent[]>([]);
  const [notifications, setNotifications] = useState<Notification[]>([]);
  const [employeeCount, setEmployeeCount] = useState<number | null>(null);
  const [journeyCount, setJourneyCount] = useState<number | null>(null);
  const [approvalCount, setApprovalCount] = useState<number | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!getAccessToken()) {
      router.replace("/login");
      return;
    }

    startTransition(() => {
      void (async () => {
        try {
          const client = getClient();
          const [profile, auditEvents, noticeItems, employees, journeys, approvals] =
            await Promise.all([
              client.me(),
              client.listAuditEvents(8),
              client.listNotifications(),
              client.listEmployees(),
              client.listJourneys(),
              client.listApprovals(),
            ]);
          setMe(profile);
          setEvents(auditEvents);
          setNotifications(noticeItems);
          setEmployeeCount(employees.length);
          setJourneyCount(journeys.length);
          setApprovalCount(approvals.filter((item) => item.status === "pending").length);
        } catch (err) {
          if (err instanceof ApiError && err.status === 401) {
            clearSession();
            router.replace("/login");
            return;
          }
          setError(err instanceof ApiError ? err.message : "Unable to load dashboard");
        }
      })();
    });
  }, [router]);

  return (
    <AdminShell>
      <div className="space-y-8">
        <Reveal>
          <Surface className="overflow-hidden">
            <PageHeader
              eyebrow="Onboarding command centre"
              title={me ? `Welcome, ${me.user.displayName}` : "Welcome"}
              description={
                me?.organization
                  ? `${me.organization.name} is ready for journeys, assignments, and approvals.`
                  : "Loading organization context…"
              }
              actions={
                <Link
                  href="/journeys"
                  className="rounded-[var(--lp-radius)] bg-[var(--lp-accent)] px-4 py-2.5 text-sm font-semibold text-white"
                >
                  Build a journey
                </Link>
              }
            />
          </Surface>
        </Reveal>

        {error ? (
          <p className="text-[var(--lp-danger)]" role="alert">
            {error}
          </p>
        ) : null}

        <Reveal delay={1}>
          <section className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
            <MetricCard
              label="Organization status"
              value={me?.organization?.status ?? (pending ? "…" : "—")}
            />
            <MetricCard label="Employees" value={employeeCount ?? (pending ? "…" : "—")} />
            <MetricCard label="Journeys" value={journeyCount ?? (pending ? "…" : "—")} />
            <MetricCard
              label="Pending approvals"
              value={approvalCount ?? (pending ? "…" : "—")}
              hint="Needs manager decision"
            />
          </section>
        </Reveal>

        <Reveal delay={2}>
          <section className="grid gap-6 lg:grid-cols-[2fr_3fr]">
            <Surface>
              <h2
                className="text-xl font-semibold"
                style={{ fontFamily: "var(--lp-font-display)" }}
              >
                Notifications
              </h2>
              {notifications.length === 0 ? (
                <div className="mt-4">
                  <EmptyState
                    dense
                    title="No notifications"
                    description="Updates about approvals, assignments, and team activity will appear here."
                  />
                </div>
              ) : (
                <ul className="mt-4 divide-y divide-[var(--lp-border)]">
                  {notifications.slice(0, 6).map((notification) => (
                    <li key={notification.id} className="py-3">
                      <p className="font-medium">{notification.title}</p>
                      <p className="text-sm text-[var(--lp-ink-muted)]">{notification.body}</p>
                      <time className="mt-1 block text-xs text-[var(--lp-ink-muted)]">
                        {new Date(notification.createdAt).toLocaleString()}
                      </time>
                    </li>
                  ))}
                </ul>
              )}
            </Surface>

            <Surface>
              <h2
                className="text-xl font-semibold"
                style={{ fontFamily: "var(--lp-font-display)" }}
              >
                Quick links
              </h2>
              <ul className="mt-4 divide-y divide-[var(--lp-border)]">
                {[
                  { href: "/employees", label: "Employees", detail: "Invite, provision, assign" },
                  { href: "/journeys", label: "Journeys", detail: "Draft, step, publish" },
                  { href: "/approvals", label: "Approvals", detail: "Review pending steps" },
                ].map((link) => (
                  <li key={link.href}>
                    <Link
                      href={link.href}
                      className="flex items-center justify-between gap-3 py-3 transition hover:text-[var(--lp-accent)]"
                    >
                      <span>
                        <span className="block font-medium">{link.label}</span>
                        <span className="text-sm text-[var(--lp-ink-muted)]">{link.detail}</span>
                      </span>
                      <span aria-hidden="true">→</span>
                    </Link>
                  </li>
                ))}
              </ul>
            </Surface>
          </section>
        </Reveal>

        <Reveal delay={3}>
          <section>
            <Surface>
              <h2
                className="text-xl font-semibold"
                style={{ fontFamily: "var(--lp-font-display)" }}
              >
                Recent audit activity
              </h2>
              {events.length === 0 ? (
                <div className="mt-4">
                  <EmptyState
                    dense
                    title="No audit events yet"
                    description="Privileged actions for this organization will appear here."
                  />
                </div>
              ) : (
                <ul className="mt-4 divide-y divide-[var(--lp-border)]">
                  {events.map((event) => (
                    <li
                      key={event.id}
                      className="flex flex-wrap items-center justify-between gap-2 py-3"
                    >
                      <div>
                        <p className="font-medium">{event.action}</p>
                        <p className="text-sm text-[var(--lp-ink-muted)]">
                          {event.resourceType} · {event.resourceId}
                        </p>
                      </div>
                      <time className="text-sm text-[var(--lp-ink-muted)]">
                        {new Date(event.createdAt).toLocaleString()}
                      </time>
                    </li>
                  ))}
                </ul>
              )}
            </Surface>
          </section>
        </Reveal>
      </div>
    </AdminShell>
  );
}
