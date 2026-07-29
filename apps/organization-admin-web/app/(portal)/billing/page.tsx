"use client";

import { useEffect, useState, useTransition } from "react";
import { useRouter } from "next/navigation";
import type { Plan, Subscription } from "@launchpad/api-client";
import { ApiError } from "@launchpad/api-client";
import { Button, EmptyState, Icon, PageHeader, Reveal, Surface } from "@launchpad/ui";
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

function isCustomPlan(plan: Plan): boolean {
  return plan.code === "enterprise" || plan.description.toLowerCase().includes("custom pricing");
}

export default function BillingPage() {
  const router = useRouter();
  const [pending, startTransition] = useTransition();
  const [changingPlan, startPlanTransition] = useTransition();
  const [subscription, setSubscription] = useState<Subscription | null>(null);
  const [plans, setPlans] = useState<Plan[]>([]);
  const [selectedPlanCode, setSelectedPlanCode] = useState<string | null>(null);
  const [success, setSuccess] = useState<string | null>(null);
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
  const selectedPlan = plans.find((plan) => plan.code === selectedPlanCode);

  function changePlan() {
    if (!selectedPlan) return;

    setError(null);
    setSuccess(null);
    startPlanTransition(() => {
      void (async () => {
        try {
          const updated = await getClient().updateBillingSubscription({
            planCode: selectedPlan.code,
          });
          setSubscription(updated);
          setSelectedPlanCode(null);
          setSuccess(`Your subscription is now on the ${selectedPlan.name} plan.`);
        } catch (err) {
          if (err instanceof ApiError && err.status === 401) {
            clearSession();
            router.replace("/login");
            return;
          }
          setError(err instanceof ApiError ? err.message : "Unable to change plan");
        }
      })();
    });
  }

  return (
      <div className="space-y-8">
        <Reveal>
          <PageHeader
            eyebrow="Account"
            title="Billing"
            description="Manage your subscription, compare benefits, and change plans."
          />
        </Reveal>

        {success ? (
          <div className="flex items-center gap-3 rounded-2xl border border-emerald-500/25 bg-emerald-500/10 px-4 py-3 text-sm text-emerald-800" role="status">
            <Icon name="check" className="size-5 shrink-0" />
            {success}
          </div>
        ) : null}

        {error ? (
          <p className="rounded-2xl border border-red-500/20 bg-red-500/10 px-4 py-3 text-sm text-[var(--lp-danger)]" role="alert">
            {error}
          </p>
        ) : null}

        <Reveal delay={1}>
          <Surface className="relative overflow-hidden">
            {subscription ? (
              <div className="grid gap-6 lg:grid-cols-[minmax(0,1.5fr)_minmax(280px,0.5fr)] lg:items-end">
                <div>
                  <div className="mb-5 flex items-center gap-3">
                    <span className="grid size-11 place-items-center rounded-2xl bg-[var(--lp-accent)]/10 text-[var(--lp-accent)]">
                      <Icon name="credit-card" className="size-5" />
                    </span>
                    <div>
                      <p className="text-xs font-semibold uppercase tracking-[0.14em] text-[var(--lp-ink-muted)]">Current subscription</p>
                      <h2 className="text-2xl font-semibold">{currentPlan?.name ?? subscription.planCode}</h2>
                    </div>
                  </div>
                  <p className="max-w-2xl text-sm leading-6 text-[var(--lp-ink-muted)]">
                    {currentPlan?.description ?? "Your active LaunchPad subscription."}
                  </p>
                </div>
                <dl className="grid grid-cols-2 gap-4 rounded-2xl bg-[var(--lp-paper)] p-4">
                  <div>
                    <dt className="text-xs text-[var(--lp-ink-muted)]">Status</dt>
                    <dd className="mt-1 font-semibold capitalize">{subscription.status}</dd>
                  </div>
                  <div>
                    <dt className="text-xs text-[var(--lp-ink-muted)]">Monthly</dt>
                    <dd className="mt-1 font-semibold">
                      {currentPlan
                        ? isCustomPlan(currentPlan)
                          ? "Custom"
                          : formatPrice(currentPlan.priceMonthlyCents, currentPlan.currency)
                        : "Unavailable"}
                    </dd>
                  </div>
                  {subscription.currentPeriodEnd ? (
                    <div className="col-span-2 border-t border-[var(--lp-border)] pt-3">
                      <dt className="text-xs text-[var(--lp-ink-muted)]">Next renewal</dt>
                      <dd className="mt-1 font-semibold">
                        {new Date(subscription.currentPeriodEnd).toLocaleDateString()}
                      </dd>
                    </div>
                  ) : null}
                </dl>
              </div>
            ) : (
              <p className="mt-4 text-sm text-[var(--lp-ink-muted)]">
                {pending ? "Loading subscription…" : "No subscription found"}
              </p>
            )}
          </Surface>
        </Reveal>

        <Reveal delay={2}>
          <section>
            <div className="mb-5 flex flex-wrap items-end justify-between gap-3">
              <div>
                <h2 className="text-xl font-semibold">Choose the right plan</h2>
                <p className="mt-1 text-sm text-[var(--lp-ink-muted)]">
                  Upgrade for more capability or move to a simpler plan when you need less.
                </p>
              </div>
              <p className="text-sm font-medium text-[var(--lp-ink-muted)]">{plans.length} plans available</p>
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
                  .sort((a, b) => {
                    if (isCustomPlan(a)) return 1;
                    if (isCustomPlan(b)) return -1;
                    return a.priceMonthlyCents - b.priceMonthlyCents;
                  })
                  .map((plan) => {
                    const isCurrent = subscription?.planCode === plan.code;
                    const custom = isCustomPlan(plan);
                    const isSelected = selectedPlanCode === plan.code;
                    const isUpgrade = currentPlan
                      ? plan.priceMonthlyCents > currentPlan.priceMonthlyCents
                      : true;
                    return (
                      <li
                        key={plan.code}
                        className={`lp-card relative flex min-h-[430px] flex-col overflow-hidden p-6 ${
                          isCurrent ? "ring-2 ring-[var(--lp-accent)]" : ""
                        }`}
                      >
                        <div className="mb-6 flex items-center justify-between gap-3">
                          <span className="grid size-10 place-items-center rounded-2xl bg-[var(--lp-accent)]/10 text-[var(--lp-accent)]">
                            <Icon name={custom ? "building" : plan.priceMonthlyCents === 0 ? "sparkles" : "chart"} className="size-5" />
                          </span>
                          {isCurrent ? (
                            <span className="rounded-full bg-[var(--lp-accent)]/10 px-3 py-1 text-xs font-semibold text-[var(--lp-accent)]">
                              Current plan
                            </span>
                          ) : null}
                        </div>
                        <h3 className="text-2xl font-semibold">{plan.name}</h3>
                        {plan.description ? (
                          <p className="mt-2 min-h-10 text-sm leading-5 text-[var(--lp-ink-muted)]">
                            {plan.description}
                          </p>
                        ) : null}
                        <p className="mt-5">
                          {custom ? (
                            <span className="text-3xl font-semibold tracking-tight">Let&apos;s talk</span>
                          ) : (
                            <>
                              <span className="text-3xl font-semibold tracking-tight">
                                {formatPrice(plan.priceMonthlyCents, plan.currency)}
                              </span>
                              <span className="text-sm text-[var(--lp-ink-muted)]"> / month</span>
                            </>
                          )}
                        </p>
                        {plan.features.length > 0 ? (
                          <ul className="mt-6 flex-1 space-y-3 border-t border-[var(--lp-border)] pt-5 text-sm text-[var(--lp-ink-muted)]">
                            {plan.features.map((feature) => (
                              <li key={feature} className="flex items-center gap-2">
                                <Icon name="check" className="size-4 shrink-0 text-[var(--lp-accent)]" />
                                {featureLabel(feature)}
                              </li>
                            ))}
                          </ul>
                        ) : null}
                        <div className="mt-6">
                          {isCurrent ? (
                            <Button className="w-full" variant="secondary" disabled>
                              Your current plan
                            </Button>
                          ) : custom ? (
                            <Button className="w-full" variant="secondary" onClick={() => router.push("/support")}>
                              Contact sales
                            </Button>
                          ) : isSelected ? (
                            <div className="rounded-2xl border border-[var(--lp-accent)]/25 bg-[var(--lp-accent)]/5 p-3">
                              <p className="text-sm font-semibold">
                                Confirm {isUpgrade ? "upgrade" : "downgrade"} to {plan.name}?
                              </p>
                              <p className="mt-1 text-xs leading-5 text-[var(--lp-ink-muted)]">
                                The plan change takes effect immediately.
                              </p>
                              <div className="mt-3 grid grid-cols-2 gap-2">
                                <Button variant="ghost" onClick={() => setSelectedPlanCode(null)} disabled={changingPlan}>
                                  Cancel
                                </Button>
                                <Button onClick={changePlan} disabled={changingPlan}>
                                  {changingPlan ? "Changing…" : "Confirm"}
                                </Button>
                              </div>
                            </div>
                          ) : (
                            <Button className="w-full" variant={isUpgrade ? "primary" : "secondary"} onClick={() => setSelectedPlanCode(plan.code)}>
                              {isUpgrade ? "Upgrade" : "Downgrade"} to {plan.name}
                            </Button>
                          )}
                        </div>
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
