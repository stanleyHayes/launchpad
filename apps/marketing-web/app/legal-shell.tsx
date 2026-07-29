import type { ReactNode } from "react";
import { Container } from "@launchpad/ui";
import { SiteFooter } from "./site-footer";
import { SiteHeader } from "./site-header";

/**
 * LegalShell gives legal and company pages (privacy, terms, security,
 * contact) the same skeleton as the other marketing routes: light header,
 * eyebrow + display-font hero, and a max-w-3xl prose column, with the site
 * footer closing the page.
 */
export function LegalShell({
  eyebrow,
  title,
  intro,
  updated,
  children,
}: {
  eyebrow: string;
  title: string;
  intro: string;
  updated?: string;
  children: ReactNode;
}) {
  return (
    <main className="relative">
      <SiteHeader variant="light" />
      <section className="pb-16 pt-36">
        <Container className="max-w-3xl">
          <p className="lp-rise lp-eyebrow">{eyebrow}</p>
          <h1
            className="lp-rise mt-5 text-4xl font-semibold tracking-tight text-[var(--lp-ink)] md:text-5xl"
            style={{ fontFamily: "var(--lp-font-display)" }}
          >
            {title}
          </h1>
          <p className="lp-rise-delay mt-5 text-lg leading-8 text-[var(--lp-ink-muted)]">{intro}</p>
          {updated ? (
            <p className="lp-rise-delay mt-4 text-sm text-[var(--lp-ink-muted)]">
              Last updated: {updated}
            </p>
          ) : null}
        </Container>
      </section>
      <section className="pb-24">
        <Container className="max-w-3xl">
          <div className="space-y-10">{children}</div>
        </Container>
      </section>
      <SiteFooter />
    </main>
  );
}

/** LegalSection is one headed prose block inside a LegalShell page. */
export function LegalSection({
  heading,
  children,
}: {
  heading: string;
  children: ReactNode;
}) {
  return (
    <div>
      <h2
        className="text-xl font-semibold tracking-tight text-[var(--lp-ink)]"
        style={{ fontFamily: "var(--lp-font-display)" }}
      >
        {heading}
      </h2>
      <div className="mt-3 space-y-3 text-base leading-7 text-[var(--lp-ink-muted)]">
        {children}
      </div>
    </div>
  );
}
