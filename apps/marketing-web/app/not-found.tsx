import Link from "next/link";
import { Container, LogoTile } from "@launchpad/ui";
import { SiteFooter } from "./site-footer";
import { SiteHeader } from "./site-header";

export default function NotFound() {
  return (
    <main className="relative min-h-screen">
      <SiteHeader variant="light" />
      <section className="pb-24 pt-40">
        <Container className="max-w-xl text-center">
          <div className="flex justify-center">
            <LogoTile size={56} />
          </div>
          <p className="lp-eyebrow mt-8 justify-center">404 — Page not found</p>
          <h1
            className="mt-4 text-4xl font-semibold tracking-tight text-[var(--lp-ink)]"
            style={{ fontFamily: "var(--lp-font-display)" }}
          >
            This page took a different onboarding path
          </h1>
          <p className="mt-4 leading-7 text-[var(--lp-ink-muted)]">
            The link may be outdated or the page may have moved. Let us get you
            back on track.
          </p>
          <div className="mt-8 flex flex-wrap justify-center gap-3">
            <Link href="/" style={{ textDecoration: "none" }}>
              <span className="lp-btn lp-btn--primary">Back to home</span>
            </Link>
            <Link href="/demo" style={{ textDecoration: "none" }}>
              <span className="lp-btn lp-btn--secondary">Book a demo</span>
            </Link>
          </div>
        </Container>
      </section>
      <SiteFooter />
    </main>
  );
}
