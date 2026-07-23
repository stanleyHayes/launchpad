import Link from "next/link";
import { Container, SectionHeading } from "@launchpad/ui";

export default function HomePage() {
  return (
    <main>
      <header className="absolute inset-x-0 top-0 z-10">
        <Container className="flex items-center justify-between py-6 text-white">
          <p className="text-lg font-semibold tracking-tight">LaunchPad</p>
          <nav className="flex items-center gap-6 text-sm">
            <Link href="/product" className="opacity-90 hover:opacity-100">
              Product
            </Link>
            <Link href="/pricing" className="opacity-90 hover:opacity-100">
              Pricing
            </Link>
            <Link
              href="/signup"
              className="rounded-[var(--lp-radius)] bg-white px-4 py-2 font-semibold text-[var(--lp-ink)]"
            >
              Start free trial
            </Link>
          </nav>
        </Container>
      </header>

      <section className="lp-hero-plane flex items-end">
        <Container className="pb-24 pt-40 text-white">
          <p className="lp-rise mb-4 text-sm font-semibold uppercase tracking-[0.2em] text-white/80">
            LaunchPad
          </p>
          <h1
            className="lp-rise max-w-3xl text-5xl font-semibold leading-tight tracking-tight md:text-6xl"
            style={{ fontFamily: "var(--lp-font-display)" }}
          >
            Help every new employee become confident and productive faster.
          </h1>
          <p className="lp-rise-delay mt-6 max-w-xl text-lg text-white/85">
            Guided journeys, automated setup, role-based training, and measurable
            onboarding outcomes — in one secure platform.
          </p>
          <div className="lp-rise-delay mt-10 flex flex-wrap gap-4">
            <Link
              href="/signup"
              className="rounded-[var(--lp-radius)] bg-[var(--lp-accent)] px-6 py-3 text-sm font-semibold text-white transition hover:bg-[var(--lp-accent-hover)] hover:-translate-y-0.5"
            >
              Start free trial
            </Link>
            <Link
              href="/demo"
              className="rounded-[var(--lp-radius)] border border-white/40 bg-white/10 px-6 py-3 text-sm font-semibold text-white backdrop-blur transition hover:-translate-y-0.5"
            >
              Book a demo
            </Link>
          </div>
        </Container>
      </section>

      <section className="py-24">
        <Container>
          <SectionHeading
            title="One journey from day zero to contributing"
            description="Replace scattered docs and tribal knowledge with structured, measurable onboarding."
          />
          <ul className="mt-12 grid gap-10 md:grid-cols-3">
            {[
              {
                title: "Workflow orchestration",
                body: "Pre-boarding, access, training, meetings, and approvals in one timeline.",
              },
              {
                title: "Manager visibility",
                body: "See blockers early and approve practical work without chasing status updates.",
              },
              {
                title: "Knowledge that answers",
                body: "An assistant grounded in your approved company sources — with citations.",
              },
            ].map((item) => (
              <li key={item.title}>
                <h3
                  className="text-xl font-semibold text-[var(--lp-ink)]"
                  style={{ fontFamily: "var(--lp-font-display)" }}
                >
                  {item.title}
                </h3>
                <p className="mt-3 text-[var(--lp-ink-muted)]">{item.body}</p>
              </li>
            ))}
          </ul>
        </Container>
      </section>
    </main>
  );
}
