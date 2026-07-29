import Link from "next/link";
import type { Metadata } from "next";
import { Container, LogoWatermark } from "@launchpad/ui";
import { buildMetadata } from "../lib/seo";
import { FeatureCard } from "./feature-card";
import { Icon, type IconName } from "./ui-icon";
import { ProductPreview } from "./product-preview";
import { ProductEvidence } from "./product-evidence";
import { SiteFooter } from "./site-footer";
import { SiteHeader } from "./site-header";

export const metadata: Metadata = buildMetadata({
  title: "LaunchPad — Employee onboarding, orchestrated",
  description:
    "Build guided onboarding journeys, automate setup, and measure time-to-productivity.",
  path: "/",
});

const logos = ["Northwind", "Acme Corp", "Globex", "Umbra", "Initech", "Hooli"];

const features: { icon: IconName; title: string; body: string }[] = [
  {
    icon: "workflow",
    title: "Workflow orchestration",
    body: "Pre-boarding, access, training, meetings, and approvals on one timeline — no more scattered checklists.",
  },
  {
    icon: "eye",
    title: "Manager visibility",
    body: "Spot blockers early and approve practical work without chasing status updates over email.",
  },
  {
    icon: "sparkles",
    title: "Grounded AI assistant",
    body: "Answers drawn from your approved company sources — every reply cites exactly where it came from.",
  },
  {
    icon: "shield",
    title: "Enterprise security",
    body: "SSO, SCIM provisioning, tenant isolation, and an audit trail on every privileged action.",
  },
  {
    icon: "plug",
    title: "HRIS & IdP sync",
    body: "Pull your directory from BambooHR and let employees sign in through Okta or Microsoft Entra.",
  },
  {
    icon: "chart",
    title: "Onboarding analytics",
    body: "Track time-to-productivity and step completion across every role, cohort, and location.",
  },
];

const steps = [
  {
    title: "Design the journey",
    body: "Compose reusable templates from pre-boarding, access, training, and approval steps.",
  },
  {
    title: "Assign & automate",
    body: "Roll journeys out by role — setup tasks and notifications fire automatically on schedule.",
  },
  {
    title: "Measure outcomes",
    body: "Watch progress live and prove time-to-productivity with cohort-level analytics.",
  },
];

const stats = [
  { value: "-41%", label: "Time to first contribution" },
  { value: "3.2×", label: "Faster access provisioning" },
  { value: "98%", label: "Onboarding step completion" },
  { value: "12k+", label: "New hires onboarded" },
];

