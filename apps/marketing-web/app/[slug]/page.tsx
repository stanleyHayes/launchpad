import Link from "next/link";
import { notFound } from "next/navigation";
import type { ReactNode } from "react";
import { ApiError, createLaunchPadClient, type CMSPage } from "@launchpad/api-client";
import { Container } from "@launchpad/ui";

const apiBaseUrl =
  process.env.NEXT_PUBLIC_API_BASE_URL ?? "http://localhost:8080";

type FallbackPage = {
  title: string;
  summary: string;
  body: string;
};

const fallbackPages: Record<string, FallbackPage> = {
  product: {
    title: "Product",
    summary: "Guided onboarding journeys with approvals, quizzes, and measurable progress.",
    body: [
      "LaunchPad replaces scattered checklists with structured journeys that managers can assign,",
      "employees can complete, and leaders can measure.",
      "",
      "Core capabilities include journey templates, step-level progress, manager approvals,",
      "in-app notifications, and organization analytics.",
    ].join("\n"),
  },
  pricing: {
    title: "Pricing",
    summary: "Simple plans that grow with your onboarding program.",
    body: [
      "Start with a free trial for your first organization, then choose a plan that matches",
      "your employee volume and support needs.",
      "",
      "Every plan includes journey builder access, employee portals, approvals,",
      "notifications, and onboarding analytics. Feature flags unlock advanced capabilities",
      "as your subscription expands.",
    ].join("\n"),
  },
};

async function loadPage(slug: string): Promise<CMSPage | FallbackPage | null> {
  try {
    const client = createLaunchPadClient({ baseUrl: apiBaseUrl });
    return await client.getPublishedCMSPage(slug);
  } catch (err) {
    if (err instanceof ApiError && err.status === 404) {
      return fallbackPages[slug] ?? null;
    }

    return fallbackPages[slug] ?? null;
  }
}

function MarketingChrome({ children }: { children: ReactNode }) {
  return (
    <main>
      <header className="border-b border-[var(--lp-border)] bg-white/70 backdrop-blur">
        <Container className="flex items-center justify-between py-5">
          <Link href="/" className="text-lg font-semibold tracking-tight">
            LaunchPad
          </Link>
          <nav className="flex items-center gap-6 text-sm">
            <Link href="/product" className="text-[var(--lp-ink-muted)] hover:text-[var(--lp-ink)]">
              Product
            </Link>
            <Link href="/pricing" className="text-[var(--lp-ink-muted)] hover:text-[var(--lp-ink)]">
              Pricing
            </Link>
            <Link
              href="/signup"
              className="rounded-[var(--lp-radius)] bg-[var(--lp-accent)] px-4 py-2 font-semibold text-white"
            >
              Start free trial
            </Link>
          </nav>
        </Container>
      </header>
      {children}
    </main>
  );
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
    <MarketingChrome>
      <section className="py-20">
        <Container className="max-w-3xl">
          <p className="lp-rise text-sm font-semibold uppercase tracking-[0.18em] text-[var(--lp-ink-muted)]">
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
              className="rounded-[var(--lp-radius)] bg-[var(--lp-accent)] px-5 py-3 text-sm font-semibold text-white"
            >
              Start free trial
            </Link>
            <Link
              href="/demo"
              className="rounded-[var(--lp-radius)] border border-[var(--lp-border)] px-5 py-3 text-sm font-semibold"
            >
              Book a demo
            </Link>
          </div>
        </Container>
      </section>
    </MarketingChrome>
  );
}
