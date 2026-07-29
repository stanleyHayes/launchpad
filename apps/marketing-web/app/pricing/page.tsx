import Link from "next/link";
import type { Metadata } from "next";
import { Container } from "@launchpad/ui";
import { buildMetadata } from "../../lib/seo";
import { Icon } from "../ui-icon";
import { SiteFooter } from "../site-footer";
import { SiteHeader } from "../site-header";

export const metadata: Metadata = buildMetadata({
  title: "Pricing — LaunchPad",
  description:
    "Start free, grow into analytics, and scale to enterprise SSO, SCIM, and SLA support when you need it.",
  path: "/pricing",
});

// Mirrors the plans seeded by the API (internal/billing) — Starter free,
// Growth $99/mo, Enterprise custom.
const tiers = [
  {
    name: "Starter",
    price: "$0",
    period: "forever",
    tagline: "For small teams getting their first journey live.",
    cta: { href: "/signup", label: "Start free" },
    featured: false,
    features: [
      "Journey templates & assignments",
      "Employee portal",
      "Manager approvals",
      "In-app notifications",
      "Up to 25 employees",
    ],
  },
  {
    name: "Growth",
    price: "$99",
    period: "per month",
    tagline: "For HR teams measuring and improving the ramp.",
    cta: { href: "/signup", label: "Start free trial" },
    featured: true,
    features: [
      "Everything in Starter",
      "Onboarding analytics & cohort trends",
      "Slack and Teams notifications",
      "AI assistant with cited answers",
      "Unlimited employees",
    ],
  },
  {
    name: "Enterprise",
    price: "Custom",
    period: "annual agreement",
    tagline: "For organizations with security and compliance bar.",
    cta: { href: "/demo", label: "Talk to us" },
    featured: false,
    features: [
      "Everything in Growth",
      "SSO (OIDC) & SCIM 2.0 provisioning",
      "Custom feature flags",
      "SLA-backed support",
      "Security review assistance",
    ],
  },
];

const faqs = [
  {
    question: "What counts as an employee?",
    answer:
      "Anyone with an active assignment in your organization. Deactivated and offboarded accounts don't count toward your plan.",
  },
  {
    question: "Can I switch plans later?",
    answer:
      "Yes. Upgrades apply immediately and advanced features unlock as soon as your subscription changes — no migration needed.",
  },
  {
    question: "Is the free trial really free?",
    answer:
      "Every new organization starts on a trial with full Growth features. No credit card required; you choose a plan when the trial ends.",
  },
];

