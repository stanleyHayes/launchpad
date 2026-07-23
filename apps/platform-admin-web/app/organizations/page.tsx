"use client";

import { useEffect, useState, useTransition } from "react";
import { useRouter } from "next/navigation";
import type { Organization } from "@launchpad/api-client";
import { ApiError } from "@launchpad/api-client";
import { EmptyState, PageHeader, Reveal, Surface } from "@launchpad/ui";
import { PlatformShell } from "@/components/platform-shell";
import { getClient } from "@/lib/api";
import { clearSession, getAccessToken } from "@/lib/session";

export default function OrganizationsPage() {
  const router = useRouter();
  const [pending, startTransition] = useTransition();
  const [organizations, setOrganizations] = useState<Organization[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [message, setMessage] = useState<string | null>(null);

  function reload() {
    startTransition(() => {
      void (async () => {
        try {
          setOrganizations(await getClient().listPlatformOrganizations());
        } catch (err) {
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
    reload();
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

  return (
    <PlatformShell>
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

        <Reveal delay={1}>
          <Surface className="overflow-hidden p-0">
            <div className="border-b border-[var(--lp-border)] px-5 py-4">
              <h2 className="text-lg font-semibold">All organizations</h2>
              <p className="text-sm text-[var(--lp-ink-muted)]">
                {organizations.length} tenants
              </p>
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
                    </div>
                    <div className="flex flex-wrap gap-2">
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
                    </div>
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
