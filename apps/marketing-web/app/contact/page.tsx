import Link from "next/link";
import type { Metadata } from "next";
import { buildMetadata } from "../../lib/seo";
import { getSiteInformation } from "../../lib/site-information";
import { LegalSection, LegalShell } from "../legal-shell";
import { Icon } from "../ui-icon";

export const metadata: Metadata = buildMetadata({
  title: "Contact | LaunchPad",
  description:
    "Talk to the LaunchPad team: book a demo, ask a sales question, or reach product support.",
  path: "/contact",
});

export const revalidate = 60;

export default async function ContactPage() {
  const siteInformation = await getSiteInformation();

  return (
    <LegalShell
      eyebrow="Company"
      title="Contact us"
      intro="Questions about onboarding your team with LaunchPad? Here are the fastest ways to reach us."
      responseTime={siteInformation.responseTime}
      icon="message"
      sections={[
        { id: "demo", label: "See LaunchPad in action" },
        { id: "sales", label: "Sales and general questions" },
        { id: "support", label: "Product support" },
        { id: "security", label: "Security reports" },
      ]}
      highlights={[
        { icon: "clock", label: "Response time", value: siteInformation.responseTime },
        { icon: "message", label: "Demo", value: "Tailored to your team" },
        { icon: "shield", label: "Security", value: "Direct reporting channel" },
      ]}
    >
      <LegalSection id="demo" heading="See LaunchPad in action">
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

      <LegalSection id="sales" heading="Sales and general questions">
        <p>
          Pricing, plan fit, security reviews, and procurement:{" "}
          <a
            href={`mailto:${siteInformation.salesEmail}`}
            className="text-[var(--lp-brand)] hover:underline"
          >
            {siteInformation.salesEmail}
          </a>
          .
        </p>
      </LegalSection>

      <LegalSection id="support" heading="Product support">
        <p>
          Already a customer? Reach the support team at{" "}
          <a
            href={`mailto:${siteInformation.supportEmail}`}
            className="text-[var(--lp-brand)] hover:underline"
          >
            {siteInformation.supportEmail}
          </a>
          . Administrators can also raise tickets from the support area inside the product, where
          they are tracked with status and priority.
        </p>
      </LegalSection>

      <LegalSection id="security" heading="Security reports">
        <p>
          To report a vulnerability, please use{" "}
          <a
            href={`mailto:${siteInformation.securityEmail}`}
            className="text-[var(--lp-brand)] hover:underline"
          >
            {siteInformation.securityEmail}
          </a>
          . See our{" "}
          <Link href="/security" className="text-[var(--lp-brand)] hover:underline">
            security page
          </Link>{" "}
          for details.
        </p>
      </LegalSection>
    </LegalShell>
  );
}
