import Link from "next/link";
import type { Metadata } from "next";
import { Container } from "@launchpad/ui";
import { buildMetadata } from "../../lib/seo";
import { orgAdminUrl } from "../env";
import { Icon, type IconName } from "../ui-icon";
import { SiteFooter } from "../site-footer";
import { SiteHeader } from "../site-header";

export const metadata: Metadata = buildMetadata({
  title: "Integrations — LaunchPad",
  description:
    "Connect LaunchPad to the tools your team already runs on: GitHub, Jira, BambooHR, OIDC SSO, SCIM 2.0, Slack, Microsoft Teams, and Anthropic AI.",
  path: "/integrations",
});

interface Integration {
  icon: IconName;
  name: string;
  capability: string;
  data: string;
  setup: string;
  setupHref?: string;
}

const live: Integration[] = [
  {
    icon: "plug",
    name: "GitHub",
    capability: "Link engineering resources and repositories to onboarding steps.",
    data: "Personal access token, validated on connect and stored encrypted; the repos and content you link to steps.",
    setup: "Organization admin → Integrations",
    setupHref: `${orgAdminUrl}/integrations`,
  },
  {
    icon: "workflow",
    name: "Jira",
    capability: "Sync onboarding tickets and tasks with your Jira site.",
    data: "Site URL and API token, validated on connect and stored encrypted; the issues linked to journeys.",
    setup: "Organization admin → Integrations",
    setupHref: `${orgAdminUrl}/integrations`,
  },
  {
    icon: "users",
    name: "BambooHR",
    capability: "Import your employee directory and keep it in sync.",
    data: "HRIS API key; directory records — name, role, department, manager, and start date.",
    setup: "Organization admin — HRIS sync",
  },
  {
    icon: "lock",
    name: "OIDC SSO",
    capability: "Let people sign in with Okta, Microsoft Entra, or any OIDC provider.",
    data: "OIDC client credentials; sign-in claims such as email, name, and groups.",
    setup: "Organization admin → Settings (SSO)",
    setupHref: `${orgAdminUrl}/settings`,
  },
  {
    icon: "shield",
    name: "SCIM 2.0",
    capability: "Provision and deprovision users and groups from your identity provider.",
    data: "Expiring, auditable tenant token; user and group provisioning records.",
    setup: "Organization admin → Settings (SSO)",
    setupHref: `${orgAdminUrl}/settings`,
  },
  {
    icon: "message",
    name: "Slack",
    capability: "Onboarding alerts in the channel your team actually reads.",
    data: "Incoming webhook URL, stored write-only; step assigned and approved notifications.",
    setup: "Organization admin → Settings → Notifications",
    setupHref: `${orgAdminUrl}/settings`,
  },
  {
    icon: "bell",
    name: "Microsoft Teams",
    capability: "The same onboarding alerts, delivered to a Teams channel.",
    data: "Incoming webhook URL, stored write-only; step assigned and approved notifications.",
    setup: "Organization admin → Settings → Notifications",
    setupHref: `${orgAdminUrl}/settings`,
  },
  {
    icon: "sparkles",
    name: "Anthropic AI",
    capability: "Grounded assistant answers, with citations from your approved knowledge.",
    data: "Approved knowledge passages and the questions asked; every answer cites its sources.",
    setup: "Built in — enabled for every organization",
  },
];

const comingNext: { icon: IconName; name: string; capability: string }[] = [
  {
    icon: "clock",
    name: "Google Calendar",
    capability: "Schedule onboarding meetings straight from journey steps.",
  },
  {
    icon: "clock",
    name: "Microsoft 365 Calendar",
    capability: "Book intros, check-ins, and training sessions in Outlook.",
  },
];

function StatusBadge({ live: isLive }: { live: boolean }) {
  return (
    <span
      className={`inline-flex items-center gap-1.5 rounded-full px-2.5 py-1 text-xs font-semibold ${
        isLive
          ? "bg-[var(--lp-success)]/12 text-[var(--lp-success)]"
          : "bg-[var(--lp-brand-soft)] text-[var(--lp-ink-muted)]"
      }`}
    >
      {isLive ? "Live" : "Coming next"}
    </span>
  );
}

