"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import { usePathname } from "next/navigation";
import { LogoTile, ThemeSwitcher } from "@launchpad/ui";
import { createLaunchPadClient, type CMSPage } from "@launchpad/api-client";
import { Icon } from "./ui-icon";
import { apiBaseUrl } from "./env";

// Only routes with real content are linked here; add entries back as pages
// are built or CMS pages are published.
const navLinks = [
  { href: "/product", label: "Product" },
  { href: "/features", label: "Features" },
  { href: "/solutions", label: "Solutions" },
  { href: "/integrations", label: "Integrations" },
  { href: "/resources", label: "Resources" },
  { href: "/pricing", label: "Pricing" },
];

/**
 * SiteHeader is the marketing navigation: a floating translucent "pill" bar
 * with the wordmark on the left, links in the centre, and a gradient CTA on the
 * right. On narrow viewports the links collapse into a toggled menu.
 *
 * `variant="light"` renders dark text for use over pale page backgrounds; the
 * default renders white text for use over the navy hero.
 */
export function SiteHeader({ variant = "hero" }: { variant?: "hero" | "light" }) {
  const [open, setOpen] = useState(false);
  const pathname = usePathname();
  const onHero = variant === "hero";
  const [cmsLinks, setCMSLinks] = useState<CMSPage[]>([]);

  useEffect(() => {
    let active = true;
    void createLaunchPadClient({ baseUrl: apiBaseUrl }).getCMSNavigation()
      .then((items) => { if (active) setCMSLinks(items); })
      .catch(() => { /* Static navigation remains available during API degradation. */ });
    return () => { active = false; };
  }, []);

  const links = [
    ...navLinks,
    ...cmsLinks
      .filter((page) => !navLinks.some((link) => link.href === `/${page.slug}`))
      .map((page) => ({ href: `/${page.slug}`, label: page.navLabel ?? page.title })),
  ];

  function linkClass(href: string): string {
    const active = pathname === href || pathname.startsWith(`${href}/`);
    const base = "relative py-1 transition";
    if (onHero) {
      return `${base} ${active ? "font-semibold text-white" : "text-white/75 hover:text-white"}`;
    }
    return `${base} ${
      active
        ? "font-semibold text-[var(--lp-brand)]"
        : "text-[var(--lp-ink-muted)] hover:text-[var(--lp-ink)]"
    }`;
  }

  function activeTick(href: string) {
    const active = pathname === href || pathname.startsWith(`${href}/`);
    return active ? (
      <span
        aria-hidden="true"
        className="absolute -bottom-0.5 left-0 right-0 h-0.5 rounded-full bg-[var(--lp-signal)]"
      />
    ) : null;
  }

  return (
    <header className="absolute inset-x-0 top-0 z-30">
      <div className="mx-auto w-full max-w-6xl px-4 pt-5">
        <div
          className={
            onHero
              ? "lp-nav-pill flex items-center justify-between gap-4 px-5 py-3 text-white"
              : "lp-nav-light flex items-center justify-between gap-4 px-5 py-3 text-[var(--lp-ink)]"
          }
        >
          <Link href="/" className="flex items-center gap-2 font-semibold tracking-tight">
            <LogoTile size={32} />
            <span className="text-lg">LaunchPad</span>
          </Link>

          <nav className="hidden items-center gap-7 text-sm md:flex">
            {links.map((link) => (
              <Link key={link.label} href={link.href} className={linkClass(link.href)}>
                {link.label}
                {activeTick(link.href)}
              </Link>
            ))}
          </nav>

          <div className="flex items-center gap-3">
            <Link
              href={pathname.startsWith("/fr") ? "/" : "/fr"}
              hrefLang={pathname.startsWith("/fr") ? "en" : "fr"}
              className={onHero ? "text-xs font-semibold text-white/80" : "text-xs font-semibold text-[var(--lp-ink-muted)]"}
            >
              {pathname.startsWith("/fr") ? "EN" : "FR"}
            </Link>
            <ThemeSwitcher onDark={onHero} className="hidden md:inline-flex" />
            <Link
              href="/signup"
              className="hidden md:inline-flex"
              style={{ textDecoration: "none" }}
            >
              <span className="lp-btn lp-btn--primary">
                Start free trial
                <Icon name="arrow-right" className="h-4 w-4" />
              </span>
            </Link>
            <button
              type="button"
              onClick={() => {
                setOpen((value) => !value);
              }}
              aria-label={open ? "Close menu" : "Open menu"}
              aria-expanded={open}
              className={
                onHero
                  ? "grid h-10 w-10 place-items-center rounded-full border border-white/20 bg-white/10 text-white md:hidden"
                  : "grid h-10 w-10 place-items-center rounded-full bg-[var(--lp-paper-elevated)] text-[var(--lp-ink)] shadow-[var(--lp-shadow)] md:hidden"
              }
            >
              <Icon name={open ? "close" : "menu"} className="h-5 w-5" />
            </button>
          </div>
        </div>

        {open ? (
          <div className="lp-card mt-2 p-4 text-[var(--lp-ink)] md:hidden">
            <nav className="flex flex-col">
              {links.map((link) => {
                const active = pathname === link.href;
                return (
                  <Link
                    key={link.label}
                    href={link.href}
                    onClick={() => {
                      setOpen(false);
                    }}
                    className={`rounded-xl px-3 py-2.5 text-sm ${
                      active
                        ? "bg-[var(--lp-brand-soft)] font-semibold text-[var(--lp-brand)]"
                        : "font-medium text-[var(--lp-ink)] hover:bg-[var(--lp-brand-soft)]"
                    }`}
                  >
                    {link.label}
                  </Link>
                );
              })}
              <Link
                href="/signup"
                onClick={() => {
                  setOpen(false);
                }}
                className="mt-2"
                style={{ textDecoration: "none" }}
              >
                <span className="lp-btn lp-btn--primary w-full">
                  Start free trial
                  <Icon name="arrow-right" className="h-4 w-4" />
                </span>
              </Link>
            </nav>
          </div>
        ) : null}
      </div>
    </header>
  );
}
