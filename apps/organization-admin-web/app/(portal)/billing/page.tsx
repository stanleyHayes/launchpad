"use client";

import { useEffect, useState, useTransition } from "react";
import { useRouter } from "next/navigation";
import type { Plan, Subscription } from "@launchpad/api-client";
import { ApiError } from "@launchpad/api-client";
import { EmptyState, PageHeader, Reveal, Surface } from "@launchpad/ui";
import { getClient } from "@/lib/api";
import { clearSession, getAccessToken } from "@/lib/session";

function formatPrice(cents: number, currency: string): string {
  return new Intl.NumberFormat(undefined, {
    style: "currency",
    currency,
  }).format(cents / 100);
}

// Human-readable labels for plan feature codes.
const featureLabels: Record<string, string> = {
  core_onboarding: "Core onboarding",
  analytics: "Analytics",
  sso: "SSO & SCIM",
  support_sla: "SLA support",
};

function featureLabel(code: string): string {
  return featureLabels[code] ?? code;
}

export default function BillingPage() {
  const router = useRouter();
  const [pending, startTransition] = useTransition();
  const [subscription, setSubscription] = useState<Subscription | null>(null);
  const [plans, setPlans] = useState<Plan[]>([]);
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
          const client = getClient();
          const [subscriptionItem, planItems] = await Promise.all([
            client.getBillingSubscription(),
            client.listBillingPlans(),
          ]);
          if (stale) return;
          setSubscription(subscriptionItem);
          setPlans(planItems);
        } catch (err) {
          if (stale) return;
          if (err instanceof ApiError && err.status === 401) {
            clearSession();
            router.replace("/login");
            return;
          }
          setError(err instanceof ApiError ? err.message : "Unable to load billing");
        }
      })();
    });
    return () => {
      stale = true;
    };
  }, [router]);

  const currentPlan = plans.find((plan) => plan.code === subscription?.planCode);

  return (
          <div className="space-y-8">
        <Reveal>
          <PageHeader
            eyebrow="Account"
            title="Billing"
            description="Your current subscription and available plans."
          />
        </Reveal>

        {error ? (
          <p className="text-[var(--lp-danger)]" role="alert">
            {error}
          </p>
        ) : null}

        <Reveal delay={1}>
          <Surface>
            <h2 className="text-lg font-semibold">Current subscription</h2>
            {subscription ? (
              <dl className="mt-4 grid gap-3 sm:grid-cols-2">
                <div>
                  <dt className="text-sm text-[var(--lp-ink-muted)]">Plan</dt>
                  <dd className="font-medium">
                    {currentPlan?.name ?? subscription.planCode}
                  </dd>
                </div>
                <div>
                  <dt className="text-sm text-[var(--lp-ink-muted)]">Status</dt>
                  <dd className="font-medium capitalize">{subscription.status}</dd>
                </div>
                {subscription.currentPeriodEnd ? (
                  <div>
                    <dt className="text-sm text-[var(--lp-ink-muted)]">Current period ends</dt>
                    <dd className="font-medium">
                      {new Date(subscription.currentPeriodEnd).toLocaleDateString()}
                    </dd>
                  </div>
                ) : null}
                {currentPlan ? (
                  <div>
                    <dt className="text-sm text-[var(--lp-ink-muted)]">Monthly price</dt>
                    <dd className="font-medium">
                      {formatPrice(currentPlan.priceMonthlyCents, currentPlan.currency)}
                    </dd>
                  </div>
                ) : null}
              </dl>
            ) : (
              <p className="mt-4 text-sm text-[var(--lp-ink-muted)]">
                {pending ? "Loading subscription…" : "No subscription found"}
              </p>
            )}
          </Surface>
        </Reveal>

        <Reveal delay={2}>
          <section>
            <div className="mb-4">
              <h2 className="text-lg font-semibold">Available plans</h2>
              <p className="text-sm text-[var(--lp-ink-muted)]">{plans.length} plans</p>
            </div>
            {plans.length === 0 ? (
              <Surface>
                <EmptyState
                  dense
                  title="No plans available"
                  description="Contact platform support to change your subscription."
                />
              </Surface>
            ) : (
              <ul className="grid gap-5 md:grid-cols-3">
                {[...plans]
                  .sort((a, b) => a.priceMonthlyCents - b.priceMonthlyCents)
                  .map((plan) => {
                    const isCurrent = subscription?.planCode === plan.code;
                    return (
                      <li
                        key={plan.code}
                        className={`lp-card flex flex-col p-6 ${
                          isCurrent ? "ring-2 ring-[var(--lp-brand)]" : ""
                        }`}
                      >
                        <div className="flex items-start justify-between gap-2">
                          <div>
                            <p className="text-lg font-semibold">{plan.name}</p>
                            <p className="text-sm text-[var(--lp-ink-muted)]">{plan.code}</p>
                          </div>
                          <p className="text-right">
                            <span className="text-xl font-semibold tracking-tight">
                              {formatPrice(plan.priceMonthlyCents, plan.currency)}
                            </span>
                            <span className="text-sm text-[var(--lp-ink-muted)]">/mo</span>
                          </p>
                        </div>
                        {plan.description ? (
                          <p className="mt-3 text-sm text-[var(--lp-ink-muted)]">
                            {plan.description}
                          </p>
                        ) : null}
                        {plan.features.length > 0 ? (
                          <ul className="mt-4 flex-1 space-y-2 border-t border-[var(--lp-border)] pt-4 text-sm text-[var(--lp-ink-muted)]">
                            {plan.features.map((feature) => (
                              <li key={feature} className="flex items-center gap-2">
                                <span className="h-1 w-1 rounded-full bg-[var(--lp-signal)]" />
                                {featureLabel(feature)}
                              </li>
                            ))}
                          </ul>
                        ) : null}
                        {isCurrent ? (
                          <p className="mt-4 text-xs font-semibold uppercase tracking-wide text-[var(--lp-accent)]">
                            Current plan
                          </p>
                        ) : null}
                      </li>
                    );
                  })}
              </ul>
            )}
          </section>
        </Reveal>
      </div>
      );
}
