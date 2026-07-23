"use client";

import { useEffect, useState, useTransition } from "react";
import { useRouter } from "next/navigation";
import type { Notification } from "@launchpad/api-client";
import { ApiError } from "@launchpad/api-client";
import { Button, EmptyState, PageHeader, Reveal, Surface, cn } from "@launchpad/ui";
import { EmployeeShell } from "@/components/employee-shell";
import { getClient } from "@/lib/api";
import { clearSession, getAccessToken } from "@/lib/session";

export default function NotificationsPage() {
  const router = useRouter();
  const [pending, startTransition] = useTransition();
  const [marking, setMarking] = useState<string | null>(null);
  const [items, setItems] = useState<Notification[]>([]);
  const [error, setError] = useState<string | null>(null);

  function reload() {
    startTransition(() => {
      void (async () => {
        try {
          setItems(await getClient().listNotifications());
          setError(null);
        } catch (err) {
          if (err instanceof ApiError && err.status === 401) {
            clearSession();
            router.replace("/login");
            return;
          }
          setError(err instanceof ApiError ? err.message : "Unable to load notifications");
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
    // eslint-disable-next-line react-hooks/exhaustive-deps -- reload on route entry
  }, [router]);

  function markRead(notificationId: string) {
    setMarking(notificationId);
    startTransition(() => {
      void (async () => {
        try {
          await getClient().markNotificationRead(notificationId);
          reload();
        } catch (err) {
          if (err instanceof ApiError && err.status === 401) {
            clearSession();
            router.replace("/login");
            return;
          }
          setError(err instanceof ApiError ? err.message : "Unable to mark notification read");
        } finally {
          setMarking(null);
        }
      })();
    });
  }

  const unreadCount = items.filter((item) => !item.readAt).length;

  return (
    <EmployeeShell>
      <div className="space-y-8">
        <Reveal>
          <Surface className="overflow-hidden">
            <PageHeader
              eyebrow="Inbox"
              title="Notifications"
              description={
                pending && items.length === 0
                  ? "Loading updates…"
                  : unreadCount > 0
                    ? `${unreadCount} unread · journey assignments and approvals`
                    : "Journey assignments and approval decisions land here."
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
          <Surface>
            {items.length === 0 && !pending ? (
              <EmptyState
                title="No notifications yet"
                description="When a manager assigns a journey or decides an approval, you will see it here."
              />
            ) : (
              <ul className="divide-y divide-[var(--lp-border)]">
                {items.map((item) => {
                  const unread = !item.readAt;

                  return (
                    <li
                      key={item.id}
                      className={cn(
                        "flex flex-wrap items-start justify-between gap-4 py-4",
                        unread && "bg-[var(--lp-accent)]/[0.04]",
                      )}
                    >
                      <div className="min-w-0 flex-1">
                        <p className="font-semibold">{item.title}</p>
                        <p className="mt-1 text-sm text-[var(--lp-ink-muted)]">{item.body}</p>
                        <time className="mt-2 block text-xs text-[var(--lp-ink-muted)]">
                          {new Date(item.createdAt).toLocaleString()}
                          {item.readAt ? " · Read" : " · Unread"}
                        </time>
                      </div>
                      {unread ? (
                        <Button
                          type="button"
                          disabled={marking === item.id}
                          onClick={() => {
                            markRead(item.id);
                          }}
                        >
                          {marking === item.id ? "Saving…" : "Mark read"}
                        </Button>
                      ) : null}
                    </li>
                  );
                })}
              </ul>
            )}
          </Surface>
        </Reveal>
      </div>
    </EmployeeShell>
  );
}
