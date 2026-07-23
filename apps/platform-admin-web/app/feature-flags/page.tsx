"use client";

import { useEffect, useState, useTransition, type SyntheticEvent } from "react";
import { useRouter } from "next/navigation";
import type { FeatureFlag } from "@launchpad/api-client";
import { ApiError } from "@launchpad/api-client";
import { EmptyState, PageHeader, Reveal, Surface } from "@launchpad/ui";
import { PlatformShell } from "@/components/platform-shell";
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

  function reload() {
    startTransition(() => {
      void (async () => {
        try {
          setFlags(await getClient().listPlatformFeatureFlags());
        } catch (err) {
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
    reload();
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
    const form = new FormData(event.currentTarget);
    const planCodesRaw = formString(form, "planCodes");
    const planCodes = planCodesRaw
      ? planCodesRaw.split(",").map((item) => item.trim()).filter(Boolean)
      : undefined;

    startTransition(() => {
      void (async () => {
        try {
          await getClient().createPlatformFeatureFlag({
            key: formString(form, "key"),
            description: formString(form, "description"),
            enabled: form.get("enabled") === "on",
            planCodes,
          });
          event.currentTarget.reset();
          setMessage("Feature flag created");
          reload();
        } catch (err) {
          setError(err instanceof ApiError ? err.message : "Unable to create feature flag");
        }
      })();
    });
  }

  return (
    <PlatformShell>
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
                    className="flex flex-wrap items-center justify-between gap-3 px-5 py-4"
                  >
                    <div>
                      <p className="font-medium">{flag.key}</p>
                      <p className="text-sm text-[var(--lp-ink-muted)]">{flag.description}</p>
                      <p className="mt-1 text-xs text-[var(--lp-ink-muted)]">
                        {flag.enabled ? "Enabled" : "Disabled"}
                        {flag.planCodes.length > 0
                          ? ` · Plans: ${flag.planCodes.join(", ")}`
                          : " · All plans"}
                      </p>
                    </div>
                    <button
                      type="button"
                      disabled={pending}
                      onClick={() => {
                        toggleFlag(flag);
                      }}
                      className="rounded-[var(--lp-radius)] border border-[var(--lp-border)] px-3 py-2 text-sm font-semibold disabled:opacity-60"
                    >
                      {flag.enabled ? "Disable" : "Enable"}
                    </button>
                  </li>
                ))}
              </ul>
            )}
          </Surface>
        </Reveal>
      </div>
    </PlatformShell>
  );
}
