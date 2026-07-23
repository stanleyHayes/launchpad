"use client";

import { useEffect, useState, useTransition, type SyntheticEvent } from "react";
import { useRouter } from "next/navigation";
import type { Plan, Subscription } from "@launchpad/api-client";
import { ApiError } from "@launchpad/api-client";
import { EmptyState, PageHeader, Reveal, Surface } from "@launchpad/ui";
import { PlatformShell } from "@/components/platform-shell";
import { getClient } from "@/lib/api";
import { clearSession, getAccessToken } from "@/lib/session";

function formString(form: FormData, key: string): string {
  const value = form.get(key);
  return typeof value === "string" ? value.trim() : "";
}

function formatPrice(cents: number, currency: string): string {
  return new Intl.NumberFormat(undefined, {
    style: "currency",
    currency,
  }).format(cents / 100);
}

export default function BillingPage() {
  const router = useRouter();
  const [pending, startTransition] = useTransition();
  const [plans, setPlans] = useState<Plan[]>([]);
  const [subscriptions, setSubscriptions] = useState<Subscription[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [message, setMessage] = useState<string | null>(null);

  function reload() {
    startTransition(() => {
      void (async () => {
        try {
          const client = getClient();
          const [planItems, subscriptionItems] = await Promise.all([
            client.listPlatformPlans(),
            client.listPlatformSubscriptions(),
          ]);
          setPlans(planItems);
          setSubscriptions(subscriptionItems);
        } catch (err) {
          if (err instanceof ApiError && err.status === 401) {
            clearSession();
            router.replace("/login");
            return;
          }
          setError(err instanceof ApiError ? err.message : "Unable to load billing data");
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

  function onSetSubscription(event: SyntheticEvent<HTMLFormElement>) {
    event.preventDefault();
    setError(null);
    setMessage(null);
    const form = new FormData(event.currentTarget);
    const organizationId = formString(form, "organizationId");
    const planCode = formString(form, "planCode");
    const status = formString(form, "status");

    startTransition(() => {
      void (async () => {
        try {
          await getClient().setOrganizationSubscription(organizationId, {
            planCode,
            status: status || undefined,
          });
          event.currentTarget.reset();
          setMessage("Organization subscription updated");
          reload();
        } catch (err) {
          setError(
            err instanceof ApiError ? err.message : "Unable to set organization subscription",
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
            eyebrow="Business"
            title="Billing"
            description="Review sellable plans, tenant subscriptions, and assign plan codes."
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
            <h2 className="text-lg font-semibold">Set organization subscription</h2>
            <form
              className="mt-4 grid gap-3 md:grid-cols-3"
              onSubmit={onSetSubscription}
            >
              <input
                className="lp-input"
                name="organizationId"
                placeholder="Organization ID"
                required
              />
              <select className="lp-input" name="planCode" required defaultValue="">
                <option value="" disabled>
                  Select plan
                </option>
                {plans.map((plan) => (
                  <option key={plan.code} value={plan.code}>
                    {plan.name} ({plan.code})
                  </option>
                ))}
              </select>
              <select className="lp-input" name="status" defaultValue="">
                <option value="">Default status</option>
                <option value="trialing">Trialing</option>
                <option value="active">Active</option>
                <option value="past_due">Past due</option>
                <option value="canceled">Canceled</option>
              </select>
              <div className="md:col-span-3">
                <button
                  type="submit"
                  disabled={pending}
                  className="rounded-[var(--lp-radius)] bg-[var(--lp-accent)] px-4 py-2.5 text-sm font-semibold text-white disabled:opacity-60"
                >
                  Assign subscription
                </button>
              </div>
            </form>
          </Surface>
        </Reveal>

        <Reveal delay={2}>
          <section className="grid gap-6 lg:grid-cols-2">
            <Surface className="overflow-hidden p-0">
              <div className="border-b border-[var(--lp-border)] px-5 py-4">
                <h2 className="text-lg font-semibold">Plans</h2>
                <p className="text-sm text-[var(--lp-ink-muted)]">{plans.length} plans</p>
              </div>
              {plans.length === 0 ? (
                <div className="p-5">
                  <EmptyState dense title="No plans" description="Billing plans will appear here." />
                </div>
              ) : (
                <ul className="divide-y divide-[var(--lp-border)]">
                  {plans.map((plan) => (
                    <li key={plan.code} className="px-5 py-4">
                      <div className="flex flex-wrap items-start justify-between gap-2">
                        <div>
                          <p className="font-medium">{plan.name}</p>
                          <p className="text-sm text-[var(--lp-ink-muted)]">
                            {plan.code} · {plan.active ? "Active" : "Inactive"}
                          </p>
                        </div>
                        <p className="text-sm font-medium">
                          {formatPrice(plan.priceMonthlyCents, plan.currency)}/mo
                        </p>
                      </div>
                      {plan.description ? (
                        <p className="mt-2 text-sm text-[var(--lp-ink-muted)]">{plan.description}</p>
                      ) : null}
                      {plan.features.length > 0 ? (
                        <p className="mt-1 text-xs text-[var(--lp-ink-muted)]">
                          Features: {plan.features.join(", ")}
                        </p>
                      ) : null}
                    </li>
                  ))}
                </ul>
              )}
            </Surface>

            <Surface className="overflow-hidden p-0">
              <div className="border-b border-[var(--lp-border)] px-5 py-4">
                <h2 className="text-lg font-semibold">Subscriptions</h2>
                <p className="text-sm text-[var(--lp-ink-muted)]">
                  {subscriptions.length} subscriptions
                </p>
              </div>
              {subscriptions.length === 0 ? (
                <div className="p-5">
                  <EmptyState
                    dense
                    title="No subscriptions"
                    description="Tenant subscriptions will appear here."
                  />
                </div>
              ) : (
                <ul className="divide-y divide-[var(--lp-border)]">
                  {subscriptions.map((subscription) => (
                    <li key={subscription.id} className="px-5 py-4">
                      <p className="font-medium">{subscription.planCode}</p>
                      <p className="text-sm text-[var(--lp-ink-muted)]">
                        Org {subscription.organizationId} · {subscription.status}
                      </p>
                      {subscription.currentPeriodEnd ? (
                        <p className="mt-1 text-xs text-[var(--lp-ink-muted)]">
                          Period ends{" "}
                          {new Date(subscription.currentPeriodEnd).toLocaleDateString()}
                        </p>
                      ) : null}
                    </li>
                  ))}
                </ul>
              )}
            </Surface>
          </section>
        </Reveal>
      </div>
    </PlatformShell>
  );
}
