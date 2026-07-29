"use client";

import { useEffect, useState, useTransition, type SyntheticEvent } from "react";
import { useRouter } from "next/navigation";
import type { FeatureFlag, FeatureFlagHistory } from "@launchpad/api-client";
import { ApiError } from "@launchpad/api-client";
import { EmptyState, PageHeader, Reveal, Surface } from "@launchpad/ui";
import { getClient } from "@/lib/api";
import { clearSession, getAccessToken } from "@/lib/session";

function formString(form: FormData, key: string): string {
  const value = form.get(key);
  return typeof value === "string" ? value.trim() : "";
}

export default function FeatureFlagsPage() {
  const router = useRouter();
  const [pending, startTransition] = useTransition();
  const [flags, setFlags] = useState<FeatureFlag[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [message, setMessage] = useState<string | null>(null);
  const [history, setHistory] = useState<FeatureFlagHistory[]>([]);
  const [historyKey, setHistoryKey] = useState<string | null>(null);

  function reload(isStale?: () => boolean) {
    startTransition(() => {
      void (async () => {
        try {
          const flagItems = await getClient().listPlatformFeatureFlags();
          if (isStale?.()) return;
          setFlags(flagItems);
        } catch (err) {
          if (isStale?.()) return;
          if (err instanceof ApiError && err.status === 401) {
            clearSession();
            router.replace("/login");
            return;
          }
          setError(err instanceof ApiError ? err.message : "Unable to load feature flags");
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

  function toggleFlag(flag: FeatureFlag) {
    setError(null);
    setMessage(null);
    startTransition(() => {
      void (async () => {
        try {
          await getClient().updatePlatformFeatureFlag(flag.key, { enabled: !flag.enabled });
          setMessage(`${flag.key} ${flag.enabled ? "disabled" : "enabled"}`);
          reload();
        } catch (err) {
          setError(err instanceof ApiError ? err.message : "Unable to update feature flag");
        }
      })();
    });
  }

  function onCreateFlag(event: SyntheticEvent<HTMLFormElement>) {
    event.preventDefault();
    setError(null);
    setMessage(null);
    const formEl = event.currentTarget;
    const form = new FormData(formEl);
    const planCodesRaw = formString(form, "planCodes");
    const planCodes = planCodesRaw
      ? planCodesRaw.split(",").map((item) => item.trim()).filter(Boolean)
      : undefined;
    const cohortRaw = formString(form, "cohortUserIds");
    const cohortUserIds = cohortRaw
      ? cohortRaw.split(",").map((item) => item.trim()).filter(Boolean)
      : undefined;

    startTransition(() => {
      void (async () => {
        try {
          await getClient().createPlatformFeatureFlag({
            key: formString(form, "key"),
            description: formString(form, "description"),
            enabled: form.get("enabled") === "on",
            planCodes,
            rolloutPercentage: Number(formString(form, "rolloutPercentage") || "100"),
            cohortUserIds,
            expiresAt: formString(form, "expiresAt")
              ? new Date(formString(form, "expiresAt")).toISOString()
              : undefined,
          });
          formEl.reset();
          setMessage("Feature flag created");
          reload();
        } catch (err) {
          setError(err instanceof ApiError ? err.message : "Unable to create feature flag");
        }
      })();
    });
  }

  function updateRollout(event: SyntheticEvent<HTMLFormElement>, flag: FeatureFlag) {
    event.preventDefault();
    const form = new FormData(event.currentTarget);
    const cohortUserIds = formString(form, "cohortUserIds")
      .split(",")
      .map((item) => item.trim())
      .filter(Boolean);
    const expiresAtRaw = formString(form, "expiresAt");

    startTransition(() => {
      void (async () => {
        try {
          await getClient().updatePlatformFeatureFlag(flag.key, {
            rolloutPercentage: Number(formString(form, "rolloutPercentage")),
            cohortUserIds,
            expiresAt: expiresAtRaw ? new Date(expiresAtRaw).toISOString() : "",
          });
          setMessage(`${flag.key} rollout updated`);
          reload();
        } catch (err) {
          setError(err instanceof ApiError ? err.message : "Unable to update rollout");
        }
      })();
    });
  }

  function loadHistory(key: string) {
    startTransition(() => {
      void (async () => {
        try {
          setHistory(await getClient().listPlatformFeatureFlagHistory(key));
          setHistoryKey(key);
        } catch (err) {
          setError(err instanceof ApiError ? err.message : "Unable to load rollout history");
        }
      })();
    });
  }

  return (
          <div className="space-y-8">
        <Reveal>
          <PageHeader
            eyebrow="Business"
            title="Feature flags"
            description="Manage global toggles and plan restrictions for tenant capabilities."
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
            <h2 className="text-lg font-semibold">Create flag</h2>
            <form
              className="mt-4 grid gap-3 md:grid-cols-2"
              onSubmit={onCreateFlag}
            >
              <input className="lp-input" name="key" placeholder="Flag key" required />
              <input
                className="lp-input md:col-span-2"
                name="description"
                placeholder="Description"
                required
              />
              <input
                className="lp-input md:col-span-2"
                name="planCodes"
                placeholder="Plan codes (comma-separated, optional)"
              />
              <label className="text-sm font-medium">
                Rollout percentage
                <input
                  className="lp-input mt-1"
                  name="rolloutPercentage"
                  type="number"
                  min={1}
                  max={100}
                  defaultValue={100}
                  required
                />
              </label>
              <label className="text-sm font-medium">
                Expiry (optional)
                <input className="lp-input mt-1" name="expiresAt" type="datetime-local" />
              </label>
              <input
                className="lp-input md:col-span-2"
                name="cohortUserIds"
                placeholder="Test cohort user IDs (comma-separated)"
              />
              <label className="flex items-center gap-2 text-sm">
                <input type="checkbox" name="enabled" defaultChecked />
                Enabled by default
              </label>
              <div className="md:col-span-2">
                <button
                  type="submit"
                  disabled={pending}
                  className="rounded-[var(--lp-radius)] bg-[var(--lp-accent)] px-4 py-2.5 text-sm font-semibold text-white disabled:opacity-60"
                >
                  Create flag
                </button>
              </div>
            </form>
          </Surface>
        </Reveal>

        <Reveal delay={2}>
          <Surface className="overflow-hidden p-0">
            <div className="border-b border-[var(--lp-border)] px-5 py-4">
              <h2 className="text-lg font-semibold">All flags</h2>
              <p className="text-sm text-[var(--lp-ink-muted)]">
                {pending && flags.length === 0 ? "Loading…" : `${flags.length} flags`}
              </p>
            </div>
            {flags.length === 0 ? (
              <div className="p-5">
                <EmptyState
                  dense
                  title="No feature flags yet"
                  description="Create a flag to control rollout across plans and tenants."
                />
              </div>
            ) : (
              <ul className="divide-y divide-[var(--lp-border)]">
                {flags.map((flag) => (
                  <li
                    key={flag.key}
                    className="grid gap-4 px-5 py-4 lg:grid-cols-[minmax(0,1fr)_minmax(24rem,1fr)]"
                  >
                    <div>
                      <p className="font-medium">{flag.key}</p>
                      <p className="text-sm text-[var(--lp-ink-muted)]">{flag.description}</p>
                      <p className="mt-1 text-xs text-[var(--lp-ink-muted)]">
                        {flag.enabled ? "Enabled" : "Disabled"}
                        {flag.planCodes.length > 0
                          ? ` · Plans: ${flag.planCodes.join(", ")}`
                          : " · All plans"}
                        {` · ${flag.rolloutPercentage}% rollout`}
                        {flag.expiresAt
                          ? ` · Expires ${new Date(flag.expiresAt).toLocaleString()}`
                          : ""}
                      </p>
                      {flag.cohortUserIds.length > 0 ? (
                        <p className="mt-1 text-xs text-[var(--lp-ink-muted)]">
                          Test cohort: {flag.cohortUserIds.join(", ")}
                        </p>
                      ) : null}
                    </div>
                    <form
                      className="grid gap-2 sm:grid-cols-3"
                      onSubmit={(event) => updateRollout(event, flag)}
                    >
                      <input
                        className="lp-input"
                        aria-label={`${flag.key} rollout percentage`}
                        name="rolloutPercentage"
                        type="number"
                        min={1}
                        max={100}
                        defaultValue={flag.rolloutPercentage}
                        required
                      />
                      <input
                        className="lp-input sm:col-span-2"
                        aria-label={`${flag.key} test cohort`}
                        name="cohortUserIds"
                        defaultValue={flag.cohortUserIds.join(", ")}
                        placeholder="Test cohort user IDs"
                      />
                      <input
                        className="lp-input sm:col-span-2"
                        aria-label={`${flag.key} expiry`}
                        name="expiresAt"
                        type="datetime-local"
                        defaultValue={flag.expiresAt?.slice(0, 16)}
                      />
                      <button
                        type="submit"
                        disabled={pending}
                        className="rounded-[var(--lp-radius)] border border-[var(--lp-border)] px-3 py-2 text-sm font-semibold disabled:opacity-60"
                      >
                        Save rollout
                      </button>
                      <button
                        type="button"
                        disabled={pending}
                        onClick={() => toggleFlag(flag)}
                        className="rounded-[var(--lp-radius)] border border-[var(--lp-border)] px-3 py-2 text-sm font-semibold disabled:opacity-60"
                      >
                        {flag.enabled ? "Kill switch: off" : "Enable"}
                      </button>
                      <button
                        type="button"
                        disabled={pending}
                        onClick={() => loadHistory(flag.key)}
                        className="rounded-[var(--lp-radius)] border border-[var(--lp-border)] px-3 py-2 text-sm font-semibold disabled:opacity-60"
                      >
                        View history
                      </button>
                    </form>
                  </li>
                ))}
              </ul>
            )}
          </Surface>
        </Reveal>
        {historyKey ? (
          <Reveal delay={3}>
            <Surface>
              <h2 className="text-lg font-semibold">Rollout history · {historyKey}</h2>
              {history.length === 0 ? (
                <p className="mt-3 text-sm text-[var(--lp-ink-muted)]">No mutations recorded yet.</p>
              ) : (
                <ol className="mt-4 divide-y divide-[var(--lp-border)]">
                  {history.map((item) => (
                    <li key={item.id} className="py-3 text-sm">
                      <p className="font-medium">{item.action.replaceAll("_", " ")}</p>
                      <p className="text-[var(--lp-ink-muted)]">
                        {new Date(item.createdAt).toLocaleString()} · {item.actorUserId || "system"} ·{" "}
                        {item.snapshot.rolloutPercentage}% rollout
                      </p>
                    </li>
                  ))}
                </ol>
              )}
            </Surface>
          </Reveal>
        ) : null}
      </div>
      );
}