export default function IntegrationsPage() {
  return (
    <main>
      {/* Hero ------------------------------------------------------------- */}
      <section className="lp-hero relative">
        <SiteHeader variant="hero" />
        <Container className="pb-24 pt-36 lg:pt-40">
          <p className="lp-rise lp-eyebrow lp-eyebrow--on-dark">Integrations</p>
          <h1
            className="lp-rise mt-6 max-w-2xl text-4xl font-semibold leading-[1.08] tracking-tight md:text-6xl"
            style={{ fontFamily: "var(--lp-font-display)" }}
          >
            Onboarding that plugs into your stack.
          </h1>
          <p className="lp-rise-delay mt-6 max-w-xl text-lg leading-8 text-white/80">
            LaunchPad connects to the tools your team already runs on — identity,
            HRIS, engineering, and chat — so journeys assign themselves, people
            sign in with what they have, and alerts land where they get read.
          </p>
          <div className="lp-rise-delay-2 mt-9 flex flex-wrap items-center gap-3">
            <Link href="/signup" style={{ textDecoration: "none" }}>
              <span className="lp-btn lp-btn--primary">
                Start free trial
                <Icon name="arrow-right" className="h-4 w-4" />
              </span>
            </Link>
            <Link href="/demo" style={{ textDecoration: "none" }}>
              <span className="lp-btn lp-btn--on-dark">Book a demo</span>
            </Link>
          </div>
        </Container>
      </section>

      {/* Live integrations ------------------------------------------------- */}
      <section className="py-24">
        <Container>
          <div className="max-w-2xl">
            <p className="lp-eyebrow">Available today</p>
            <h2
              className="mt-4 text-3xl font-semibold tracking-tight text-[var(--lp-ink)] md:text-4xl"
              style={{ fontFamily: "var(--lp-font-display)" }}
            >
              Every integration here is live
            </h2>
            <p className="mt-4 leading-7 text-[var(--lp-ink-muted)]">
              What you see is what ships: each entry lists the data that flows
              and where an organization admin connects it.
            </p>
          </div>

          <div className="mt-14 grid gap-6 md:grid-cols-2 lg:grid-cols-3">
            {live.map((integration) => (
              <div key={integration.name} className="lp-card lp-feature-card flex flex-col p-7">
                <div className="flex items-start justify-between gap-3">
                  <span className="lp-icon-chip" aria-hidden="true">
                    <Icon name={integration.icon} className="h-5 w-5" />
                  </span>
                  <StatusBadge live />
                </div>
                <h3
                  className="mt-5 text-lg font-semibold text-[var(--lp-ink)]"
                  style={{ fontFamily: "var(--lp-font-display)" }}
                >
                  {integration.name}
                </h3>
                <p className="mt-2 text-sm leading-6 text-[var(--lp-ink-muted)]">
                  {integration.capability}
                </p>
                <dl className="mt-5 space-y-3 border-t border-[var(--lp-border)] pt-5 text-sm">
                  <div>
                    <dt className="text-xs font-semibold uppercase tracking-wide text-[var(--lp-ink-muted)]">
                      Data that flows
                    </dt>
                    <dd className="mt-1 leading-6 text-[var(--lp-ink)]">{integration.data}</dd>
                  </div>
                  <div>
                    <dt className="text-xs font-semibold uppercase tracking-wide text-[var(--lp-ink-muted)]">
                      Setup
                    </dt>
                    <dd className="mt-1 leading-6 text-[var(--lp-ink)]">
                      {integration.setupHref ? (
                        <a
                          href={integration.setupHref}
                          className="font-medium text-[var(--lp-brand)] hover:underline"
                        >
                          {integration.setup}
                        </a>
                      ) : (
                        integration.setup
                      )}
                    </dd>
                  </div>
                </dl>
              </div>
            ))}
          </div>
        </Container>
      </section>

      {/* Coming next -------------------------------------------------------- */}
      <section className="bg-[var(--lp-paper-elevated)] py-24">
        <Container>
          <div className="max-w-2xl">
            <p className="lp-eyebrow">On the roadmap</p>
            <h2
              className="mt-4 text-3xl font-semibold tracking-tight text-[var(--lp-ink)] md:text-4xl"
              style={{ fontFamily: "var(--lp-font-display)" }}
            >
              Coming next
            </h2>
            <p className="mt-4 leading-7 text-[var(--lp-ink-muted)]">
              Calendar scheduling is on the roadmap. These are not available
              yet — we list them so you can plan, not to sell vaporware.
            </p>
          </div>

          <div className="mt-14 grid gap-6 md:grid-cols-2">
            {comingNext.map((integration) => (
              <div key={integration.name} className="lp-card lp-feature-card p-7">
                <div className="flex items-start justify-between gap-3">
                  <span className="lp-icon-chip" aria-hidden="true">
                    <Icon name={integration.icon} className="h-5 w-5" />
                  </span>
                  <StatusBadge live={false} />
                </div>
                <h3
                  className="mt-5 text-lg font-semibold text-[var(--lp-ink)]"
                  style={{ fontFamily: "var(--lp-font-display)" }}
                >
                  {integration.name}
                </h3>
                <p className="mt-2 text-sm leading-6 text-[var(--lp-ink-muted)]">
                  {integration.capability}
                </p>
              </div>
            ))}
          </div>
        </Container>
      </section>

      {/* CTA band ---------------------------------------------------------- */}
      <section className="py-24">
        <Container>
          <div className="lp-hero relative overflow-hidden rounded-3xl px-8 py-16 text-center">
            <h2
              className="mx-auto max-w-2xl text-3xl font-semibold tracking-tight md:text-4xl"
              style={{ fontFamily: "var(--lp-font-display)" }}
            >
              Connect your stack on day one
            </h2>
            <p className="mx-auto mt-4 max-w-xl text-lg text-white/75">
              Start free today, or book a walkthrough tailored to your team.
            </p>
            <div className="mt-8 flex flex-wrap justify-center gap-3">
              <Link href="/signup" style={{ textDecoration: "none" }}>
                <span className="lp-btn lp-btn--primary">
                  Start free trial
                  <Icon name="arrow-right" className="h-4 w-4" />
                </span>
              </Link>
              <Link href="/demo" style={{ textDecoration: "none" }}>
                <span className="lp-btn lp-btn--on-dark">Book a demo</span>
              </Link>
            </div>
          </div>
        </Container>
      </section>

      <SiteFooter />
    </main>
  );
}
