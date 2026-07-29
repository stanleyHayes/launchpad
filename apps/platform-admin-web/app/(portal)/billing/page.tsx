"use client";

import { useEffect, useState, useTransition, type SyntheticEvent } from "react";
import { useRouter } from "next/navigation";
import type { Coupon, Invoice, Plan, Subscription } from "@launchpad/api-client";
import { ApiError } from "@launchpad/api-client";
import {
  Select,
  EmptyState,
  PageHeader,
  Reveal,
  Surface,
  ToggleSwitch,
} from "@launchpad/ui";
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

const planFeatureOptions = [
  { code: "core_onboarding", label: "Core onboarding", description: "Journeys, assignments, and employee progress" },
  { code: "analytics", label: "Analytics", description: "Cohorts, completion, and onboarding insights" },
  { code: "sso", label: "SSO & SCIM", description: "Enterprise identity and directory provisioning" },
  { code: "support_sla", label: "SLA support", description: "Priority support and response commitments" },
] as const;

function FeaturePicker({
  value,
  onChange,
}: {
  value: string[];
  onChange: (features: string[]) => void;
}) {
  function toggle(code: string) {
    onChange(
      value.includes(code)
        ? value.filter((feature) => feature !== code)
        : [...value, code],
    );
  }

  return (
    <fieldset className="sm:col-span-2">
      <legend className="text-sm font-semibold">Plan features</legend>
      <p className="mt-1 text-xs text-[var(--lp-ink-muted)]">
        Select the capabilities included in this plan.
      </p>
      {value.length > 0 ? (
        <div className="mt-3 flex flex-wrap gap-2" aria-label="Selected plan features">
          {value.map((code) => {
            const option = planFeatureOptions.find((item) => item.code === code);
            return (
              <button
                key={code}
                type="button"
                className="inline-flex items-center gap-2 rounded-[var(--lp-radius-input)] bg-[var(--lp-accent)] px-3 py-2 text-xs font-semibold text-white shadow-[0_8px_18px_rgba(22,56,110,0.2)]"
                aria-label={`Remove ${option?.label ?? code}`}
                onClick={() => toggle(code)}
              >
                {option?.label ?? code}
                <span aria-hidden="true">×</span>
              </button>
            );
          })}
        </div>
      ) : (
        <p className="mt-3 rounded-[var(--lp-radius-input)] bg-[var(--lp-paper)] px-3 py-2 text-xs text-[var(--lp-ink-muted)] shadow-[var(--lp-shadow-inset)]">
          No features selected
        </p>
      )}
      <div className="mt-3 grid gap-2 sm:grid-cols-2">
        {planFeatureOptions.map((option) => {
          const selected = value.includes(option.code);
          return (
            <button
              key={option.code}
              type="button"
              aria-pressed={selected}
              className={`rounded-[var(--lp-radius-input)] border p-3 text-left transition ${
                selected
                  ? "border-[var(--lp-accent)] bg-[color-mix(in_srgb,var(--lp-accent)_8%,var(--lp-paper))]"
                  : "border-[var(--lp-border)] bg-[var(--lp-paper-elevated)] hover:border-[var(--lp-accent)]"
              }`}
              onClick={() => toggle(option.code)}
            >
              <span className="block text-sm font-semibold">{option.label}</span>
              <span className="mt-1 block text-xs leading-5 text-[var(--lp-ink-muted)]">
                {option.description}
              </span>
            </button>
          );
        })}
      </div>
    </fieldset>
  );
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
  const [createActive, setCreateActive] = useState(true);
  const [createFeatures, setCreateFeatures] = useState<string[]>(["core_onboarding"]);
  const [editingPlan, setEditingPlan] = useState<Plan | null>(null);
  const [editingActive, setEditingActive] = useState(true);
  const [editingFeatures, setEditingFeatures] = useState<string[]>([]);

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

  function onCreatePlan(event: SyntheticEvent<HTMLFormElement>) {
    event.preventDefault();
    setError(null);
    setMessage(null);
    const formEl = event.currentTarget;
    const form = new FormData(formEl);
    startTransition(() => {
      void getClient().createPlatformPlan({
        code: formString(form, "code"),
        name: formString(form, "name"),
        description: formString(form, "description"),
        priceMonthlyCents: Math.round(Number(formString(form, "monthlyPrice") || "0") * 100),
        currency: formString(form, "currency") || "USD",
        features: createFeatures,
        active: createActive,
      }).then((plan) => {
        formEl.reset();
        setCreateActive(true);
        setCreateFeatures(["core_onboarding"]);
        setMessage(`${plan.name} plan created`);
        reload();
      }).catch((err: unknown) => {
        setError(err instanceof ApiError ? err.message : "Unable to create plan");
      });
    });
  }

  function onUpdatePlan(event: SyntheticEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!editingPlan) return;
    setError(null);
    setMessage(null);
    const form = new FormData(event.currentTarget);
    startTransition(() => {
      void getClient().updatePlatformPlan(editingPlan.code, {
        name: formString(form, "name"),
        description: formString(form, "description"),
        priceMonthlyCents: Math.round(Number(formString(form, "monthlyPrice") || "0") * 100),
        currency: formString(form, "currency") || "USD",
        features: editingFeatures,
        active: editingActive,
      }).then((plan) => {
        setEditingPlan(null);
        setMessage(`${plan.name} plan updated`);
        reload();
      }).catch((err: unknown) => {
        setError(err instanceof ApiError ? err.message : "Unable to update plan");
      });
    });
  }

  function beginEditing(plan: Plan) {
    setEditingPlan(plan);
    setEditingActive(plan.active);
    setEditingFeatures(plan.features);
    setError(null);
    setMessage(null);
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
            <div className="flex flex-wrap items-start justify-between gap-3">
              <div>
                <p className="lp-eyebrow">Plan catalog</p>
                <h2 className="mt-2 text-xl font-semibold">Create a subscription plan</h2>
                <p className="mt-1 text-sm text-[var(--lp-ink-muted)]">
                  Define the public price and feature codes used by plan-gated capabilities.
                </p>
              </div>
              <span className="rounded-[var(--lp-radius-input)] bg-[var(--lp-paper)] px-3 py-2 text-xs font-semibold text-[var(--lp-ink-muted)] shadow-[var(--lp-shadow-inset)]">
                {plans.filter((plan) => plan.active).length} active
              </span>
            </div>
            <form
              className="mt-5 grid gap-3 md:grid-cols-2 xl:grid-cols-4"
              onSubmit={onCreatePlan}
            >
              <label className="text-sm font-semibold">
                Plan code
                <input
                  className="lp-input mt-1.5"
                  name="code"
                  placeholder="scale"
                  pattern="[a-z0-9_-]+"
                  title="Use lowercase letters, numbers, underscores, or hyphens"
                  required
                />
              </label>
              <label className="text-sm font-semibold">
                Display name
                <input className="lp-input mt-1.5" name="name" placeholder="Scale" required />
              </label>
              <label className="text-sm font-semibold">
                Monthly price
                <input
                  className="lp-input mt-1.5"
                  name="monthlyPrice"
                  type="number"
                  min="0"
                  step="0.01"
                  placeholder="249.00"
                  required
                />
              </label>
              <label className="text-sm font-semibold">
                Currency
                <Select className="mt-1.5" name="currency" defaultValue="USD">
                  <option value="USD">USD — US dollar</option>
                  <option value="GHS">GHS — Ghana cedi</option>
                  <option value="GBP">GBP — Pound sterling</option>
                  <option value="EUR">EUR — Euro</option>
                  <option value="NGN">NGN — Nigerian naira</option>
                </Select>
              </label>
              <label className="text-sm font-semibold md:col-span-2">
                Description
                <input
                  className="lp-input mt-1.5"
                  name="description"
                  placeholder="For established teams scaling onboarding operations"
                />
              </label>
              <FeaturePicker value={createFeatures} onChange={setCreateFeatures} />
              <div className="flex items-center gap-3 md:col-span-2">
                <ToggleSwitch
                  checked={createActive}
                  onChange={setCreateActive}
                  label="Make new plan active"
                />
                <span className="text-sm font-semibold">
                  {createActive ? "Available for subscriptions" : "Save as inactive"}
                </span>
              </div>
              <div className="flex items-center justify-end md:col-span-2">
                <button className="lp-btn lp-btn--primary" disabled={pending}>
                  {pending ? "Saving…" : "Create plan"}
                </button>
              </div>
            </form>
          </Surface>
        </Reveal>

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
                        <div className="flex items-center gap-2">
                          <p className="text-sm font-medium">
                            {formatPrice(plan.priceMonthlyCents, plan.currency)}/mo
                          </p>
                          <button
                            type="button"
                            className="lp-btn lp-btn--quiet !px-3 !py-1.5"
                            onClick={() => beginEditing(plan)}
                          >
                            Edit
                          </button>
                        </div>
                      </div>
                      {plan.description ? (
                        <p className="mt-2 text-sm text-[var(--lp-ink-muted)]">{plan.description}</p>
                      ) : null}
                      {plan.features.length > 0 ? (
                        <p className="mt-1 text-xs text-[var(--lp-ink-muted)]">
                          Features: {plan.features.join(", ")}
                        </p>
                      ) : null}
                      {editingPlan?.code === plan.code ? (
                        <form
                          key={plan.updatedAt}
                          className="mt-4 grid gap-3 rounded-[var(--lp-radius)] bg-[var(--lp-paper)] p-4 shadow-[var(--lp-shadow-inset)] sm:grid-cols-2"
                          onSubmit={onUpdatePlan}
                        >
                          <label className="text-sm font-semibold">
                            Display name
                            <input
                              className="lp-input mt-1.5"
                              name="name"
                              defaultValue={plan.name}
                              required
                            />
                          </label>
                          <label className="text-sm font-semibold">
                            Monthly price
                            <input
                              className="lp-input mt-1.5"
                              name="monthlyPrice"
                              type="number"
                              min="0"
                              step="0.01"
                              defaultValue={(plan.priceMonthlyCents / 100).toFixed(2)}
                              required
                            />
                          </label>
                          <label className="text-sm font-semibold">
                            Currency
                            <Select
                              className="mt-1.5"
                              name="currency"
                              defaultValue={plan.currency}
                            >
                              <option value="USD">USD — US dollar</option>
                              <option value="GHS">GHS — Ghana cedi</option>
                              <option value="GBP">GBP — Pound sterling</option>
                              <option value="EUR">EUR — Euro</option>
                              <option value="NGN">NGN — Nigerian naira</option>
                            </Select>
                          </label>
                          <label className="text-sm font-semibold sm:col-span-2">
                            Description
                            <textarea
                              className="lp-input mt-1.5 min-h-20"
                              name="description"
                              defaultValue={plan.description}
                            />
                          </label>
                          <FeaturePicker
                            value={editingFeatures}
                            onChange={setEditingFeatures}
                          />
                          <div className="flex flex-wrap items-center justify-between gap-4 sm:col-span-2">
                            <div className="flex min-w-0 items-center gap-3">
                              <ToggleSwitch
                                checked={editingActive}
                                onChange={setEditingActive}
                                label={`${plan.name} active status`}
                              />
                              <span className="whitespace-nowrap text-sm font-semibold">
                                {editingActive ? "Active plan" : "Inactive plan"}
                              </span>
                            </div>
                            <div className="flex shrink-0 items-center gap-2">
                              <button
                                type="button"
                                className="lp-btn lp-btn--quiet whitespace-nowrap"
                                onClick={() => setEditingPlan(null)}
                              >
                                Cancel
                              </button>
                              <button
                                className="lp-btn lp-btn--primary whitespace-nowrap"
                                disabled={pending}
                              >
                                {pending ? "Saving…" : "Save changes"}
                              </button>
                            </div>
                          </div>
                        </form>
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
