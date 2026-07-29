"use client";

import Link from "next/link";
import { Container, LogoTile } from "@launchpad/ui";

export default function Error({
  reset,
}: {
  error: Error & { digest?: string };
  reset: () => void;
}) {
  return (
    <main className="relative grid min-h-screen place-items-center">
      <Container className="max-w-xl py-24 text-center">
        <div className="flex justify-center">
          <LogoTile size={56} />
        </div>
        <p className="lp-eyebrow mt-8 justify-center">Something went wrong</p>
        <h1
          className="mt-4 text-4xl font-semibold tracking-tight text-[var(--lp-ink)]"
          style={{ fontFamily: "var(--lp-font-display)" }}
        >
          We hit an unexpected snag
        </h1>
        <p className="mt-4 leading-7 text-[var(--lp-ink-muted)]">
          The page failed to load. Try again — if it keeps happening, reach out
          and we will take a look.
        </p>
        <div className="mt-8 flex flex-wrap justify-center gap-3">
          <button type="button" onClick={reset} className="lp-btn lp-btn--primary">
            Try again
          </button>
          <Link href="/" style={{ textDecoration: "none" }}>
            <span className="lp-btn lp-btn--secondary">Back to home</span>
          </Link>
        </div>
      </Container>
    </main>
  );
}
