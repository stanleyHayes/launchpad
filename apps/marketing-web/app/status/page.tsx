import type { Metadata } from "next";
import { Container } from "@launchpad/ui";
import { buildMetadata } from "../../lib/seo";
import { SiteHeader } from "../site-header";
import { SiteFooter } from "../site-footer";
import { StatusCheck } from "./status-check";

export const metadata: Metadata = buildMetadata({
  title: "Service status — LaunchPad",
  description: "LaunchPad service availability and component status.",
  path: "/status",
});

export default function StatusPage() {
  return (
    <main className="min-h-screen">
      <SiteHeader variant="light" />
      <section className="pb-20 pt-36">
        <Container className="max-w-3xl">
          <p className="lp-eyebrow">Service status</p>
          <h1 className="mt-4 text-5xl font-semibold tracking-tight">LaunchPad availability</h1>
          <p className="mt-4 text-[var(--lp-ink-muted)]">A live check of the public API edge. Detailed incident updates are shared with affected customers.</p>
          <StatusCheck />
        </Container>
      </section>
      <SiteFooter />
    </main>
  );
}
