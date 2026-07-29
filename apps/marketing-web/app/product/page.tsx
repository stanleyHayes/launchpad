import Link from "next/link";
import type { Metadata } from "next";
import { Container } from "@launchpad/ui";
import { buildMetadata } from "../../lib/seo";
import { CapabilityVisual } from "../capability-visual";
import { FeatureCard } from "../feature-card";
import { Icon, type IconName } from "../ui-icon";
import { ProductPreview } from "../product-preview";
import { SiteFooter } from "../site-footer";
import { SiteHeader } from "../site-header";

export const metadata: Metadata = buildMetadata({
  title: "Product — LaunchPad",
  description:
    "Guided onboarding journeys, manager approvals, grounded AI answers, and enterprise-grade provisioning in one platform.",
  path: "/product",
});

const capabilities: {
  icon: IconName;
  eyebrow: string;
  title: string;
  body: string;
  points: string[];
}[] = [
  {
    icon: "workflow",
    eyebrow: "Journeys",
    title: "Compose the journey once, reuse it for every hire",
    body: "Build templates from document, access, training, quiz, and approval steps. Publish a version, assign it by role, and every new hire gets the same deliberate ramp — with due dates computed from their start date.",
    points: [
      "Draft, publish, and version templates safely",
      "Step-level progress with manager visibility",
      "Automatic assignment by role and department",
    ],
  },
  {
    icon: "eye",
    eyebrow: "Approvals",
    title: "Approvals land with the person who can say yes",
    body: "Practical work routes to the hire's manager for sign-off — not to whoever created the paperwork. Managers see every pending decision and every stalled step across their team without asking for status.",
    points: [
      "Manager-routed approval steps",
      "Org-wide progress and completion analytics",
      "Audit trail on every decision",
    ],
  },
  {
    icon: "sparkles",
    eyebrow: "Assistant",
    title: "Answers from your sources, with receipts",
    body: "The assistant only answers from knowledge your team has approved and indexed. Every reply cites the exact passages it used — and when nothing supports an answer, it says so instead of guessing.",
    points: [
      "Draft → approved → indexed knowledge lifecycle",
      "Citations built from retrieved sources, never model memory",
      "Refuses to answer without supporting evidence",
    ],
  },
];

const included: { icon: IconName; title: string; body: string }[] = [
  {
    icon: "users",
    title: "Employee portal",
    body: "A calm, personal checklist for every hire — steps, due dates, and submissions in one place.",
  },
  {
    icon: "plug",
    title: "HRIS & IdP sync",
    body: "Import your directory from BambooHR and let people sign in with Okta or Microsoft Entra.",
  },
  {
    icon: "shield",
    title: "SSO & SCIM",
    body: "OIDC single sign-on and SCIM 2.0 provisioning with expiring, auditable tenant tokens.",
  },
  {
    icon: "chart",
    title: "Onboarding analytics",
    body: "Completion, time-to-productivity, and cohort trends for managers and leadership.",
  },
  {
    icon: "clock",
    title: "Notifications",
    body: "In-app alerts plus Slack and Teams webhooks, delivered when steps are assigned or approved.",
  },
  {
    icon: "check",
    title: "Quiz checkpoints",
    body: "Gate progress on comprehension with server-scored quiz steps inside any journey.",
  },
];

