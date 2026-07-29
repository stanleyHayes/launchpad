"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { useParams, useRouter } from "next/navigation";
import type { PlatformOrganizationDetail } from "@launchpad/api-client";
import { ApiError } from "@launchpad/api-client";
import { PageHeader, Reveal, Surface } from "@launchpad/ui";
import { getClient } from "@/lib/api";
import { clearSession, getAccessToken } from "@/lib/session";

const resourceLabels: Record<string, string> = {
  employees: "Employees",
  journey_templates: "Journey templates",
  knowledge_documents: "Knowledge documents",
  integrations: "Integrations",
};

function formatDate(value?: string): string {
  return value ? new Date(value).toLocaleString() : "Not completed";
}

export default function OrganizationDetailPage() {
  const params = useParams<{ organizationID: string }>();
  const router = useRouter();
  const [detail, setDetail] = useState<PlatformOrganizationDetail | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!getAccessToken()) {
      router.replace("/login");
      return;
    }

    let stale = false;
    void getClient()
      .getPlatformOrganizationDetail(params.organizationID)
      .then((result) => {
        if (!stale) setDetail(result);
      })
      .catch((err: unknown) => {
        if (stale) return;
        if (err instanceof ApiError && err.status === 401) {
          clearSession();
          router.replace("/login");
          return;
        }
        setError(err instanceof ApiError ? err.message : "Unable to load organization");
      });
    return () => {
      stale = true;
    };
  }, [params.organizationID, router]);

  if (error) {
    return (
      <div className="space-y-5">
        <Link href="/organizations" className="text-sm font-semibold text-[var(--lp-accent)]">
          ← Back to organizations
        </Link>
        <Surface>
          <p className="text-[var(--lp-danger)]" role="alert">{error}</p>
        </Surface>
      </div>
    );
  }

  if (!detail) {
    return <Surface className="animate-pulse text-[var(--lp-ink-muted)]">Loading organization…</Surface>;
  }

  const { organization, usage } = detail;

  return (
    <div className="space-y-8">
      <Reveal>
        <Link href="/organizations" className="mb-5 inline-flex text-sm font-semibold text-[var(--lp-accent)]">
          ← Back to organizations
        </Link>
        <PageHeader
          eyebrow="Organization profile"
          title={organization.name}
          description={`Tenant record, subscription capacity, and onboarding state for ${organization.slug}.`}
        />
      </Reveal>

      <Reveal delay={1}>
        <div className="grid gap-5 lg:grid-cols-[1.25fr_.75fr]">
          <Surface className="space-y-6">
            <div className="flex flex-wrap items-center justify-between gap-3">
              <div>
                <p className="text-xs font-bold uppercase tracking-[0.18em] text-[var(--lp-ink-muted)]">
                  Account profile
                </p>
                <h2 className="mt-2 text-2xl font-semibold">Tenant information</h2>
              </div>
              <span className="rounded-full border border-[var(--lp-accent)] px-3 py-1 text-sm font-semibold text-[var(--lp-accent)]">
                {organization.status}
              </span>
            </div>
            <dl className="grid gap-x-8 gap-y-5 sm:grid-cols-2">
              {[
                ["Organization ID", organization.id],
                ["Workspace slug", organization.slug],
                ["Plan", organization.planCode],
                ["Timezone", organization.timezone],
                ["Custom domain", organization.customDomain || "Not configured"],
                ["Setup progress", `Step ${organization.setupStep ?? 0} of 10`],
                ["Setup completed", formatDate(organization.setupCompletedAt)],
                ["Created", formatDate(organization.createdAt)],
                ["Last updated", formatDate(organization.updatedAt)],
              ].map(([label, value]) => (
                <div key={label} className="border-b border-[var(--lp-border)] pb-4">
                  <dt className="text-xs font-bold uppercase tracking-[0.12em] text-[var(--lp-ink-muted)]">{label}</dt>
                  <dd className="mt-1.5 break-words font-medium">{value}</dd>
                </div>
              ))}
            </dl>
          </Surface>

          <Surface className="space-y-5">
            <div>
              <p className="text-xs font-bold uppercase tracking-[0.18em] text-[var(--lp-ink-muted)]">
                Brand configuration
              </p>
              <h2 className="mt-2 text-2xl font-semibold">Workspace identity</h2>
            </div>
            <div
              className="h-28 rounded-[var(--lp-radius)] border border-[var(--lp-border)]"
              style={{ background: organization.branding?.primaryColor || "var(--lp-accent)" }}
            />
            <dl className="space-y-4 text-sm">
              <div className="flex justify-between gap-4">
                <dt className="text-[var(--lp-ink-muted)]">Primary</dt>
                <dd className="font-semibold">{organization.branding?.primaryColor || "Default"}</dd>
              </div>
              <div className="flex justify-between gap-4">
                <dt className="text-[var(--lp-ink-muted)]">Accent</dt>
                <dd className="font-semibold">{organization.branding?.accentColor || "Default"}</dd>
              </div>
              <div className="flex justify-between gap-4">
                <dt className="text-[var(--lp-ink-muted)]">Logo</dt>
                <dd className="max-w-48 truncate font-semibold">{organization.branding?.logoUrl || "Not uploaded"}</dd>
              </div>
            </dl>
          </Surface>
        </div>
      </Reveal>

      <Reveal delay={2}>
        <section className="space-y-4">
          <div>
            <p className="text-xs font-bold uppercase tracking-[0.18em] text-[var(--lp-ink-muted)]">
              {usage.planCode} plan
            </p>
            <h2 className="mt-2 text-2xl font-semibold">Subscription usage</h2>
            <p className="mt-1 text-[var(--lp-ink-muted)]">
              Limits are enforced by the API when tenant teams create these resources.
            </p>
          </div>
          <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
            {usage.items.map((item) => {
              const unlimited = item.limit < 0;
              const percent = unlimited ? 0 : Math.min(100, Math.round((item.used / item.limit) * 100));
              return (
                <Surface key={item.resource} className="space-y-5">
                  <div>
                    <p className="text-sm font-semibold text-[var(--lp-ink-muted)]">
                      {resourceLabels[item.resource] ?? item.resource}
                    </p>
                    <p className="mt-2 text-3xl font-semibold">
                      {item.used}
                      <span className="text-base text-[var(--lp-ink-muted)]">
                        {" "}/ {unlimited ? "Unlimited" : item.limit}
                      </span>
                    </p>
                  </div>
                  <div className="h-2 overflow-hidden rounded-full bg-[var(--lp-border)]">
                    <div
                      className="h-full rounded-full bg-[var(--lp-accent)] transition-[width]"
                      style={{ width: unlimited ? "12%" : `${percent}%` }}
                    />
                  </div>
                  <p className="text-xs font-semibold text-[var(--lp-ink-muted)]">
                    {unlimited ? "No plan cap" : `${Math.max(0, item.limit - item.used)} remaining`}
                  </p>
                </Surface>
              );
            })}
          </div>
        </section>
      </Reveal>
    </div>
  );
}
