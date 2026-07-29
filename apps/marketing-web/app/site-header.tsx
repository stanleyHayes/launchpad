"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import { usePathname } from "next/navigation";
import { LogoTile, ThemeSwitcher } from "@launchpad/ui";
import { createLaunchPadClient, type CMSPage } from "@launchpad/api-client";
import { Icon, type IconName } from "./ui-icon";
import { apiBaseUrl } from "./env";

// Only routes with real content are linked here; add entries back as pages
// are built or CMS pages are published.
interface NavLink {
  href: string;
  label: string;
  description: string;
  icon: IconName;
}

const navLinks = [
  { href: "/product", label: "Product", description: "How LaunchPad works", icon: "workflow" },
  { href: "/features", label: "Features", description: "Explore every capability", icon: "sparkles" },
  { href: "/solutions", label: "Solutions", description: "Paths for every team", icon: "users" },
  { href: "/integrations", label: "Integrations", description: "Connect your systems", icon: "plug" },
  { href: "/resources", label: "Resources", description: "Guides and playbooks", icon: "book" },
  { href: "/pricing", label: "Pricing", description: "Plans that scale", icon: "credit-card" },
] satisfies NavLink[];

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

  useEffect(() => {
    if (!open) return;

    const previousOverflow = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    const closeOnEscape = (event: KeyboardEvent) => {
      if (event.key === "Escape") setOpen(false);
    };
    window.addEventListener("keydown", closeOnEscape);

    return () => {
      document.body.style.overflow = previousOverflow;
      window.removeEventListener("keydown", closeOnEscape);
    };
  }, [open]);

  useEffect(() => {
    setOpen(false);
  }, [pathname]);

  const links = [
    ...navLinks,
    ...cmsLinks
      .filter((page) => !navLinks.some((link) => link.href === `/${page.slug}`))
      .map((page): NavLink => ({
        href: `/${page.slug}`,
        label: page.navLabel ?? page.title,
        description: "Company information",
        icon: "book",
      })),
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
    <header className={`absolute inset-x-0 top-0 ${open ? "z-50" : "z-30"}`}>
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
            <div className="hidden md:block">
              <ThemeSwitcher onDark={onHero} />
            </div>
            <div className="hidden md:block">
              <Link href="/signup" style={{ textDecoration: "none" }}>
                <span className="lp-btn lp-btn--primary">
                  Start free trial
                  <Icon name="arrow-right" className="h-4 w-4" />
                </span>
              </Link>
            </div>
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
          <>
            <button
              type="button"
              aria-label="Close menu"
              onClick={() => {
                setOpen(false);
              }}
              className="lp-mobile-menu-backdrop fixed inset-0 z-[-1] bg-[#071426] md:hidden"
            />
            <div className="lp-mobile-menu fixed inset-x-0 bottom-0 top-[5rem] overflow-y-auto border-t border-white/10 bg-[#071426] px-4 pb-5 pt-6 text-white md:hidden">
              <div className="mx-auto flex min-h-full max-w-md flex-col">
                <div className="flex items-end justify-between gap-4 px-1">
                  <div>
                    <p className="text-sm font-medium text-white/55">Explore LaunchPad</p>
                    <p className="mt-1 text-2xl font-semibold tracking-tight">
                      Where do you want to go?
                    </p>
                  </div>
                  <Link
                    href={pathname.startsWith("/fr") ? "/" : "/fr"}
                    hrefLang={pathname.startsWith("/fr") ? "en" : "fr"}
                    onClick={() => {
                      setOpen(false);
                    }}
                    className="rounded-full border border-white/15 px-3 py-1.5 text-xs font-semibold text-white/75"
                  >
                    {pathname.startsWith("/fr") ? "EN" : "FR"}
                  </Link>
                </div>

                <nav className="mt-6 grid grid-cols-2 gap-2.5">
                  {links.map((link) => {
                    const active =
                      pathname === link.href || pathname.startsWith(`${link.href}/`);
                    return (
                      <Link
                        key={link.label}
                        href={link.href}
                        aria-current={active ? "page" : undefined}
                        onClick={() => {
                          setOpen(false);
                        }}
                        className={`group min-h-32 rounded-2xl border p-4 transition active:scale-[0.98] ${
                          active
                            ? "border-[#5f8de0] bg-[#173b73] text-white"
                            : "border-white/10 bg-white/[0.055] text-white hover:border-white/20 hover:bg-white/[0.09]"
                        }`}
                      >
                        <span
                          className={`grid h-9 w-9 place-items-center rounded-xl ${
                            active
                              ? "bg-white text-[#173b73]"
                              : "bg-white/10 text-white/75 group-hover:text-white"
                          }`}
                        >
                          <Icon name={link.icon} className="h-4.5 w-4.5" />
                        </span>
                        <span className="mt-4 block text-base font-semibold">{link.label}</span>
                        <span className="mt-1 block text-xs leading-5 text-white/55">
                          {link.description}
                        </span>
                      </Link>
                    );
                  })}
                </nav>

                <div className="mt-auto pt-6">
                  <div className="mb-3 flex items-center justify-between rounded-2xl border border-white/10 bg-white/[0.045] px-4 py-3">
                    <span className="text-sm text-white/60">Appearance</span>
                    <ThemeSwitcher onDark />
                  </div>
                  <div className="grid grid-cols-[0.9fr_1.1fr] gap-2.5">
                    <Link
                      href="/demo"
                      onClick={() => {
                        setOpen(false);
                      }}
                      className="grid min-h-12 place-items-center rounded-xl border border-white/20 px-4 text-sm font-semibold text-white transition hover:bg-white/10 active:scale-[0.98]"
                    >
                      Book a demo
                    </Link>
                    <Link
                      href="/signup"
                      onClick={() => {
                        setOpen(false);
                      }}
                      className="flex min-h-12 items-center justify-center gap-2 rounded-xl bg-white px-4 text-sm font-semibold text-[#102b55] transition hover:bg-[#eaf1ff] active:scale-[0.98]"
                    >
                      Start free trial
                      <Icon name="arrow-right" className="h-4 w-4" />
                    </Link>
                  </div>
                </div>
              </div>
            </div>
          </>
        ) : null}
      </div>
    </header>
  );
}
