"use client";

import { useEffect, useState, useTransition } from "react";
import { useRouter } from "next/navigation";
import type { Employee, OrgRequest, OrgRequestStatus } from "@launchpad/api-client";
import { ApiError } from "@launchpad/api-client";
import { Select, EmptyState, PageHeader, Reveal, Surface } from "@launchpad/ui";
import { getClient } from "@/lib/api";
import { clearSession, getAccessToken } from "@/lib/session";

const itemLabels: Record<string, string> = {
  laptop: "Laptop",
  mobile: "Mobile device",
  badge: "Access badge",
  desk_equipment: "Desk equipment",
  vpn: "VPN",
  email: "Email",
  software: "Software",
  github_repo: "GitHub repository",
  jira_project: "Jira project",
  other: "Other",
};

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

const statusFilters: { value: "" | OrgRequestStatus; label: string }[] = [
  { value: "", label: "All statuses" },
  { value: "pending", label: "Pending" },
  { value: "approved", label: "Approved" },
  { value: "fulfilled", label: "Fulfilled" },
  { value: "rejected", label: "Rejected" },
  { value: "cancelled", label: "Cancelled" },
];

export default function RequestsPage() {
  const router = useRouter();
  const [pending, startTransition] = useTransition();
  const [requests, setRequests] = useState<OrgRequest[]>([]);
  const [employees, setEmployees] = useState<Record<string, string>>({});
  const [statusFilter, setStatusFilter] = useState<"" | OrgRequestStatus>("");
  const [loaded, setLoaded] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [message, setMessage] = useState<string | null>(null);

  function reload(isStale?: () => boolean, status: "" | OrgRequestStatus = statusFilter) {
    startTransition(() => {
      void (async () => {
        try {
          const client = getClient();
          const [items, employeeItems] = await Promise.all([
            client.listOrgRequests(status || undefined),
            client.listEmployees(200),
          ]);
          if (isStale?.()) return;
          setRequests(items);
          setEmployees(
            Object.fromEntries(
              employeeItems.map((employee: Employee) => [
                employee.id,
                `${employee.firstName} ${employee.lastName}`.trim(),
              ]),
            ),
          );
          setLoaded(true);
          setError(null);
        } catch (err) {
          if (isStale?.()) return;
          if (err instanceof ApiError && err.status === 401) {
            clearSession();
            router.replace("/login");
            return;
          }
          setError(err instanceof ApiError ? err.message : "Unable to load requests");
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

  function act(action: () => Promise<unknown>, confirmation: string) {
    setError(null);
    setMessage(null);
    startTransition(() => {
      void (async () => {
        try {
          await action();
          setMessage(confirmation);
          reload();
        } catch (err) {
          setError(err instanceof ApiError ? err.message : "Unable to update the request");
        }
      })();
    });
  }

  function decide(request: OrgRequest, approve: boolean) {
    const note = approve ? "Approved" : "Rejected";
    act(
      () => getClient().decideOrgRequest(request.id, { approve, note }),
      approve ? "Request approved" : "Request rejected",
    );
  }

  function fulfill(request: OrgRequest) {
    act(() => getClient().fulfillOrgRequest(request.id), "Request marked fulfilled");
  }

  function onFilterChange(value: string) {
    const status = value as "" | OrgRequestStatus;
    setStatusFilter(status);
    reload(undefined, status);
  }

  const queue = requests.filter((request) => request.status === "pending" || request.status === "approved");

  return (
    <div className="space-y-8">
      <Reveal>
        <PageHeader
          eyebrow="People ops"
          title="Equipment & access requests"
          description="Approve or reject employee requests and mark approved ones as provisioned."
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
            <h2 className="text-lg font-semibold">Action needed</h2>
            <p className="text-sm text-[var(--lp-ink-muted)]">
              {pending && !loaded ? "Loading…" : `${queue.length} requests awaiting a decision or fulfillment`}
            </p>
          </div>
          {loaded && queue.length === 0 ? (
            <div className="p-5">
              <EmptyState
                dense
                title="Queue is clear"
                description="Pending and approved requests will appear here."
              />
            </div>
          ) : (
            <ul className="divide-y divide-[var(--lp-border)]">
              {queue.map((request) => (
                <li key={request.id} className="px-5 py-4">
                  <div className="flex flex-wrap items-start justify-between gap-3">
                    <div>
                      <p className="font-medium">
                        {itemLabels[request.item] ?? request.item}
                        <span className="ml-2 text-sm font-normal text-[var(--lp-ink-muted)]">
                          {request.kind} · {employees[request.requesterEmployeeId] ?? request.requesterEmployeeId}
                        </span>
                      </p>
                      {request.details ? (
                        <p className="whitespace-pre-wrap text-sm text-[var(--lp-ink-muted)]">
                          {request.details}
                        </p>
                      ) : null}
                      <p className="mt-1 text-xs text-[var(--lp-ink-muted)]">
                        Requested {new Date(request.createdAt).toLocaleString()}
                      </p>
                    </div>
                    <div className="flex items-center gap-2">
                      <span className={statusBadgeClass[request.status] ?? statusBadgeClass.pending}>
                        {request.status}
                      </span>
                      {request.status === "pending" ? (
                        <>
                          <button
                            type="button"
                            disabled={pending}
                            onClick={() => {
                              decide(request, true);
                            }}
                            className="rounded-[var(--lp-radius)] bg-[var(--lp-accent)] px-3 py-1.5 text-sm font-semibold text-white disabled:opacity-60"
                          >
                            Approve
                          </button>
                          <button
                            type="button"
                            disabled={pending}
                            onClick={() => {
                              decide(request, false);
                            }}
                            className="rounded-[var(--lp-radius)] border border-[var(--lp-border)] px-3 py-1.5 text-sm font-semibold text-[var(--lp-danger)] disabled:opacity-60"
                          >
                            Reject
                          </button>
                        </>
                      ) : null}
                      {request.status === "approved" ? (
                        <button
                          type="button"
                          disabled={pending}
                          onClick={() => {
                            fulfill(request);
                          }}
                          className="rounded-[var(--lp-radius)] bg-[var(--lp-success)] px-3 py-1.5 text-sm font-semibold text-white disabled:opacity-60"
                        >
                          Mark fulfilled
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

      <Reveal delay={2}>
        <Surface className="overflow-hidden p-0">
          <div className="flex flex-wrap items-center justify-between gap-3 border-b border-[var(--lp-border)] px-5 py-4">
            <div>
              <h2 className="text-lg font-semibold">All requests</h2>
              <p className="text-sm text-[var(--lp-ink-muted)]">{requests.length} requests</p>
            </div>
            <Select
              className="lp-input w-auto"
              value={statusFilter}
              onChange={(event) => {
                onFilterChange(event.target.value);
              }}
            >
              {statusFilters.map((filter) => (
                <option key={filter.value} value={filter.value}>
                  {filter.label}
                </option>
              ))}
            </Select>
          </div>
          {loaded && requests.length === 0 ? (
            <div className="p-5">
              <EmptyState dense title="No requests" description="No requests match this filter." />
            </div>
          ) : (
            <ul className="divide-y divide-[var(--lp-border)]">
              {requests.map((request) => (
                <li key={request.id} className="flex flex-wrap items-center justify-between gap-3 px-5 py-4">
                  <div>
                    <p className="font-medium">
                      {itemLabels[request.item] ?? request.item}
                      <span className="ml-2 text-sm font-normal text-[var(--lp-ink-muted)]">
                        {request.kind} · {employees[request.requesterEmployeeId] ?? request.requesterEmployeeId}
                      </span>
                    </p>
                    <p className="mt-1 text-xs text-[var(--lp-ink-muted)]">
                      Requested {new Date(request.createdAt).toLocaleString()}
                      {request.decisionNote ? ` · Note: ${request.decisionNote}` : ""}
                    </p>
                  </div>
                  <span className={statusBadgeClass[request.status] ?? statusBadgeClass.pending}>
                    {request.status}
                  </span>
                </li>
              ))}
            </ul>
          )}
        </Surface>
      </Reveal>
    </div>
  );
}
