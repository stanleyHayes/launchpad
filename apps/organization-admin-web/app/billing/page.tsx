"use client";

import { useEffect, useState, useTransition } from "react";
import { useRouter } from "next/navigation";
import type { Plan, Subscription } from "@launchpad/api-client";
import { ApiError } from "@launchpad/api-client";
import { EmptyState, PageHeader, Reveal, Surface } from "@launchpad/ui";
import { AdminShell } from "@/components/admin-shell";
import { getClient } from "@/lib/api";
import { clearSession, getAccessToken } from "@/lib/session";

function formatPrice(cents: number, currency: string): string {
  return new Intl.NumberFormat(undefined, {
    style: "currency",
    currency,
  }).format(cents / 100);
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

    startTransition(() => {
      void (async () => {
        try {
          const client = getClient();
          const [subscriptionItem, planItems] = await Promise.all([
            client.getBillingSubscription(),
            client.listBillingPlans(),
          ]);
          setSubscription(subscriptionItem);
          setPlans(planItems);
        } catch (err) {
          if (err instanceof ApiError && err.status === 401) {
            clearSession();
            router.replace("/login");
            return;
          }
          setError(err instanceof ApiError ? err.message : "Unable to load billing");
        }
      })();
    });
  }, [router]);

  const currentPlan = plans.find((plan) => plan.code === subscription?.planCode);

  return (
    <AdminShell>
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
          <Surface className="overflow-hidden p-0">
            <div className="border-b border-[var(--lp-border)] px-5 py-4">
              <h2 className="text-lg font-semibold">Available plans</h2>
              <p className="text-sm text-[var(--lp-ink-muted)]">{plans.length} plans</p>
            </div>
            {plans.length === 0 ? (
              <div className="p-5">
                <EmptyState
                  dense
                  title="No plans available"
                  description="Contact platform support to change your subscription."
                />
              </div>
            ) : (
              <ul className="divide-y divide-[var(--lp-border)]">
                {plans.map((plan) => (
                  <li key={plan.code} className="px-5 py-4">
                    <div className="flex flex-wrap items-start justify-between gap-2">
                      <div>
                        <p className="font-medium">{plan.name}</p>
                        <p className="text-sm text-[var(--lp-ink-muted)]">{plan.code}</p>
                      </div>
                      <p className="text-sm font-medium">
                        {formatPrice(plan.priceMonthlyCents, plan.currency)}/mo
                      </p>
                    </div>
                    {plan.description ? (
                      <p className="mt-2 text-sm text-[var(--lp-ink-muted)]">{plan.description}</p>
                    ) : null}
                    {plan.features.length > 0 ? (
                      <ul className="mt-2 list-inside list-disc text-sm text-[var(--lp-ink-muted)]">
                        {plan.features.map((feature) => (
                          <li key={feature}>{feature}</li>
                        ))}
                      </ul>
                    ) : null}
                    {subscription?.planCode === plan.code ? (
                      <p className="mt-2 text-xs font-semibold uppercase tracking-wide text-[var(--lp-accent)]">
                        Current plan
                      </p>
                    ) : null}
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
