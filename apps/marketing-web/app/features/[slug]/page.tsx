import Link from "next/link";
import { notFound } from "next/navigation";
import { Container } from "@launchpad/ui";
import { buildMetadata } from "../../../lib/seo";
import { featurePages, findMarketingPage } from "../../../lib/marketing-pages";
import { SiteHeader } from "../../site-header";
import { SiteFooter } from "../../site-footer";
import { Icon } from "../../ui-icon";
import { ProductEvidence, evidenceForSlug } from "../../product-evidence";

export function generateStaticParams() {
  return featurePages.map(({ slug }) => ({ slug }));
}

export async function generateMetadata({ params }: { params: Promise<{ slug: string }> }) {
  const { slug } = await params;
  const page = findMarketingPage(featurePages, slug);
  return page ? buildMetadata({ title: `${page.title} — LaunchPad`, description: page.description, path: `/features/${slug}` }) : {};
}

export default async function FeaturePage({ params }: { params: Promise<{ slug: string }> }) {
  const { slug } = await params;
  const page = findMarketingPage(featurePages, slug);
  if (!page) notFound();
  return <MarketingDetail page={page} />;
}

function MarketingDetail({ page }: { page: (typeof featurePages)[number] }) {
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
          <h2 className="max-w-2xl text-3xl font-semibold tracking-tight md:text-4xl">
            What your team can do from this view
          </h2>
          <div className="mt-10 grid gap-5 md:grid-cols-3">
            {page.outcomes.map((outcome) => (
              <article key={outcome} className="lp-card p-6">
                <Icon name="check" className="h-5 w-5 text-[var(--lp-success)]" />
                <h2 className="mt-4 text-xl font-semibold">{outcome}</h2>
                <p className="mt-3 text-sm leading-6 text-[var(--lp-ink-muted)]">Configured in one workspace, visible to the people responsible, and measured throughout the employee journey.</p>
              </article>
            ))}
          </div>
          <div className="mt-14 flex gap-3">
            <Link href="/signup" className="lp-btn lp-btn--primary">Start free trial</Link>
            <Link href="/demo" className="lp-btn lp-btn--secondary">Book a demo</Link>
          </div>
        </Container>
      </section>
      <SiteFooter />
    </main>
  );
}
