import type { Metadata } from "next";
import Link from "next/link";
import { Container } from "@launchpad/ui";
import { buildMetadata } from "../../lib/seo";
import { SiteHeader } from "../site-header";
import { SiteFooter } from "../site-footer";

export const metadata: Metadata = buildMetadata({
  title: "Onboarding journey templates — LaunchPad",
  description: "Preview reusable onboarding journey templates for different teams and moments.",
  path: "/templates",
});

const templates = [
  ["New hire essentials", "12 steps", "HR, IT, manager"],
  ["Engineering launch", "18 steps", "Access, security, shipping"],
  ["Frontline team ramp", "10 steps", "Mobile-first, manager-led"],
  ["Internal transfer", "8 steps", "Role context and relationships"],
  ["Contractor access", "7 steps", "Time-bound approvals"],
  ["Manager onboarding", "14 steps", "People leadership foundations"],
];

export default function TemplatesPage() {
  return (
    <main className="min-h-screen">
      <SiteHeader variant="light" />
      <section className="pb-20 pt-36">
        <Container>
          <p className="lp-eyebrow">Template preview</p>
          <h1 className="mt-4 max-w-3xl text-5xl font-semibold tracking-tight">Start with a proven journey. Make it yours.</h1>
          <div className="mt-12 grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
            {templates.map(([name, count, scope]) => (
              <article className="lp-card p-6" key={name}>
                <p className="text-sm font-semibold text-[var(--lp-brand)]">{count}</p>
                <h2 className="mt-3 text-xl font-semibold">{name}</h2>
                <p className="mt-2 text-sm text-[var(--lp-ink-muted)]">{scope}</p>
                <Link className="lp-btn lp-btn--secondary mt-6" href="/signup">Use template</Link>
              </article>
            ))}
          </div>
        </Container>
      </section>
      <SiteFooter />
    </main>
  );
}
