"use client";

import { useEffect, useState, useTransition, type SyntheticEvent } from "react";
import { useRouter } from "next/navigation";
import type { Coupon, Invoice, Plan, Subscription } from "@launchpad/api-client";
import { ApiError } from "@launchpad/api-client";
import { Select, EmptyState, PageHeader, Reveal, Surface } from "@launchpad/ui";
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
  const [invoices, setInvoices] = useState<Invoice[]>([]);
  const [coupons, setCoupons] = useState<Coupon[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [message, setMessage] = useState<string | null>(null);

  function reload(isStale?: () => boolean) {
    startTransition(() => {
      void (async () => {
        try {
          const client = getClient();
          const [planItems, subscriptionItems, invoiceItems, couponItems] = await Promise.all([
            client.listPlatformPlans(),
            client.listPlatformSubscriptions(),
            client.listPlatformInvoices(),
            client.listPlatformCoupons(),
          ]);
          if (isStale?.()) return;
          setPlans(planItems);
          setSubscriptions(subscriptionItems);
          setInvoices(invoiceItems);
          setCoupons(couponItems);
        } catch (err) {
          if (isStale?.()) return;
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

  function onCreateCoupon(event: SyntheticEvent<HTMLFormElement>) {
    event.preventDefault();
    const form = new FormData(event.currentTarget);
    const formEl = event.currentTarget;
    startTransition(() => {
      void getClient().createPlatformCoupon({
        code: formString(form, "code"),
        percentOffBasisPoints: Number(formString(form, "percentOffBasisPoints") || "0"),
        maxRedemptions: Number(formString(form, "maxRedemptions") || "0"),
        expiresAt: formString(form, "expiresAt") ? new Date(formString(form, "expiresAt")).toISOString() : undefined,
      }).then(() => {
        formEl.reset();
        reload();
      }).catch((err: unknown) => {
        setError(err instanceof ApiError ? err.message : "Unable to create coupon");
      });
    });
  }

  async function adjust(invoice: Invoice) {
    const couponCode = window.prompt("Coupon code (optional)", invoice.couponCode ?? "") ?? "";
    const tax = window.prompt("Tax rate in basis points (1250 = 12.5%)", "0");
    if (tax === null) return;
    startTransition(() => {
      void getClient().adjustPlatformInvoice(invoice.id, {
        couponCode, taxRateBasisPoints: Number(tax),
      }).then(() => reload()).catch((err: unknown) => {
        setError(err instanceof ApiError ? err.message : "Unable to adjust invoice");
      });
    });
  }

  async function refund(invoice: Invoice) {
    const amount = window.prompt("Refund amount in cents", String(invoice.amountCents));
    const reason = window.prompt("Refund reason");
    if (!amount || !reason) return;
    startTransition(() => {
      void getClient().refundPlatformInvoice(invoice.id, {
        amountCents: Number(amount), reason,
      }).then(() => reload()).catch((err: unknown) => {
        setError(err instanceof ApiError ? err.message : "Unable to refund invoice");
      });
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

  function onSetSubscription(event: SyntheticEvent<HTMLFormElement>) {
    event.preventDefault();
    setError(null);
    setMessage(null);
    const formEl = event.currentTarget;
    const form = new FormData(formEl);
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
          formEl.reset();
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
              <Select className="lp-input" name="planCode" required defaultValue="">
                <option value="" disabled>
                  Select plan
                </option>
                {plans.map((plan) => (
                  <option key={plan.code} value={plan.code}>
                    {plan.name} ({plan.code})
                  </option>
                ))}
              </Select>
              <Select className="lp-input" name="status" defaultValue="">
                <option value="">Default status</option>
                <option value="trialing">Trialing</option>
                <option value="active">Active</option>
                <option value="past_due">Past due</option>
                <option value="canceled">Canceled</option>
              </Select>
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

        <Reveal delay={3}>
          <Surface className="overflow-hidden p-0">
            <div className="border-b border-[var(--lp-border)] px-5 py-4">
              <h2 className="text-lg font-semibold">Invoices & collections</h2>
              <p className="text-sm text-[var(--lp-ink-muted)]">{invoices.length} invoices</p>
            </div>
            <ul className="divide-y divide-[var(--lp-border)]">
              {invoices.map((invoice) => (
                <li key={invoice.id} className="flex flex-wrap items-center justify-between gap-3 px-5 py-4">
                  <div>
                    <p className="font-medium">{invoice.number} · {formatPrice(invoice.amountCents, invoice.currency)}</p>
                    <p className="text-sm text-[var(--lp-ink-muted)]">
                      Org {invoice.organizationId} · {invoice.status} · {invoice.dunningAttempts} collection attempts
                    </p>
                    {invoice.couponCode ? <p className="text-xs text-[var(--lp-ink-muted)]">Coupon {invoice.couponCode}</p> : null}
                  </div>
                  <div className="flex gap-2">
                    {invoice.status === "open" ? <button className="lp-btn lp-btn--secondary" onClick={() => { void adjust(invoice); }}>Tax / coupon</button> : null}
                    {invoice.status === "paid" ? <button className="lp-btn lp-btn--secondary" onClick={() => { void refund(invoice); }}>Refund</button> : null}
                  </div>
                </li>
              ))}
            </ul>
          </Surface>
        </Reveal>

        <Reveal delay={3}>
          <Surface>
            <h2 className="text-lg font-semibold">Coupon catalog</h2>
            <form className="mt-4 grid gap-3 md:grid-cols-4" onSubmit={onCreateCoupon}>
              <input className="lp-input" name="code" placeholder="Code" required />
              <input className="lp-input" name="percentOffBasisPoints" type="number" min="1" max="10000" placeholder="Basis points" required />
              <input className="lp-input" name="maxRedemptions" type="number" min="0" placeholder="Max uses (0 unlimited)" />
              <input className="lp-input" name="expiresAt" type="datetime-local" />
              <button className="lp-btn lp-btn--secondary justify-self-start" disabled={pending}>Create coupon</button>
            </form>
            <ul className="mt-5 divide-y divide-[var(--lp-border)]">
              {coupons.map((coupon) => (
                <li key={coupon.code} className="flex justify-between gap-3 py-3 text-sm">
                  <span className="font-medium">{coupon.code} · {(coupon.percentOffBasisPoints / 100).toFixed(2)}% off</span>
                  <span className="text-[var(--lp-ink-muted)]">{coupon.redemptionCount}/{coupon.maxRedemptions || "∞"} uses</span>
                </li>
              ))}
            </ul>
          </Surface>
        </Reveal>
      </div>
      );
}
