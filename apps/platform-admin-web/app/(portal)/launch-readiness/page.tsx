"use client";

import { useEffect, useState, useTransition } from "react";
import { useRouter } from "next/navigation";
import type { LaunchReadinessCheck } from "@launchpad/api-client";
import { ApiError } from "@launchpad/api-client";
import { EmptyState, PageHeader, Reveal, Surface } from "@launchpad/ui";
import { getClient } from "@/lib/api";
import { clearSession, getAccessToken } from "@/lib/session";

const statusMeta: Record<
  LaunchReadinessCheck["status"],
  { label: string; icon: string; color: string }
> = {
  ready: { label: "Ready", icon: "✓", color: "var(--lp-success)" },
  watch: { label: "Watch", icon: "!", color: "var(--lp-warning)" },
  blocked: { label: "Blocked", icon: "✕", color: "var(--lp-danger)" },
};

export default function LaunchReadinessPage() {
  const router = useRouter();
  const [pending, startTransition] = useTransition();
  const [checks, setChecks] = useState<LaunchReadinessCheck[]>([]);
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
          const report = await getClient().getLaunchReadiness();
          if (stale) return;
          setChecks(report.checks);
        } catch (err) {
          if (stale) return;
          if (err instanceof ApiError && err.status === 401) {
            clearSession();
            router.replace("/login");
            return;
          }
          setError(err instanceof ApiError ? err.message : "Unable to load launch readiness");
        }
      })();
    });
    return () => {
      stale = true;
    };
  }, [router]);

  const blockedCount = checks.filter((check) => check.status === "blocked").length;
  const watchCount = checks.filter((check) => check.status === "watch").length;

  return (
          <div className="space-y-8">
        <Reveal>
          <PageHeader
            eyebrow="Operations"
            title="Launch readiness"
            description="Live signals that decide whether this environment can serve a launch."
          />
        </Reveal>

        {error ? (
          <p className="text-[var(--lp-danger)]" role="alert">
            {error}
          </p>
        ) : null}

        <Reveal delay={1}>
          {checks.length === 0 ? (
            <Surface>
              <EmptyState
                dense
                title={pending ? "Loading checks…" : "No checks reported"}
                description="Readiness signals are evaluated on every request."
              />
            </Surface>
          ) : (
            <>
              <p className="mb-4 text-sm text-[var(--lp-ink-muted)]">
                {checks.length} checks
                {blockedCount > 0 ? ` · ${blockedCount} blocked` : ""}
                {watchCount > 0 ? ` · ${watchCount} on watch` : ""}
              </p>
              <ul className="grid gap-5 md:grid-cols-2">
                {checks.map((check) => {
                  const meta = statusMeta[check.status];
                  return (
                    <li key={check.name} className="lp-card p-6">
                      <div className="flex items-center gap-3">
                        <span
                          aria-hidden="true"
                          className="flex h-8 w-8 items-center justify-center rounded-full text-sm font-bold text-white"
                          style={{ backgroundColor: meta.color }}
                        >
                          {meta.icon}
                        </span>
                        <div>
                          <p className="font-medium">{check.name}</p>
                          <p
                            className="text-xs font-semibold uppercase tracking-wide"
                            style={{ color: meta.color }}
                          >
                            {meta.label}
                          </p>
                        </div>
                      </div>
                      <p className="mt-3 text-sm text-[var(--lp-ink-muted)]">{check.summary}</p>
                      {check.action ? (
                        <p className="mt-2 border-t border-[var(--lp-border)] pt-2 text-sm">
                          <span className="font-semibold">Action: </span>
                          {check.action}
                        </p>
                      ) : null}
                    </li>
                  );
                })}
              </ul>
            </>
          )}
        </Reveal>
      </div>
      );
}