export default function HomePage() {
  return (
    <main>
      {/* Hero -------------------------------------------------------------- */}
      <section className="lp-hero relative">
        <LogoWatermark onDark className="-right-24 top-16 size-[30rem] rotate-[-9deg]" />
        <SiteHeader variant="hero" />
        <Container className="grid items-center gap-14 pb-24 pt-36 lg:grid-cols-[1.05fr_0.95fr] lg:pt-40">
          <div>
            <p className="lp-rise lp-eyebrow lp-eyebrow--on-dark">
              Employee onboarding platform
            </p>
            <h1
              className="lp-rise mt-6 max-w-xl text-4xl font-semibold leading-[1.08] tracking-tight md:text-6xl"
              style={{ fontFamily: "var(--lp-font-display)" }}
            >
              Every new hire, confident and productive faster.
            </h1>
            <p className="lp-rise-delay mt-6 max-w-lg text-lg leading-8 text-white/80">
              Guided journeys, automated setup, role-based training, and a grounded
              AI assistant — one secure platform that turns day one into real output.
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
            <p className="lp-rise-delay-2 mt-6 flex items-center gap-2 text-sm text-white/60">
              <Icon name="check" className="h-4 w-4 text-white" />
              No credit card required · Set up in minutes
            </p>
            <div className="lp-rise-delay-2 mt-6 flex flex-wrap gap-2">
              {["SSO & SCIM", "Audit trail", "Tenant isolation"].map((chip) => (
                <span
                  key={chip}
                  className="inline-flex items-center gap-1.5 rounded-full border border-white/25 bg-white/10 px-3 py-1 text-xs font-medium text-white/85 backdrop-blur"
                >
                  <Icon name="check" className="h-3 w-3 text-white" />
                  {chip}
                </span>
              ))}
            </div>
          </div>

          <div className="lp-rise-delay relative lg:pl-6">
            <ProductPreview />
          </div>
        </Container>
      </section>

      {/* Logo cloud -------------------------------------------------------- */}
      <section className="border-b border-[var(--lp-border)] bg-[var(--lp-paper-elevated)] py-10">
        <Container>
          <p className="text-center text-xs font-semibold uppercase tracking-[0.2em] text-[var(--lp-ink-muted)]">
            Trusted by people teams at fast-growing companies
          </p>
          <div className="mt-6 flex flex-wrap items-center justify-center gap-x-12 gap-y-4">
            {logos.map((logo) => (
              <span
                key={logo}
                className="text-lg font-semibold tracking-tight text-[var(--lp-ink)] opacity-45"
              >
                {logo}
              </span>
            ))}
          </div>
        </Container>
      </section>

      {/* Features ---------------------------------------------------------- */}
      <section className="py-24">
        <Container>
          <div className="mx-auto max-w-2xl text-center">
            <p className="lp-eyebrow justify-center">
              One platform
            </p>
            <h2
              className="mt-3 text-3xl font-semibold tracking-tight text-[var(--lp-ink)] md:text-4xl"
              style={{ fontFamily: "var(--lp-font-display)" }}
            >
              From day zero to contributing
            </h2>
            <p className="mt-4 text-lg text-[var(--lp-ink-muted)]">
              Replace scattered docs and tribal knowledge with structured, measurable
              onboarding your whole company can rely on.
            </p>
          </div>
          <div className="mt-14 grid gap-6 md:grid-cols-2 lg:grid-cols-3">
            {features.map((feature) => (
              <FeatureCard
                key={feature.title}
                icon={feature.icon}
                title={feature.title}
                body={feature.body}
              />
            ))}
          </div>
        </Container>
      </section>

      {/* Product proof ---------------------------------------------------- */}
      <section className="py-24">
        <Container>
          <div className="max-w-2xl">
            <h2
              className="text-3xl font-semibold tracking-tight text-[var(--lp-ink)] md:text-5xl"
              style={{ fontFamily: "var(--lp-font-display)" }}
            >
              See the work. Then see where it needs help.
            </h2>
            <p className="mt-4 text-lg leading-8 text-[var(--lp-ink-muted)]">
              Managers get a live operating view of progress, blockers, approvals, and upcoming conversations.
            </p>
          </div>
          <ProductEvidence kind="manager" className="mt-12 lp-evidence--offset" />
        </Container>
      </section>

      {/* How it works ------------------------------------------------------ */}
      <section className="bg-[var(--lp-paper-elevated)] py-24">
        <Container>
          <div className="mx-auto max-w-2xl text-center">
            <p className="lp-eyebrow justify-center">
              How it works
            </p>
            <h2
              className="mt-3 text-3xl font-semibold tracking-tight text-[var(--lp-ink)] md:text-4xl"
              style={{ fontFamily: "var(--lp-font-display)" }}
            >
              Launch onboarding in three moves
            </h2>
          </div>
          <div className="mt-14 grid gap-6 md:grid-cols-3">
            {steps.map((step, index) => (
              <div key={step.title} className="lp-card p-7">
                <span className="lp-icon-chip text-lg font-semibold">
                  {index + 1}
                </span>
                <h3
                  className="mt-5 text-xl font-semibold text-[var(--lp-ink)]"
                  style={{ fontFamily: "var(--lp-font-display)" }}
                >
                  {step.title}
                </h3>
                <p className="mt-2 leading-7 text-[var(--lp-ink-muted)]">{step.body}</p>
              </div>
            ))}
          </div>
        </Container>
      </section>

      {/* Grounded answers -------------------------------------------------- */}
      <section className="py-24">
        <Container className="grid items-center gap-12 lg:grid-cols-[0.72fr_1.28fr]">
          <div>
            <h2
              className="text-3xl font-semibold tracking-tight text-[var(--lp-ink)] md:text-5xl"
              style={{ fontFamily: "var(--lp-font-display)" }}
            >
              Answers employees can verify.
            </h2>
            <p className="mt-5 text-lg leading-8 text-[var(--lp-ink-muted)]">
              LaunchPad cites approved company sources and keeps a human escalation path close.
            </p>
            <Link href="/features/knowledge-assistant" className="mt-7 inline-flex font-semibold text-[var(--lp-brand)]">
              Explore the knowledge assistant
              <Icon name="arrow-right" className="ml-2 h-4 w-4" />
            </Link>
          </div>
          <ProductEvidence kind="assistant" caption={false} />
        </Container>
      </section>

      {/* Stats band -------------------------------------------------------- */}
      <section className="py-6">
        <Container>
          <div
            className="relative overflow-hidden rounded-3xl px-8 py-12 text-white"
            style={{
              background:
                "radial-gradient(ellipse 60% 80% at 100% 0%, rgba(46,91,176,0.55), transparent 60%), linear-gradient(160deg, #0f1e3a, #16386e)",
            }}
          >
            <LogoWatermark onDark className="-bottom-16 -left-16 size-72 rotate-12" />
            <div className="grid gap-8 text-center sm:grid-cols-2 lg:grid-cols-4">
              {stats.map((stat) => (
                <div key={stat.label}>
                  <p
                    className="text-4xl font-semibold tracking-tight md:text-5xl"
                    style={{ fontFamily: "var(--lp-font-display)" }}
                  >
                    {stat.value}
                  </p>
                  <p className="mt-2 text-sm text-white/70">{stat.label}</p>
                </div>
              ))}
            </div>
          </div>
        </Container>
      </section>

      {/* Testimonial ------------------------------------------------------- */}
      <section className="py-24">
        <Container className="max-w-3xl text-center">
          <div className="flex justify-center gap-1 text-[var(--lp-signal)]" aria-hidden="true">
            {Array.from({ length: 5 }).map((_, index) => (
              <Icon key={index} name="sparkles" className="h-5 w-5" />
            ))}
          </div>
          <blockquote
            className="mt-6 text-2xl font-medium leading-relaxed text-[var(--lp-ink)] md:text-3xl"
            style={{ fontFamily: "var(--lp-font-display)" }}
          >
            &ldquo;LaunchPad cut our new-hire ramp from six weeks to three. Managers
            finally see where onboarding stalls — and the AI assistant answers the
            questions we used to field by hand.&rdquo;
          </blockquote>
          <div className="mt-8 flex items-center justify-center gap-3">
            <span
              className="grid h-11 w-11 place-items-center rounded-full font-semibold text-white"
              style={{ background: "var(--lp-cta-gradient)" }}
            >
              JD
            </span>
            <div className="text-left">
              <p className="font-semibold text-[var(--lp-ink)]">Jordan Diaz</p>
              <p className="text-sm text-[var(--lp-ink-muted)]">VP People, Northwind</p>
            </div>
          </div>
        </Container>
      </section>

      {/* CTA band ---------------------------------------------------------- */}
      <section className="pb-24">
        <Container>
          <div className="lp-hero relative overflow-hidden rounded-3xl px-8 py-16 text-center">
            <LogoWatermark onDark className="-right-20 -top-20 size-80 rotate-[-12deg]" />
            <h2
              className="mx-auto max-w-2xl text-3xl font-semibold tracking-tight md:text-4xl"
              style={{ fontFamily: "var(--lp-font-display)" }}
            >
              Ready to make onboarding your advantage?
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