export default function ProductPage() {
  return (
    <main>
      {/* Hero ------------------------------------------------------------- */}
      <section className="lp-hero relative">
        <SiteHeader variant="hero" />
        <Container className="grid items-center gap-14 pb-24 pt-36 lg:grid-cols-[1.05fr_0.95fr] lg:pt-40">
          <div>
            <p className="lp-rise lp-eyebrow lp-eyebrow--on-dark">Product</p>
            <h1
              className="lp-rise mt-6 max-w-xl text-4xl font-semibold leading-[1.08] tracking-tight md:text-6xl"
              style={{ fontFamily: "var(--lp-font-display)" }}
            >
              One journey from offer letter to first win.
            </h1>
            <p className="lp-rise-delay mt-6 max-w-lg text-lg leading-8 text-white/80">
              LaunchPad turns scattered onboarding checklists into structured
              journeys your managers assign, your hires complete, and your
              leadership can finally measure.
            </p>
            <div className="lp-rise-delay-2 mt-9 flex flex-wrap items-center gap-3">
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
          <div className="lp-rise-delay relative lg:pl-6">
            <ProductPreview />
          </div>
        </Container>
      </section>

      {/* Capability deep-dives -------------------------------------------- */}
      <section className="py-24">
        <Container>
          <div className="max-w-2xl">
            <p className="lp-eyebrow">Capabilities</p>
            <h2
              className="mt-4 text-3xl font-semibold tracking-tight text-[var(--lp-ink)] md:text-4xl"
              style={{ fontFamily: "var(--lp-font-display)" }}
            >
              Built for the whole ramp, not just day one
            </h2>
          </div>

          <div className="mt-16 space-y-20">
            {capabilities.map((capability, index) => (
              <div
                key={capability.title}
                className={`grid items-center gap-10 lg:grid-cols-2 ${
                  index % 2 === 1 ? "lg:[&>*:first-child]:order-2" : ""
                }`}
              >
                <div>
                  <p className="lp-eyebrow">{capability.eyebrow}</p>
                  <h3
                    className="mt-4 text-2xl font-semibold tracking-tight text-[var(--lp-ink)] md:text-3xl"
                    style={{ fontFamily: "var(--lp-font-display)" }}
                  >
                    {capability.title}
                  </h3>
                  <p className="mt-4 max-w-lg leading-7 text-[var(--lp-ink-muted)]">
                    {capability.body}
                  </p>
                  <ul className="mt-6 space-y-3">
                    {capability.points.map((point) => (
                      <li key={point} className="flex items-start gap-3 text-sm text-[var(--lp-ink)]">
                        <span className="mt-0.5 grid h-5 w-5 shrink-0 place-items-center rounded-full bg-[var(--lp-success)]/12 text-[var(--lp-success)]">
                          <Icon name="check" className="h-3 w-3" />
                        </span>
                        {point}
                      </li>
                    ))}
                  </ul>
                </div>
                <CapabilityVisual icon={capability.icon} />
              </div>
            ))}
          </div>
        </Container>
      </section>

      {/* Everything included ---------------------------------------------- */}
      <section className="bg-[var(--lp-paper-elevated)] py-24">
        <Container>
          <div className="mx-auto max-w-2xl text-center">
            <p className="lp-eyebrow justify-center">Everything included</p>
            <h2
              className="mt-4 text-3xl font-semibold tracking-tight text-[var(--lp-ink)] md:text-4xl"
              style={{ fontFamily: "var(--lp-font-display)" }}
            >
              The platform around the journey
            </h2>
          </div>
          <div className="mt-14 grid gap-6 md:grid-cols-2 lg:grid-cols-3">
            {included.map((item) => (
              <FeatureCard key={item.title} icon={item.icon} title={item.title} body={item.body} />
            ))}
          </div>
        </Container>
      </section>

      {/* Security band ----------------------------------------------------- */}
      <section className="py-6">
        <Container>
          <div
            className="overflow-hidden rounded-3xl px-8 py-14 text-white md:px-14"
            style={{
              background:
                "radial-gradient(ellipse 60% 80% at 100% 0%, rgba(46,91,176,0.55), transparent 60%), linear-gradient(160deg, #0f1e3a, #16386e)",
            }}
          >
            <div className="grid items-center gap-10 lg:grid-cols-[1.1fr_0.9fr]">
              <div>
                <p className="lp-eyebrow lp-eyebrow--on-dark">Security</p>
                <h2
                  className="mt-4 text-3xl font-semibold tracking-tight md:text-4xl"
                  style={{ fontFamily: "var(--lp-font-display)" }}
                >
                  Enterprise controls from the first employee
                </h2>
                <p className="mt-4 max-w-lg leading-8 text-white/75">
                  Tenant isolation on every query, OIDC single sign-on, SCIM
                  provisioning with expiring tokens, encrypted secrets at rest,
                  and an audit event on every privileged action.
                </p>
              </div>
              <ul className="lp-dark-scope grid gap-4 sm:grid-cols-2">
                {["OIDC SSO", "SCIM 2.0", "Audit trail", "Tenant isolation", "Encrypted secrets", "Rate limiting"].map(
                  (item) => (
                    <li
                      key={item}
                      className="lp-dark-tile flex items-center gap-3 px-4 py-3 text-sm font-medium"
                    >
                      <span className="lp-icon-chip lp-icon-chip--sm">
                        <Icon name="shield" className="h-4 w-4" />
                      </span>
                      {item}
                    </li>
                  ),
                )}
              </ul>
            </div>
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
              See it running against your own onboarding
            </h2>
            <p className="mx-auto mt-4 max-w-xl text-lg text-white/75">
              Start free today, or book a walkthrough tailored to your team.
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
