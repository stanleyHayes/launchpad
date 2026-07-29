import Link from "next/link";
import { Container, LogoTile, LogoWatermark } from "@launchpad/ui";
import { orgAdminUrl } from "./env";
import { NewsletterForm } from "./newsletter-form";

// Only links with real destinations are listed; more columns return as pages
// are built or CMS pages are published.
const columns = [
  {
    heading: "Product",
    links: [
      { href: "/product", label: "Overview" },
      { href: "/features", label: "Features" },
      { href: "/solutions", label: "Solutions" },
      { href: "/integrations", label: "Integrations" },
      { href: "/pricing", label: "Pricing" },
      { href: "/demo", label: "Book a demo" },
      { href: "/templates", label: "Templates" },
    ],
  },
  {
    heading: "Get started",
    links: [
      { href: "/signup", label: "Start free trial" },
      { href: `${orgAdminUrl}/login`, label: "Sign in" },
      { href: "/status", label: "System status" },
    ],
  },
  {
    heading: "Legal",
    links: [
      { href: "/privacy", label: "Privacy" },
      { href: "/dpa", label: "DPA" },
      { href: "/terms", label: "Terms" },
      { href: "/security", label: "Security" },
      { href: "/contact", label: "Contact" },
    ],
  },
];

/**
 * SiteFooter closes every marketing page: brand block with the wordmark and
 * pitch, lean link columns with eyebrow-style headings, and a bottom bar.
 * Surfaces follow the active design system via the .lp-footer tokens.
 */
export function SiteFooter() {
  return (
    <footer className="lp-footer relative overflow-hidden">
      <LogoWatermark className="-bottom-24 -left-16 size-72 rotate-12" />
      <Container className="relative py-14">
        <div className="grid gap-10 md:grid-cols-[1.6fr_1fr_1fr_1fr]">
          <div>
            <Link href="/" className="flex items-center gap-2 font-semibold tracking-tight">
              <LogoTile size={32} />
              <span className="text-lg text-[var(--lp-ink)]">LaunchPad</span>
            </Link>
            <p className="mt-4 max-w-xs text-sm leading-6 text-[var(--lp-ink-muted)]">
              Guided onboarding journeys that make every new hire confident and
              productive — faster, and measurably.
            </p>
            <NewsletterForm />
          </div>

          {columns.map((column) => (
            <nav key={column.heading} aria-label={column.heading}>
              <h3 className="lp-eyebrow">{column.heading}</h3>
              <ul className="mt-5 space-y-3 text-sm">
                {column.links.map((link) => (
                  <li key={link.label}>
                    <Link
                      href={link.href}
                      className="text-[var(--lp-ink-muted)] transition hover:text-[var(--lp-brand)]"
                    >
                      {link.label}
                    </Link>
                  </li>
                ))}
              </ul>
            </nav>
          ))}
        </div>

        <div className="mt-12 flex flex-wrap items-center justify-between gap-3 border-t border-[var(--lp-border)] pt-6 text-sm text-[var(--lp-ink-muted)]">
          <p>© {new Date().getFullYear()} LaunchPad, Inc. All rights reserved.</p>
          <p>Employee onboarding, orchestrated.</p>
        </div>
        <p className="mt-4 text-xs text-[var(--lp-ink-muted)] opacity-70">
          SSO · SCIM 2.0 · Audit trail · Tenant isolation
        </p>
      </Container>
    </footer>
  );
}
