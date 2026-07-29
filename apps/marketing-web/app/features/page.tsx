import Link from "next/link";
import { Container } from "@launchpad/ui";
import { buildMetadata } from "../../lib/seo";
import { featurePages } from "../../lib/marketing-pages";
import { SiteHeader } from "../site-header";
import { SiteFooter } from "../site-footer";
import { ProductEvidence, evidenceForSlug } from "../product-evidence";

export const metadata = buildMetadata({
  title: "Product features — LaunchPad",
  description: "Explore LaunchPad onboarding journeys, workflows, assessments, knowledge, analytics, security, integrations, and templates.",
  path: "/features",
});

export default function FeaturesPage() {
  return (
    <main>
      <section className="lp-hero relative">
        <SiteHeader variant="hero" />
        <Container className="grid items-center gap-12 pb-20 pt-36 lg:grid-cols-[0.78fr_1.22fr]">
          <div>
            <p className="lp-eyebrow lp-eyebrow--on-dark">Product features</p>
            <h1 className="mt-6 text-4xl font-semibold tracking-tight md:text-6xl">
              See how a confident first month gets built.
            </h1>
            <p className="mt-5 max-w-xl text-lg leading-8 text-white/75">
              Real operating views for journeys, managers, and every employee question.
            </p>
          </div>
          <ProductEvidence kind="manager" priority caption={false} />
        </Container>
      </section>
      <section className="py-24">
        <Container>
          <div className="grid gap-x-12 gap-y-16 md:grid-cols-2">
            {featurePages.map((page, index) => (
              <Link key={page.slug} href={`/features/${page.slug}`} className="group">
                {index < 4 ? (
                  <ProductEvidence
                    kind={evidenceForSlug(page.slug)}
                    caption={false}
                    className="mb-6"
                  />
                ) : null}
                <div className="border-t border-[var(--lp-border)] pt-5">
                  <h2 className="text-2xl font-semibold group-hover:text-[var(--lp-brand)]">
                    {page.title}
                  </h2>
                  <p className="mt-3 max-w-xl leading-7 text-[var(--lp-ink-muted)]">
                    {page.description}
                  </p>
                </div>
              </Link>
            ))}
          </div>
        </Container>
      </section>
      <SiteFooter />
    </main>
  );
}
