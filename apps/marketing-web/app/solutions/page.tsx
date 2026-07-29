import Link from "next/link";
import { Container } from "@launchpad/ui";
import { buildMetadata } from "../../lib/seo";
import { solutionPages } from "../../lib/marketing-pages";
import { SiteHeader } from "../site-header";
import { SiteFooter } from "../site-footer";

export const metadata = buildMetadata({
  title: "Onboarding solutions — LaunchPad",
  description: "LaunchPad onboarding solutions for HR, IT, security, engineering, sales, support, remote, startup, enterprise, and regulated teams.",
  path: "/solutions",
});

export default function SolutionsPage() {
  return (
    <main>
      <section className="lp-hero relative"><SiteHeader variant="hero" /><Container className="pb-20 pt-40"><p className="lp-eyebrow lp-eyebrow--on-dark">Solutions</p><h1 className="mt-6 max-w-3xl text-5xl font-semibold md:text-7xl">Onboarding shaped around how your team works.</h1></Container></section>
      <section className="py-24"><Container><div className="grid gap-5 md:grid-cols-2">{solutionPages.map((page) => <Link key={page.slug} href={`/solutions/${page.slug}`} className="lp-card p-6 transition hover:-translate-y-1"><p className="lp-eyebrow">{page.eyebrow}</p><h2 className="mt-3 text-2xl font-semibold">{page.title}</h2><p className="mt-3 text-[var(--lp-ink-muted)]">{page.description}</p></Link>)}</div></Container></section>
      <SiteFooter />
    </main>
  );
}
