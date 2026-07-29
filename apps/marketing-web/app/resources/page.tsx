import type { Metadata } from "next";
import Link from "next/link";
import { Container } from "@launchpad/ui";
import { buildMetadata } from "../../lib/seo";
import { SiteHeader } from "../site-header";
import { SiteFooter } from "../site-footer";

export const metadata: Metadata = buildMetadata({
  title: "Employee onboarding resources — LaunchPad",
  description: "Practical guides for designing measurable, human employee onboarding.",
  path: "/resources",
});

const guides = [
  ["The 30-day onboarding scorecard", "Measure confidence, access, relationships, and role readiness."],
  ["Manager’s first-week checklist", "The conversations and checkpoints that prevent a new hire from drifting."],
  ["Designing approval-safe journeys", "Place human gates around access, equipment, policy, and compliance work."],
  ["From orientation to productivity", "Use milestones and drop-off signals to continuously improve each journey."],
];

export default function ResourcesPage() {
  return (
    <main className="min-h-screen">
      <SiteHeader variant="light" />
      <section className="pb-20 pt-36">
        <Container>
          <p className="lp-eyebrow">Resource library</p>
          <h1 className="mt-4 max-w-3xl text-5xl font-semibold tracking-tight">Build onboarding people can actually follow.</h1>
          <p className="mt-5 max-w-2xl text-lg leading-8 text-[var(--lp-ink-muted)]">Field-tested playbooks for HR, IT, managers, and people operations teams.</p>
          <div className="mt-12 grid gap-4 md:grid-cols-2">
            {guides.map(([title, description]) => (
              <article className="lp-card p-7" key={title}>
                <p className="lp-eyebrow">Guide</p>
                <h2 className="mt-3 text-2xl font-semibold">{title}</h2>
                <p className="mt-3 leading-7 text-[var(--lp-ink-muted)]">{description}</p>
                <Link className="mt-6 inline-flex font-semibold text-[var(--lp-brand)]" href="/demo">Discuss this playbook →</Link>
              </article>
            ))}
          </div>
        </Container>
      </section>
      <SiteFooter />
    </main>
  );
}
