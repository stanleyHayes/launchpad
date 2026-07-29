import Link from "next/link";
import { Container } from "@launchpad/ui";
import { buildMetadata } from "../../lib/seo";
import { solutionPages } from "../../lib/marketing-pages";
import { SiteHeader } from "../site-header";
import { SiteFooter } from "../site-footer";
import { ProductEvidence, evidenceForSlug } from "../product-evidence";

export const metadata = buildMetadata({
  title: "Onboarding solutions — LaunchPad",
  description: "LaunchPad onboarding solutions for HR, IT, security, engineering, sales, support, remote, startup, enterprise, and regulated teams.",
  path: "/solutions",
});

export default function SolutionsPage() {
  return (
    <main>
      <section className="lp-hero relative">
        <SiteHeader variant="hero" />
        <Container className="grid items-center gap-12 pb-20 pt-36 lg:grid-cols-[0.82fr_1.18fr]">
          <div>
            <p className="lp-eyebrow lp-eyebrow--on-dark">Solutions</p>
            <h1 className="mt-6 text-4xl font-semibold tracking-tight md:text-6xl">
              Onboarding shaped around how your team works.
            </h1>
          </div>
          <ProductEvidence kind="journey" priority caption={false} />
        </Container>
      </section>
      <section className="py-24">
        <Container>
          <div className="grid gap-x-12 gap-y-14 md:grid-cols-2">
            {solutionPages.map((page, index) => (
              <Link key={page.slug} href={`/solutions/${page.slug}`} className="group">
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
