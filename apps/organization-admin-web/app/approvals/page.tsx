"use client";

import { useEffect, useState, useTransition } from "react";
import { useRouter } from "next/navigation";
import type { Approval } from "@launchpad/api-client";
import { ApiError } from "@launchpad/api-client";
import { EmptyState, PageHeader, Reveal, Surface } from "@launchpad/ui";
import { AdminShell } from "@/components/admin-shell";
import { getClient } from "@/lib/api";
import { clearSession, getAccessToken } from "@/lib/session";

export default function ApprovalsPage() {
  const router = useRouter();
  const [pending, startTransition] = useTransition();
  const [approvals, setApprovals] = useState<Approval[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [message, setMessage] = useState<string | null>(null);

  function reload() {
    startTransition(() => {
      void (async () => {
        try {
          setApprovals(await getClient().listApprovals());
        } catch (err) {
          if (err instanceof ApiError && err.status === 401) {
            clearSession();
            router.replace("/login");
            return;
          }
          setError(err instanceof ApiError ? err.message : "Unable to load approvals");
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

  function decide(approvalId: string, approve: boolean) {
    setError(null);
    setMessage(null);
    startTransition(() => {
      void (async () => {
        try {
          await getClient().decideApproval(approvalId, {
            approve,
            note: approve ? "Approved" : "Needs revision",
          });
          setMessage(approve ? "Approved" : "Rejected");
          reload();
        } catch (err) {
          setError(err instanceof ApiError ? err.message : "Unable to decide approval");
        }
      })();
    });
  }

  return (
    <AdminShell>
      <div className="space-y-8">
        <Reveal>
          <PageHeader
            eyebrow="Approvals"
            title="Approval queue"
            description="Review employee submissions that need a manager decision."
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
              <h2 className="text-lg font-semibold">Queue</h2>
              <p className="text-sm text-[var(--lp-ink-muted)]">{approvals.length} approvals</p>
            </div>
            {approvals.length === 0 ? (
              <div className="p-5">
                <EmptyState
                  dense
                  title="No approvals yet"
                  description="Approval steps appear here when employees submit them."
                />
              </div>
            ) : (
              <ul className="divide-y divide-[var(--lp-border)]">
                {approvals.map((approval) => (
                  <li
                    key={approval.id}
                    className="flex flex-wrap items-center justify-between gap-3 px-5 py-4"
                  >
                    <div>
                      <p className="font-medium">{approval.status}</p>
                      <p className="text-sm text-[var(--lp-ink-muted)]">
                        Step {approval.stepAssignmentId}
                      </p>
                    </div>
                    {approval.status === "pending" ? (
                      <div className="flex gap-2">
                        <button
                          type="button"
                          disabled={pending}
                          onClick={() => {
                            decide(approval.id, true);
                          }}
                          className="rounded-[var(--lp-radius)] bg-[var(--lp-accent)] px-3 py-1.5 text-sm font-semibold text-white disabled:opacity-60"
                        >
                          Approve
                        </button>
                        <button
                          type="button"
                          disabled={pending}
                          onClick={() => {
                            decide(approval.id, false);
                          }}
                          className="rounded-[var(--lp-radius)] border border-[var(--lp-border)] px-3 py-1.5 text-sm font-semibold disabled:opacity-60"
                        >
                          Reject
                        </button>
                      </div>
                    ) : (
                      <p className="text-sm text-[var(--lp-ink-muted)]">{approval.note || "—"}</p>
                    )}
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
