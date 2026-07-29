import Link from "next/link";
import type { ReactNode } from "react";
import { Container } from "@launchpad/ui";
import { SiteFooter } from "./site-footer";
import { SiteHeader } from "./site-header";
import { Icon, type IconName } from "./ui-icon";

export interface LegalNavItem {
  id: string;
  label: string;
}

export interface LegalHighlight {
  icon: IconName;
  label: string;
  value: string;
}

export function LegalShell({
  eyebrow,
  title,
  intro,
  updated,
  responseTime,
  icon = "book",
  sections,
  highlights,
  children,
}: {
  eyebrow: string;
  title: string;
  intro: string;
  updated?: string;
  responseTime?: string;
  icon?: IconName;
  sections: LegalNavItem[];
  highlights: LegalHighlight[];
  children: ReactNode;
}) {
  return (
    <main className="relative">
      <SiteHeader variant="light" />

      <section className="lp-legal-hero">
        <Container>
          <div className="grid items-end gap-10 lg:grid-cols-[minmax(0,1.35fr)_minmax(280px,.65fr)]">
            <div>
              <div className="lp-rise lp-legal-kicker">
                <span className="lp-legal-kicker__icon" aria-hidden="true">
                  <Icon name={icon} className="h-4 w-4" />
                </span>
                {eyebrow}
              </div>
              <h1 className="lp-rise mt-6 max-w-4xl text-5xl font-semibold leading-[1.02] tracking-[-0.045em] text-[var(--lp-ink)] md:text-7xl">
                {title}
              </h1>
            </div>

            <div className="lp-rise-delay border-l border-[var(--lp-border)] pl-6">
              <p className="text-lg leading-8 text-[var(--lp-ink-muted)]">{intro}</p>
              {updated ? (
                <p className="mt-5 flex items-center gap-2 text-sm font-medium text-[var(--lp-ink)]">
                  <Icon name="clock" className="h-4 w-4 text-[var(--lp-brand)]" />
                  Effective {updated}
                </p>
              ) : (
                <p className="mt-5 flex items-center gap-2 text-sm font-medium text-[var(--lp-ink)]">
                  <Icon name="message" className="h-4 w-4 text-[var(--lp-brand)]" />
                  Typical response: {responseTime ?? "One business day"}
                </p>
              )}
            </div>
          </div>

          <div className="lp-rise-delay-2 mt-12 grid overflow-hidden rounded-2xl border border-[var(--lp-border)] bg-[var(--lp-paper-elevated)] sm:grid-cols-3">
            {highlights.map((highlight) => (
              <div
                key={highlight.label}
                className="flex items-center gap-4 border-b border-[var(--lp-border)] px-5 py-5 last:border-b-0 sm:border-b-0 sm:border-r sm:last:border-r-0"
              >
                <span className="grid h-10 w-10 shrink-0 place-items-center rounded-xl bg-[var(--lp-brand-soft)] text-[var(--lp-brand)]">
                  <Icon name={highlight.icon} className="h-5 w-5" />
                </span>
                <span>
                  <span className="block text-xs font-semibold uppercase tracking-[0.12em] text-[var(--lp-ink-muted)]">
                    {highlight.label}
                  </span>
                  <span className="mt-1 block text-sm font-semibold text-[var(--lp-ink)]">
                    {highlight.value}
                  </span>
                </span>
              </div>
            ))}
          </div>
        </Container>
      </section>

      <section className="pb-24 pt-10 md:pt-16">
        <Container>
          <div className="grid gap-12 lg:grid-cols-[240px_minmax(0,1fr)] lg:gap-20">
            <aside className="lg:sticky lg:top-28 lg:self-start">
              <p className="text-xs font-semibold uppercase tracking-[0.14em] text-[var(--lp-ink-muted)]">
                On this page
              </p>
              <nav className="mt-5" aria-label={`${title} sections`}>
                <ol className="space-y-1">
                  {sections.map((section, index) => (
                    <li key={section.id}>
                      <a className="lp-legal-nav-link" href={`#${section.id}`}>
                        <span>{String(index + 1).padStart(2, "0")}</span>
                        {section.label}
                      </a>
                    </li>
                  ))}
                </ol>
              </nav>
              <div className="mt-8 rounded-2xl bg-[var(--lp-brand-soft)] p-5">
                <Icon name="message" className="h-5 w-5 text-[var(--lp-brand)]" />
                <p className="mt-3 text-sm font-semibold text-[var(--lp-ink)]">
                  Need a specific answer?
                </p>
                <p className="mt-1 text-sm leading-6 text-[var(--lp-ink-muted)]">
                  Our team can help with legal, privacy, procurement, or security questions.
                </p>
                <Link
                  href="/contact"
                  className="mt-4 inline-flex items-center gap-2 text-sm font-semibold text-[var(--lp-brand)]"
                >
                  Contact LaunchPad
                  <Icon name="arrow-right" className="h-4 w-4" />
                </Link>
              </div>
            </aside>

            <article className="min-w-0">
              <div className="space-y-4">{children}</div>
            </article>
          </div>
        </Container>
      </section>

      <SiteFooter />
    </main>
  );
}

export function LegalSection({
  id,
  heading,
  children,
}: {
  id: string;
  heading: string;
  children: ReactNode;
}) {
  return (
    <section id={id} className="lp-legal-section scroll-mt-28">
      <h2 className="text-2xl font-semibold tracking-[-0.025em] text-[var(--lp-ink)] md:text-3xl">
        {heading}
      </h2>
      <div className="lp-legal-prose mt-5 space-y-4 text-base leading-8 text-[var(--lp-ink-muted)]">
        {children}
      </div>
    </section>
  );
}
