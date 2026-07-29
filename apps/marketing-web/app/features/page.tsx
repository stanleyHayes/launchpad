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
      <section className="py-24 md:py-28">
        <Container>
          <div className="mb-12 max-w-2xl">
            <p className="lp-eyebrow">Built into the workflow</p>
            <h2 className="mt-5 text-3xl font-semibold tracking-tight md:text-5xl">
              Every capability has an operating view.
            </h2>
            <p className="mt-4 text-lg leading-8 text-[var(--lp-ink-muted)]">
              Explore the product through real LaunchPad screens, not feature claims alone.
            </p>
          </div>
          <div className="grid gap-6 md:grid-cols-2">
            {featurePages.map((page, index) => (
              <Link
                key={page.slug}
                href={`/features/${page.slug}`}
                className={`lp-feature-card group ${index % 3 === 0 ? "md:translate-y-6" : ""}`}
              >
                <div className="lp-feature-card__media">
                  <ProductEvidence
                    kind={evidenceForSlug(page.slug)}
                    caption={false}
                    className="h-full"
                  />
                  <span className="lp-feature-card__index" aria-hidden="true">
                    {String(index + 1).padStart(2, "0")}
                  </span>
                </div>
                <div className="p-6 md:p-7">
                  <h3 className="text-2xl font-semibold transition-colors group-hover:text-[var(--lp-brand)]">
                    {page.title}
                  </h3>
                  <p className="mt-3 max-w-xl leading-7 text-[var(--lp-ink-muted)]">
                    {page.description}
                  </p>
                  <span className="mt-6 inline-flex items-center gap-2 text-sm font-semibold text-[var(--lp-brand)]">
                    Explore capability
                    <span aria-hidden="true" className="transition-transform group-hover:translate-x-1">→</span>
                  </span>
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
