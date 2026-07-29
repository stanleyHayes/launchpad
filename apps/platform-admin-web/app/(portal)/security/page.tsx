"use client";

import { useCallback, useEffect, useState } from "react";
import type { AccessReviewItem } from "@launchpad/api-client";
import { ApiError } from "@launchpad/api-client";
import { EmptyState, PageHeader, Surface } from "@launchpad/ui";
import { getClient } from "@/lib/api";

export default function SecurityCenterPage() {
  const [items, setItems] = useState<AccessReviewItem[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState<string | null>(null);

  const load = useCallback(async () => {
    try {
      setItems(await getClient().getPlatformAccessReview());
      setError(null);
    } catch (cause) {
      setError(cause instanceof ApiError ? cause.message : "Unable to load security posture");
    }
  }, []);

  useEffect(() => { void load(); }, [load]);

  async function attest(staffId: string) {
    setBusy(staffId);
    try {
      await getClient().attestPlatformAccess(staffId);
      await load();
    } catch (cause) {
      setError(cause instanceof ApiError ? cause.message : "Unable to attest access");
    } finally { setBusy(null); }
  }

  async function grant(staffId: string) {
    const reason = window.prompt("Emergency access reason");
    if (!reason) return;
    const duration = Number(window.prompt("Duration in minutes (maximum 60)", "30"));
    setBusy(staffId);
    try {
      await getClient().grantPlatformBreakGlass(staffId, reason, duration);
      await load();
    } catch (cause) {
      setError(cause instanceof ApiError ? cause.message : "Unable to grant emergency access");
    } finally { setBusy(null); }
  }

  async function revoke(staffId: string) {
    setBusy(staffId);
    try {
      await getClient().revokePlatformBreakGlass(staffId);
      await load();
    } catch (cause) {
      setError(cause instanceof ApiError ? cause.message : "Unable to revoke emergency access");
    } finally { setBusy(null); }
  }

  const activeBreakGlass = items.filter(({ staff }) =>
    staff.breakGlass && !staff.breakGlass.revokedAt && new Date(staff.breakGlass.expiresAt) > new Date(),
  ).length;
  const due = items.filter((item) => item.reviewDue).length;

  return (
    <div className="space-y-8">
      <PageHeader eyebrow="Security" title="Security center" description="Review privileged staff access and control time-bounded emergency elevation." />
      {error ? <p role="alert" className="text-[var(--lp-danger)]">{error}</p> : null}
      <section className="grid gap-4 sm:grid-cols-3">
        <Surface><p className="text-sm text-[var(--lp-ink-muted)]">Staff accounts</p><p className="mt-2 text-3xl font-semibold">{items.length}</p></Surface>
        <Surface><p className="text-sm text-[var(--lp-ink-muted)]">Reviews due</p><p className="mt-2 text-3xl font-semibold">{due}</p></Surface>
        <Surface><p className="text-sm text-[var(--lp-ink-muted)]">Active break-glass</p><p className="mt-2 text-3xl font-semibold">{activeBreakGlass}</p></Surface>
      </section>
      <Surface className="overflow-hidden p-0">
        {items.length === 0 ? <div className="p-5"><EmptyState dense title="No staff access records" description="Platform staff appear here for review." /></div> : (
          <ul className="divide-y divide-[var(--lp-border)]">
            {items.map(({ staff, reviewDue, effectiveRoleCode }) => {
              const grantDetails = staff.breakGlass;
              const grantActive = grantDetails && !grantDetails.revokedAt && new Date(grantDetails.expiresAt) > new Date();
              return (
                <li key={staff.id} className="flex flex-wrap items-center justify-between gap-4 px-5 py-4">
                  <div>
                    <p className="font-semibold">{staff.displayName || staff.email}</p>
                    <p className="text-sm text-[var(--lp-ink-muted)]">{staff.email} · {effectiveRoleCode} · {staff.status}</p>
                    <p className={reviewDue ? "text-sm text-[var(--lp-danger)]" : "text-sm text-[var(--lp-success)]"}>
                      {reviewDue ? "Quarterly access review due" : `Reviewed ${new Date(staff.accessReviewedAt!).toLocaleDateString()}`}
                    </p>
                    {grantActive ? <p className="mt-1 text-sm text-[var(--lp-danger)]">Emergency elevation until {new Date(grantDetails.expiresAt).toLocaleString()} · {grantDetails.reason}</p> : null}
                  </div>
                  <div className="flex flex-wrap gap-2">
                    <button className="lp-btn lp-btn--secondary" disabled={busy === staff.id} onClick={() => { void attest(staff.id); }}>Attest</button>
                    {grantActive ? (
                      <button className="lp-btn lp-btn--secondary" disabled={busy === staff.id} onClick={() => { void revoke(staff.id); }}>Revoke emergency access</button>
                    ) : (
                      <button className="lp-btn lp-btn--secondary" disabled={busy === staff.id || staff.status !== "active"} onClick={() => { void grant(staff.id); }}>Grant break-glass</button>
                    )}
                  </div>
                </li>
              );
            })}
          </ul>
        )}
      </Surface>
    </div>
  );
}
