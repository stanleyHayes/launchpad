import Link from "next/link";
import type { Metadata } from "next";
import { notFound } from "next/navigation";
import { ApiError, createLaunchPadClient, type CMSPage } from "@launchpad/api-client";
import { Container } from "@launchpad/ui";
import { apiBaseUrl } from "../env";
import { buildMetadata } from "../../lib/seo";
import { SiteFooter } from "../site-footer";
import { SiteHeader } from "../site-header";

async function loadPage(slug: string): Promise<CMSPage | null> {
  try {
    const client = createLaunchPadClient({ baseUrl: apiBaseUrl });
    return await client.getPublishedCMSPage(slug);
  } catch (err) {
    if (err instanceof ApiError && err.status === 404) {
      return null;
    }
    throw err;
  }
}

export async function generateMetadata({
  params,
}: {
  params: Promise<{ slug: string }>;
}): Promise<Metadata> {
  const { slug } = await params;
  const page = await loadPage(slug);

  if (!page) {
    return buildMetadata({
      title: "LaunchPad",
      description:
        "Build guided onboarding journeys, automate setup, and measure time-to-productivity.",
      path: `/${slug}`,
    });
  }

  return buildMetadata({
    title: `${page.title} — LaunchPad`,
    description:
      page.summary ||
      "Build guided onboarding journeys, automate setup, and measure time-to-productivity.",
    path: `/${slug}`,
  });
}

export default async function MarketingCMSPage({
  params,
}: {
  params: Promise<{ slug: string }>;
}) {
  const { slug } = await params;
  const page = await loadPage(slug);

  if (!page) {
    notFound();
  }

  const paragraphs = page.body
    .split(/\n+/)
    .map((part) => part.trim())
    .filter(Boolean);

  return (
    <main className="relative">
      <SiteHeader variant="light" />
      <section className="pb-20 pt-36">
        <Container className="max-w-3xl">
          <p className="lp-rise lp-eyebrow">
            LaunchPad
          </p>
          <h1
            className="lp-rise mt-4 text-4xl font-semibold tracking-tight md:text-5xl"
            style={{ fontFamily: "var(--lp-font-display)" }}
          >
            {page.title}
          </h1>
          {page.summary ? (
            <p className="lp-rise-delay mt-5 text-lg text-[var(--lp-ink-muted)]">{page.summary}</p>
          ) : null}
          <div className="lp-rise-delay mt-10 space-y-5 text-base leading-7 text-[var(--lp-ink)]">
            {paragraphs.map((paragraph, index) => (
              <p key={`${index}-${paragraph.slice(0, 24)}`}>{paragraph}</p>
            ))}
          </div>
          <div className="mt-12 flex flex-wrap gap-4">
            <Link
              href="/signup"
              className="lp-btn lp-btn--primary"
            >
              Start free trial
            </Link>
            <Link
              href="/demo"
              className="lp-btn lp-btn--secondary"
            >
              Book a demo
            </Link>
          </div>
        </Container>
      </section>

      <SiteFooter />
    </main>
  );
}
