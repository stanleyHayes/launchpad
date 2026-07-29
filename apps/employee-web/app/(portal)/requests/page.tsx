"use client";

import { useEffect, useState, useTransition, type SyntheticEvent } from "react";
import { useRouter } from "next/navigation";
import type { OrgRequest } from "@launchpad/api-client";
import { ApiError } from "@launchpad/api-client";
import { EmptyState, PageHeader, Reveal, Surface } from "@launchpad/ui";
import { getClient } from "@/lib/api";
import { clearSession, getAccessToken } from "@/lib/session";

const equipmentItems: { value: string; label: string }[] = [
  { value: "laptop", label: "Laptop" },
  { value: "mobile", label: "Mobile device" },
  { value: "badge", label: "Access badge" },
  { value: "desk_equipment", label: "Desk equipment" },
  { value: "other", label: "Other" },
];

const accessItems: { value: string; label: string }[] = [
  { value: "vpn", label: "VPN" },
  { value: "email", label: "Email" },
  { value: "software", label: "Software" },
  { value: "github_repo", label: "GitHub repository" },
  { value: "jira_project", label: "Jira project" },
  { value: "other", label: "Other" },
];

const itemLabels: Record<string, string> = Object.fromEntries(
  [...equipmentItems, ...accessItems].map((item) => [item.value, item.label]),
);

const statusBadgeClass: Record<string, string> = {
  pending:
    "rounded-full bg-[var(--lp-accent)]/10 px-3 py-1 text-xs font-semibold text-[var(--lp-accent)]",
  approved:
    "rounded-full bg-[var(--lp-success)]/10 px-3 py-1 text-xs font-semibold text-[var(--lp-success)]",
  fulfilled:
    "rounded-full bg-[var(--lp-success)]/10 px-3 py-1 text-xs font-semibold text-[var(--lp-success)]",
  rejected: "rounded-full px-3 py-1 text-xs font-semibold text-[var(--lp-danger)]",
  cancelled: "rounded-full px-3 py-1 text-xs font-semibold text-[var(--lp-ink-muted)]",
};

function formString(form: FormData, key: string): string {
  const value = form.get(key);
  return typeof value === "string" ? value.trim() : "";
}

export default function RequestsPage() {
  const router = useRouter();
  const [pending, startTransition] = useTransition();
  const [requests, setRequests] = useState<OrgRequest[]>([]);
  const [loaded, setLoaded] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [message, setMessage] = useState<string | null>(null);
  const [kind, setKind] = useState<"equipment" | "access">("equipment");

  function reload(isStale?: () => boolean) {
    startTransition(() => {
      void (async () => {
        try {
          const items = await getClient().listMyRequests();
          if (isStale?.()) return;
          setRequests(items);
          setLoaded(true);
          setError(null);
        } catch (err) {
          if (isStale?.()) return;
          if (err instanceof ApiError && err.status === 401) {
            clearSession();
            router.replace("/login");
            return;
          }
          setError(err instanceof ApiError ? err.message : "Unable to load your requests");
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
          await getClient().createMyRequest({
            kind,
            item: formString(form, "item"),
            details: formString(form, "details") || undefined,
          });
          formEl.reset();
          setMessage("Request submitted — your team will review it");
          reload();
        } catch (err) {
          if (err instanceof ApiError && err.status === 401) {
            clearSession();
            router.replace("/login");
            return;
          }
          setError(err instanceof ApiError ? err.message : "Unable to submit your request");
        }
      })();
    });
  }

  function onCancel(requestId: string) {
    setError(null);
    setMessage(null);
    startTransition(() => {
      void (async () => {
        try {
          await getClient().cancelMyRequest(requestId);
          setMessage("Request cancelled");
          reload();
        } catch (err) {
          setError(err instanceof ApiError ? err.message : "Unable to cancel the request");
        }
      })();
    });
  }

  const items = kind === "equipment" ? equipmentItems : accessItems;

  return (
    <div className="space-y-8">
      <Reveal>
        <PageHeader
          eyebrow="Self-service"
          title="Equipment & access"
          description="Request the hardware, accounts, and tools you need and track their status."
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
          <h2 className="text-lg font-semibold">New request</h2>
          <form className="mt-4 grid gap-3" onSubmit={onCreate}>
            <div className="grid gap-3 sm:grid-cols-2">
              <select
                className="lp-input"
                name="kind"
                value={kind}
                onChange={(event) => {
                  setKind(event.target.value as "equipment" | "access");
                }}
              >
                <option value="equipment">Equipment</option>
                <option value="access">Access</option>
              </select>
              <select className="lp-input" name="item" key={kind} defaultValue={items[0].value}>
                {items.map((item) => (
                  <option key={item.value} value={item.value}>
                    {item.label}
                  </option>
                ))}
              </select>
            </div>
            <textarea
              className="lp-input min-h-24 resize-y"
              name="details"
              placeholder="Add details (model, access level, project, …)"
            />
            <div>
              <button
                type="submit"
                disabled={pending}
                className="rounded-[var(--lp-radius)] bg-[var(--lp-accent)] px-4 py-2.5 text-sm font-semibold text-white disabled:opacity-60"
              >
                Submit request
              </button>
            </div>
          </form>
        </Surface>
      </Reveal>

      <Reveal delay={2}>
        <Surface className="overflow-hidden p-0">
          <div className="border-b border-[var(--lp-border)] px-5 py-4">
            <h2 className="text-lg font-semibold">My requests</h2>
            <p className="text-sm text-[var(--lp-ink-muted)]">
              {pending && !loaded ? "Loading…" : `${requests.length} requests`}
            </p>
          </div>
          {loaded && requests.length === 0 ? (
            <div className="p-5">
              <EmptyState
                dense
                title="No requests yet"
                description="Request equipment or access when you need it for your work."
              />
            </div>
          ) : (
            <ul className="divide-y divide-[var(--lp-border)]">
              {requests.map((request) => (
                <li key={request.id} className="px-5 py-4">
                  <div className="flex flex-wrap items-start justify-between gap-3">
                    <div>
                      <p className="font-medium">
                        {itemLabels[request.item] ?? request.item}
                        <span className="ml-2 text-sm font-normal text-[var(--lp-ink-muted)]">
                          {request.kind}
                        </span>
                      </p>
                      {request.details ? (
                        <p className="whitespace-pre-wrap text-sm text-[var(--lp-ink-muted)]">
                          {request.details}
                        </p>
                      ) : null}
                      <p className="mt-1 text-xs text-[var(--lp-ink-muted)]">
                        Requested {new Date(request.createdAt).toLocaleString()}
                        {request.decisionNote ? ` · Note: ${request.decisionNote}` : ""}
                      </p>
                    </div>
                    <div className="flex items-center gap-3">
                      <span className={statusBadgeClass[request.status] ?? statusBadgeClass.pending}>
                        {request.status}
                      </span>
                      {request.status === "pending" ? (
                        <button
                          type="button"
                          disabled={pending}
                          onClick={() => {
                            onCancel(request.id);
                          }}
                          className="text-sm font-semibold text-[var(--lp-danger)] disabled:opacity-60"
                        >
                          Cancel
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
