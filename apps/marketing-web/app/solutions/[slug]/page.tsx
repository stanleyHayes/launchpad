import Link from "next/link";
import { notFound } from "next/navigation";
import { Container } from "@launchpad/ui";
import { buildMetadata } from "../../../lib/seo";
import { findMarketingPage, solutionPages } from "../../../lib/marketing-pages";
import { SiteHeader } from "../../site-header";
import { SiteFooter } from "../../site-footer";
import { Icon } from "../../ui-icon";
import { ProductEvidence, evidenceForSlug } from "../../product-evidence";

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
        <Container className="grid items-center gap-12 pb-24 pt-36 lg:grid-cols-[0.78fr_1.22fr]">
          <div>
            <p className="lp-eyebrow lp-eyebrow--on-dark">{page.eyebrow}</p>
            <h1 className="mt-6 text-4xl font-semibold tracking-tight md:text-6xl">{page.title}</h1>
            <p className="mt-6 max-w-xl text-lg leading-8 text-white/80">{page.description}</p>
          </div>
          <ProductEvidence kind={evidenceForSlug(page.slug)} priority caption={false} />
        </Container>
      </section>
      <section className="py-24">
        <Container>
          <h2 className="max-w-3xl text-4xl font-semibold tracking-tight">Give every new hire the context, access, practice, and feedback they need.</h2>
          <div className="mt-10 grid gap-5 md:grid-cols-3">
            {page.outcomes.map((outcome) => <div key={outcome} className="border-t border-[var(--lp-border)] pt-5"><Icon name="check" className="h-5 w-5 text-[var(--lp-success)]" /><p className="mt-3 text-lg font-semibold">{outcome}</p></div>)}
          </div>
          <div className="mt-10 flex gap-3">
            <Link href="/signup" className="lp-btn lp-btn--primary">Start free trial</Link>
            <Link href="/demo" className="lp-btn lp-btn--secondary">Talk to sales</Link>
          </div>
        </Container>
      </section>
      <SiteFooter />
    </main>
  );
}