export default function PricingPage() {
  return (
    <main className="relative">
      <SiteHeader variant="light" />

      {/* Header ------------------------------------------------------------ */}
      <section className="pb-16 pt-36">
        <Container className="max-w-3xl text-center">
          <p className="lp-rise lp-eyebrow justify-center">Pricing</p>
          <h1
            className="lp-rise mt-5 text-4xl font-semibold tracking-tight text-[var(--lp-ink)] md:text-5xl"
            style={{ fontFamily: "var(--lp-font-display)" }}
          >
            Pricing that grows with your program
          </h1>
          <p className="lp-rise-delay mt-5 text-lg leading-8 text-[var(--lp-ink-muted)]">
            Start free with your first journey, add analytics when you want to
            measure, and bring on enterprise controls when security asks.
          </p>
        </Container>
      </section>

      {/* Tiers ------------------------------------------------------------- */}
      <section className="pb-24">
        <Container>
          <div className="grid items-stretch gap-6 lg:grid-cols-3">
            {tiers.map((tier) => (
              <div
                key={tier.name}
                className={`lp-card relative flex flex-col p-8 ${
                  tier.featured ? "ring-2 ring-[var(--lp-brand)]" : ""
                }`}
              >
                {tier.featured ? (
                  <span className="absolute -top-3 left-8 bg-[var(--lp-brand)] px-2 py-1 text-[0.7rem] font-bold uppercase tracking-[0.14em] text-white">
                    Most popular
                  </span>
                ) : null}
                <h2
                  className="text-xl font-semibold text-[var(--lp-ink)]"
                  style={{ fontFamily: "var(--lp-font-display)" }}
                >
                  {tier.name}
                </h2>
                <p className="mt-1 text-sm text-[var(--lp-ink-muted)]">{tier.tagline}</p>
                <p className="mt-6 flex items-baseline gap-2">
                  <span
                    className="text-4xl font-semibold tracking-tight text-[var(--lp-ink)]"
                    style={{ fontFamily: "var(--lp-font-display)" }}
                  >
                    {tier.price}
                  </span>
                  <span className="text-sm text-[var(--lp-ink-muted)]">{tier.period}</span>
                </p>
                <ul className="mt-8 flex-1 space-y-3 border-t border-[var(--lp-border)] pt-8">
                  {tier.features.map((feature) => (
                    <li key={feature} className="flex items-start gap-3 text-sm text-[var(--lp-ink)]">
                      <span className="mt-0.5 grid h-5 w-5 shrink-0 place-items-center rounded-full bg-[var(--lp-success)]/12 text-[var(--lp-success)]">
                        <Icon name="check" className="h-3 w-3" />
                      </span>
                      {feature}
                    </li>
                  ))}
                </ul>
                <Link href={tier.cta.href} className="mt-8" style={{ textDecoration: "none" }}>
                  {tier.featured ? (
                    <span className="lp-btn lp-btn--primary w-full">{tier.cta.label}</span>
                  ) : (
                    <span className="lp-btn lp-btn--secondary w-full">
                      {tier.cta.label}
                    </span>
                  )}
                </Link>
              </div>
            ))}
          </div>
          <p className="mt-8 text-center text-sm text-[var(--lp-ink-muted)]">
            All prices in USD. Every plan includes unlimited journeys, approvals,
            and notifications.
          </p>
        </Container>
      </section>

      {/* Fine print -------------------------------------------------------- */}
      <section className="pb-24">
        <Container className="max-w-3xl">
          <div className="lp-card p-8">
            <p className="lp-eyebrow">The fine print, in plain words</p>
            <ul className="mt-6 space-y-3 text-sm leading-6 text-[var(--lp-ink-muted)]">
              <li>All prices are in USD; local taxes may apply where required.</li>
              <li>Cancel anytime — you keep access until the end of your paid period.</li>
              <li>Your trial converts to a paid plan only when you choose one. No credit card, no surprise charges.</li>
              <li>Enterprise agreements include a security review with your team before rollout.</li>
            </ul>
          </div>
        </Container>
      </section>

      {/* FAQ --------------------------------------------------------------- */}
      <section className="bg-[var(--lp-paper-elevated)] py-24">
        <Container>
          <div className="max-w-2xl">
            <p className="lp-eyebrow">Questions</p>
            <h2
              className="mt-4 text-3xl font-semibold tracking-tight text-[var(--lp-ink)] md:text-4xl"
              style={{ fontFamily: "var(--lp-font-display)" }}
            >
              Asked before you did
            </h2>
          </div>
          <div className="mt-12 grid gap-10 md:grid-cols-3">
            {faqs.map((faq) => (
              <div key={faq.question}>
                <h3 className="text-base font-semibold text-[var(--lp-ink)]">{faq.question}</h3>
                <p className="mt-3 text-sm leading-7 text-[var(--lp-ink-muted)]">{faq.answer}</p>
              </div>
            ))}
          </div>
        </Container>
      </section>

      {/* CTA band ---------------------------------------------------------- */}
      <section className="py-24">
        <Container>
          <div className="lp-hero relative overflow-hidden rounded-3xl px-8 py-16 text-center">
            <h2
              className="mx-auto max-w-2xl text-3xl font-semibold tracking-tight md:text-4xl"
              style={{ fontFamily: "var(--lp-font-display)" }}
            >
              Your first journey can be live today
            </h2>
            <p className="mx-auto mt-4 max-w-xl text-lg text-white/75">
              Start free — no credit card, no sales call required.
            </p>
            <div className="mt-8 flex flex-wrap justify-center gap-3">
              <Link href="/signup" style={{ textDecoration: "none" }}>
                <span className="lp-btn lp-btn--primary">
                  Start free trial
                  <Icon name="arrow-right" className="h-4 w-4" />
                </span>
              </Link>
              <Link href="/demo" style={{ textDecoration: "none" }}>
                <span className="lp-btn lp-btn--on-dark">Book a demo</span>
              </Link>
            </div>
          </div>
        </Container>
      </section>

      <SiteFooter />
    </main>
  );
}
