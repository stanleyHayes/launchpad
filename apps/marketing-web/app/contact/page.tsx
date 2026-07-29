import Link from "next/link";
import type { Metadata } from "next";
import { buildMetadata } from "../../lib/seo";
import { LegalSection, LegalShell } from "../legal-shell";
import { Icon } from "../ui-icon";

export const metadata: Metadata = buildMetadata({
  title: "Contact — LaunchPad",
  description:
    "Talk to the LaunchPad team: book a demo, ask a sales question, or reach product support.",
  path: "/contact",
});

export default function ContactPage() {
  return (
    <LegalShell
      eyebrow="Company"
      title="Contact us"
      intro="Questions about onboarding your team with LaunchPad? Here are the fastest ways to reach us."
    >
      <LegalSection heading="See LaunchPad in action">
        <p>
          The quickest way to evaluate LaunchPad is a guided demo: we walk through journeys,
          assignments, analytics, and the enterprise controls your security team will ask about.
        </p>
        <p>
          <Link href="/demo" style={{ textDecoration: "none" }}>
            <span className="lp-btn lp-btn--primary">
              Book a demo
              <Icon name="arrow-right" className="h-4 w-4" />
            </span>
          </Link>
        </p>
      </LegalSection>

      <LegalSection heading="Sales and general questions">
        <p>
          Pricing, plan fit, security reviews, and procurement:{" "}
          <a
            href="mailto:sales@launchpad.example"
            className="text-[var(--lp-brand)] hover:underline"
          >
            sales@launchpad.example
          </a>
          .
        </p>
      </LegalSection>

      <LegalSection heading="Product support">
        <p>
          Already a customer? Reach the support team at{" "}
          <a
            href="mailto:support@launchpad.example"
            className="text-[var(--lp-brand)] hover:underline"
          >
            support@launchpad.example
          </a>
          . Administrators can also raise tickets from the support area inside the product, where
          they are tracked with status and priority.
        </p>
      </LegalSection>

      <LegalSection heading="Security reports">
        <p>
          To report a vulnerability, please use{" "}
          <a
            href="mailto:security@launchpad.example"
            className="text-[var(--lp-brand)] hover:underline"
          >
            security@launchpad.example
          </a>{" "}
          — see our{" "}
          <Link href="/security" className="text-[var(--lp-brand)] hover:underline">
            security page
          </Link>{" "}
          for details.
        </p>
      </LegalSection>
    </LegalShell>
  );
}
