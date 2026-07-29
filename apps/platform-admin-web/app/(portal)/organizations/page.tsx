"use client";

import { useEffect, useState, useTransition } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import type { Organization, SupportSessionCreated } from "@launchpad/api-client";
import { ApiError } from "@launchpad/api-client";
import { EmptyState, PageHeader, Reveal, Surface } from "@launchpad/ui";
import { getClient } from "@/lib/api";
import { clearSession, getAccessToken } from "@/lib/session";

const minReasonLength = 10;
const pageSize = 10;

export default function OrganizationsPage() {
  const router = useRouter();
  const [pending, startTransition] = useTransition();
  const [organizations, setOrganizations] = useState<Organization[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(0);
  const [error, setError] = useState<string | null>(null);
  const [message, setMessage] = useState<string | null>(null);
  const [search, setSearch] = useState("");
  const [statusFilter, setStatusFilter] = useState("");
  const [planFilter, setPlanFilter] = useState("");
  const [sessionFormOrgId, setSessionFormOrgId] = useState<string | null>(null);
  const [sessionReason, setSessionReason] = useState("");
  const [createdSession, setCreatedSession] = useState<SupportSessionCreated | null>(null);

  function reload(targetPage = page, isStale?: () => boolean) {
    startTransition(() => {
      void (async () => {
        try {
          const organizationPage = await getClient().listPlatformOrganizations({
            search: search.trim() || undefined,
            status: statusFilter || undefined,
            planCode: planFilter.trim() || undefined,
            offset: targetPage * pageSize,
            limit: pageSize,
          });
          if (isStale?.()) return;
          setOrganizations(organizationPage.items);
          setTotal(organizationPage.total);
          setPage(targetPage);
        } catch (err) {
          if (isStale?.()) return;
          if (err instanceof ApiError && err.status === 401) {
            clearSession();
            router.replace("/login");
            return;
          }
          setError(err instanceof ApiError ? err.message : "Unable to load organizations");
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
    reload(0, () => stale);
    return () => {
      stale = true;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps -- initial load only
  }, [router]);

  function updateStatus(organizationId: string, action: "suspend" | "activate") {
    setError(null);
    setMessage(null);
    startTransition(() => {
      void (async () => {
        try {
          const client = getClient();
          if (action === "suspend") {
            await client.suspendOrganization(organizationId);
            setMessage("Organization suspended");
          } else {
            await client.activateOrganization(organizationId);
            setMessage("Organization activated");
          }
          reload();
        } catch (err) {
          setError(
            err instanceof ApiError ? err.message : "Unable to update organization status",
          );
        }
      })();
    });
  }

  function closeOrganization(organization: Organization) {
    if (
      !window.confirm(
        `Close ${organization.name}? This blocks login and cannot be reversed from this screen.`,
      )
    ) {
      return;
    }

    setError(null);
    setMessage(null);
    startTransition(() => {
      void (async () => {
        try {
          await getClient().closeOrganization(organization.id);
          setMessage("Organization closed");
          reload();
        } catch (err) {
          setError(err instanceof ApiError ? err.message : "Unable to close organization");
        }
      })();
    });
  }

  function startSupportSession(organizationId: string) {
    setError(null);
    setMessage(null);
    startTransition(() => {
      void (async () => {
        try {
          const created = await getClient().startSupportSession(organizationId, sessionReason.trim());
          setCreatedSession(created);
          setSessionFormOrgId(null);
          setSessionReason("");
          setMessage("Support session started — copy the token now, it is shown only once");
        } catch (err) {
          setError(
            err instanceof ApiError ? err.message : "Unable to start the support session",
          );
        }
      })();
    });
  }

  function endSupportSession(sessionId: string) {
    setError(null);
    setMessage(null);
    startTransition(() => {
      void (async () => {
        try {
          await getClient().endSupportSession(sessionId);
          setCreatedSession(null);
          setMessage("Support session ended");
        } catch (err) {
          setError(
            err instanceof ApiError ? err.message : "Unable to end the support session",
          );
        }
      })();
    });
  }

  return (
          <div className="space-y-8">
        <Reveal>
          <PageHeader
            eyebrow="Operations"
            title="Organizations"
            description="Review customer tenants, plans, and lifecycle status."
          />
        </Reveal>

        {error ? (
          <p className="text-[var(--lp-danger)]" role="alert">
            {error}
          </p>
        ) : null}
        {message ? <p className="text-[var(--lp-success)]">{message}</p> : null}

        {createdSession ? (
          <Reveal>
            <Surface className="space-y-3 border-[var(--lp-warning)]">
              <h2 className="text-lg font-semibold">Support session active</h2>
              <p className="text-sm text-[var(--lp-ink-muted)]">
                Session {createdSession.session.id} · token expires at{" "}
                {new Date(createdSession.tokenExpiresAt).toLocaleString()} · session expires at{" "}
                {new Date(createdSession.session.expiresAt).toLocaleString()}
              </p>
              <p className="text-sm text-[var(--lp-ink-muted)]">
                Impersonation token (read-only, shown once):
              </p>
              <code className="block break-all rounded-[var(--lp-radius)] border border-[var(--lp-border)] bg-[var(--lp-paper)] p-3 text-xs">
                {createdSession.token}
              </code>
              <div className="flex flex-wrap gap-2">
                <button
                  type="button"
                  onClick={() => {
                    void navigator.clipboard.writeText(createdSession.token);
                  }}
                  className="rounded-[var(--lp-radius)] border border-[var(--lp-border)] px-3 py-2 text-sm font-semibold"
                >
                  Copy token
                </button>
                <button
                  type="button"
                  disabled={pending}
                  onClick={() => {
                    endSupportSession(createdSession.session.id);
                  }}
                  className="rounded-[var(--lp-radius)] border border-[var(--lp-danger)] px-3 py-2 text-sm font-semibold text-[var(--lp-danger)] disabled:opacity-60"
                >
                  End session
                </button>
              </div>
            </Surface>
          </Reveal>
        ) : null}

        <Reveal delay={1}>
          <Surface className="overflow-hidden p-0">
            <div className="border-b border-[var(--lp-border)] px-5 py-4">
              <h2 className="text-lg font-semibold">All organizations</h2>
              <p className="text-sm text-[var(--lp-ink-muted)]">
                {total} {total === 1 ? "tenant" : "tenants"}
              </p>
              <form
                className="mt-4 grid gap-3 md:grid-cols-[minmax(0,1fr)_12rem_12rem_auto]"
                onSubmit={(event) => {
                  event.preventDefault();
                  reload(0);
                }}
              >
                <label className="text-sm font-medium">
                  Search
                  <input
                    value={search}
                    onChange={(event) => setSearch(event.target.value)}
                    placeholder="Name or slug"
                    className="mt-1.5 block w-full rounded-[var(--lp-radius)] border border-[var(--lp-border)] bg-transparent px-3 py-2"
                  />
                </label>
                <label className="text-sm font-medium">
                  Status
                  <select
                    value={statusFilter}
                    onChange={(event) => setStatusFilter(event.target.value)}
                    className="mt-1.5 block w-full rounded-[var(--lp-radius)] border border-[var(--lp-border)] bg-[var(--lp-surface)] px-3 py-2"
                  >
                    <option value="">All statuses</option>
                    <option value="trial">Trial</option>
                    <option value="active">Active</option>
                    <option value="suspended">Suspended</option>
                    <option value="closed">Closed</option>
                  </select>
                </label>
                <label className="text-sm font-medium">
                  Plan
                  <input
                    value={planFilter}
                    onChange={(event) => setPlanFilter(event.target.value)}
                    placeholder="e.g. growth"
                    className="mt-1.5 block w-full rounded-[var(--lp-radius)] border border-[var(--lp-border)] bg-transparent px-3 py-2"
                  />
                </label>
                <button
                  type="submit"
                  disabled={pending}
                  className="self-end rounded-[var(--lp-radius)] bg-[var(--lp-accent)] px-4 py-2 font-semibold text-white disabled:opacity-60"
                >
                  Apply
                </button>
              </form>
            </div>
            {organizations.length === 0 ? (
              <div className="p-5">
                <EmptyState
                  dense
                  title="No organizations yet"
                  description="Customer sign-ups will appear here."
                />
              </div>
            ) : (
              <ul className="divide-y divide-[var(--lp-border)]">
                {organizations.map((organization) => (
                  <li
                    key={organization.id}
                    className="flex flex-wrap items-center justify-between gap-3 px-5 py-4"
                  >
                    <div>
                      <p className="font-medium">{organization.name}</p>
                      <p className="text-sm text-[var(--lp-ink-muted)]">
                        {organization.slug} · {organization.planCode} · {organization.status}
                      </p>
                      {sessionFormOrgId === organization.id ? (
                        <div className="mt-3 space-y-2">
                          <label className="block text-sm font-medium">
                            Support session reason (min {minReasonLength} characters)
                            <textarea
                              className="mt-1.5 block w-80 max-w-full rounded-[var(--lp-radius)] border border-[var(--lp-border)] bg-transparent p-2 text-sm"
                              rows={3}
                              value={sessionReason}
                              onChange={(event) => {
                                setSessionReason(event.target.value);
                              }}
                              placeholder="e.g. Investigating support ticket 12345"
                            />
                          </label>
                          <div className="flex flex-wrap gap-2">
                            <button
                              type="button"
                              disabled={pending || sessionReason.trim().length < minReasonLength}
                              onClick={() => {
                                startSupportSession(organization.id);
                              }}
                              className="rounded-[var(--lp-radius)] bg-[var(--lp-accent)] px-3 py-2 text-sm font-semibold text-white disabled:opacity-60"
                            >
                              Start session
                            </button>
                            <button
                              type="button"
                              disabled={pending}
                              onClick={() => {
                                setSessionFormOrgId(null);
                                setSessionReason("");
                              }}
                              className="rounded-[var(--lp-radius)] border border-[var(--lp-border)] px-3 py-2 text-sm font-semibold disabled:opacity-60"
                            >
                              Cancel
                            </button>
                          </div>
                        </div>
                      ) : null}
                    </div>
                    <div className="flex flex-wrap gap-2">
                      <Link
                        href={`/organizations/${organization.id}`}
                        className="rounded-[var(--lp-radius)] bg-[var(--lp-ink)] px-3 py-2 text-sm font-semibold text-white"
                      >
                        View details
                      </Link>
                      {organization.status === "suspended" ? (
                        <button
                          type="button"
                          disabled={pending}
                          onClick={() => {
                            updateStatus(organization.id, "activate");
                          }}
                          className="rounded-[var(--lp-radius)] bg-[var(--lp-accent)] px-3 py-2 text-sm font-semibold text-white disabled:opacity-60"
                        >
                          Activate
                        </button>
                      ) : (
                        <button
                          type="button"
                          disabled={pending || organization.status === "suspended"}
                          onClick={() => {
                            updateStatus(organization.id, "suspend");
                          }}
                          className="rounded-[var(--lp-radius)] border border-[var(--lp-border)] px-3 py-2 text-sm font-semibold disabled:opacity-60"
                        >
                          Suspend
                        </button>
                      )}
                      {organization.status !== "closed" ? (
                        <button
                          type="button"
                          disabled={pending}
                          onClick={() => closeOrganization(organization)}
                          className="rounded-[var(--lp-radius)] border border-[var(--lp-danger)] px-3 py-2 text-sm font-semibold text-[var(--lp-danger)] disabled:opacity-60"
                        >
                          Close
                        </button>
                      ) : null}
                      <button
                        type="button"
                        disabled={
                          pending ||
                          organization.status === "suspended" ||
                          organization.status === "closed"
                        }
                        onClick={() => {
                          setSessionFormOrgId(
                            sessionFormOrgId === organization.id ? null : organization.id,
                          );
                          setSessionReason("");
                        }}
                        className="rounded-[var(--lp-radius)] border border-[var(--lp-border)] px-3 py-2 text-sm font-semibold disabled:opacity-60"
                      >
                        Support session
                      </button>
                    </div>
                  </li>
                ))}
              </ul>
            )}
            {total > pageSize ? (
              <div className="flex flex-wrap items-center justify-between gap-3 border-t border-[var(--lp-border)] px-5 py-4">
                <p className="text-sm text-[var(--lp-ink-muted)]">
                  Page {page + 1} of {Math.ceil(total / pageSize)}
                </p>
                <div className="flex gap-2">
                  <button
                    type="button"
                    disabled={pending || page === 0}
                    onClick={() => reload(page - 1)}
                    className="rounded-[var(--lp-radius)] border border-[var(--lp-border)] px-4 py-2 text-sm font-semibold disabled:opacity-40"
                  >
                    Previous
                  </button>
                  <button
                    type="button"
                    disabled={pending || (page + 1) * pageSize >= total}
                    onClick={() => reload(page + 1)}
                    className="rounded-[var(--lp-radius)] bg-[var(--lp-accent)] px-4 py-2 text-sm font-semibold text-white disabled:opacity-40"
                  >
                    Next
                  </button>
                </div>
              </div>
            ) : null}
          </Surface>
        </Reveal>
      </div>
      );
}
