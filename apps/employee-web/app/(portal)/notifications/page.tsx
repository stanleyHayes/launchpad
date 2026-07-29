"use client";

import { useEffect, useState, useTransition } from "react";
import { useRouter } from "next/navigation";
import type { Notification } from "@launchpad/api-client";
import { ApiError } from "@launchpad/api-client";
import { Button, EmptyState, PageHeader, Reveal, Surface, cn } from "@launchpad/ui";
import { getClient } from "@/lib/api";
import { clearSession, getAccessToken } from "@/lib/session";

export default function NotificationsPage() {
  const router = useRouter();
  const [pending, startTransition] = useTransition();
  const [marking, setMarking] = useState<string | null>(null);
  const [items, setItems] = useState<Notification[]>([]);
  const [error, setError] = useState<string | null>(null);

  function reload(isStale?: () => boolean) {
    startTransition(() => {
      void (async () => {
        try {
          const notificationItems = await getClient().listNotifications();
          if (isStale?.()) return;
          setItems(notificationItems);
          setError(null);
        } catch (err) {
          if (isStale?.()) return;
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
    let stale = false;
    reload(() => stale);
    return () => {
      stale = true;
    };
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

  // Opening a linked notification marks it read, then navigates to its target.
  function openNotification(item: Notification) {
    if (!item.link) return;
    if (!item.readAt) {
      void getClient()
        .markNotificationRead(item.id)
        .catch(() => undefined);
    }
    router.push(item.link);
  }

  const unreadCount = items.filter((item) => !item.readAt).length;

  return (
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
                        item.link && "cursor-pointer",
                      )}
                      onClick={() => {
                        openNotification(item);
                      }}
                    >
                      <div className="min-w-0 flex-1">
                        <p className="flex flex-wrap items-center gap-2 font-semibold">
                          {item.title}
                          <span className="rounded-full bg-[var(--lp-accent)]/10 px-2 py-0.5 text-xs font-medium capitalize text-[var(--lp-accent)]">
                            {(item.type || "system").replace(/_/g, " ")}
                          </span>
                        </p>
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
                          onClick={(event) => {
                            event.stopPropagation();
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
      );
}
