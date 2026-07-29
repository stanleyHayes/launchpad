import Link from "next/link";
import { notFound } from "next/navigation";
import { Container } from "@launchpad/ui";
import { buildMetadata } from "../../../lib/seo";
import { findMarketingPage, solutionPages } from "../../../lib/marketing-pages";
import { SiteHeader } from "../../site-header";
import { SiteFooter } from "../../site-footer";
import { Icon } from "../../ui-icon";

export function generateStaticParams() {
  return solutionPages.map(({ slug }) => ({ slug }));
}

export async function generateMetadata({ params }: { params: Promise<{ slug: string }> }) {
  const { slug } = await params;
  const page = findMarketingPage(solutionPages, slug);
  return page ? buildMetadata({ title: `${page.title} — LaunchPad`, description: page.description, path: `/solutions/${slug}` }) : {};
}

export default async function SolutionPage({ params }: { params: Promise<{ slug: string }> }) {
  const { slug } = await params;
  const page = findMarketingPage(solutionPages, slug);
  if (!page) notFound();
  return (
    <main>
      <section className="lp-hero relative">
        <SiteHeader variant="hero" />
        <Container className="grid gap-12 pb-24 pt-40 lg:grid-cols-[1.2fr_0.8fr]">
          <div>
            <p className="lp-eyebrow lp-eyebrow--on-dark">{page.eyebrow}</p>
            <h1 className="mt-6 text-5xl font-semibold tracking-tight md:text-7xl">{page.title}</h1>
            <p className="mt-6 max-w-2xl text-xl leading-8 text-white/80">{page.description}</p>
          </div>
          <div className="rounded-3xl border border-white/15 bg-white/10 p-7 backdrop-blur">
            <p className="text-sm font-semibold uppercase tracking-wider text-white/60">Built for your operating model</p>
            <ul className="mt-6 space-y-5">
              {page.outcomes.map((outcome) => <li key={outcome} className="flex gap-3 text-lg"><Icon name="check" className="mt-1 h-5 w-5 text-[var(--lp-signal)]" />{outcome}</li>)}
            </ul>
          </div>
        </Container>
      </section>
      <section className="py-24">
        <Container className="text-center">
          <p className="lp-eyebrow justify-center">A deliberate first month</p>
          <h2 className="mx-auto mt-4 max-w-3xl text-4xl font-semibold tracking-tight">Give every new hire the context, access, practice, and feedback they need.</h2>
          <div className="mt-9 flex justify-center gap-3">
            <Link href="/signup" className="lp-btn lp-btn--primary">Start free trial</Link>
            <Link href="/demo" className="lp-btn lp-btn--secondary">Talk to sales</Link>
          </div>
        </Container>
      </section>
      <SiteFooter />
    </main>
  );
}
