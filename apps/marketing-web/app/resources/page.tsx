import type { Metadata } from "next";
import Link from "next/link";
import { Container } from "@launchpad/ui";
import { buildMetadata } from "../../lib/seo";
import { SiteHeader } from "../site-header";
import { SiteFooter } from "../site-footer";
import { ProductEvidence, type EvidenceKind } from "../product-evidence";

export const metadata: Metadata = buildMetadata({
  title: "Employee onboarding resources — LaunchPad",
  description: "Practical guides for designing measurable, human employee onboarding.",
  path: "/resources",
});

const guides = [
  ["The 30-day onboarding scorecard", "Measure confidence, access, relationships, and role readiness.", "manager"],
  ["Manager’s first-week checklist", "The conversations and checkpoints that prevent a new hire from drifting.", "journey"],
  ["Designing approval-safe journeys", "Place human gates around access, equipment, policy, and compliance work.", "manager"],
  ["From orientation to productivity", "Use milestones and drop-off signals to continuously improve each journey.", "assistant"],
] satisfies [string, string, EvidenceKind][];

export default function ResourcesPage() {
  return (
    <main className="min-h-screen">
      <SiteHeader variant="light" />
      <section className="pb-20 pt-36">
        <Container>
          <p className="lp-eyebrow">Resource library</p>
          <h1 className="mt-4 max-w-3xl text-5xl font-semibold tracking-tight">Build onboarding people can actually follow.</h1>
          <p className="mt-5 max-w-2xl text-lg leading-8 text-[var(--lp-ink-muted)]">Field-tested playbooks for HR, IT, managers, and people operations teams.</p>
          <div className="mt-12 grid gap-x-10 gap-y-14 md:grid-cols-2">
            {guides.map(([title, description, kind]) => (
              <article key={title}>
                <ProductEvidence kind={kind} caption={false} />
                <h2 className="mt-6 text-2xl font-semibold">{title}</h2>
                <p className="mt-3 leading-7 text-[var(--lp-ink-muted)]">{description}</p>
                <Link className="mt-5 inline-flex font-semibold text-[var(--lp-brand)]" href="/demo">Discuss this playbook <span aria-hidden="true" className="ml-2">→</span></Link>
              </article>
            ))}
          </div>
        </Container>
      </section>
      <SiteFooter />
    </main>
  );
}
