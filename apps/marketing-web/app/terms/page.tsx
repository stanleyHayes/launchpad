import Link from "next/link";
import type { Metadata } from "next";
import { buildMetadata } from "../../lib/seo";
import { LegalSection, LegalShell } from "../legal-shell";

export const metadata: Metadata = buildMetadata({
  title: "Terms of Service — LaunchPad",
  description:
    "The terms that govern use of the LaunchPad onboarding platform: accounts, acceptable use, data ownership, and service levels.",
  path: "/terms",
});

export default function TermsPage() {
  return (
    <LegalShell
      eyebrow="Legal"
      title="Terms of Service"
      intro="The agreement between LaunchPad, Inc. and the organizations that use our onboarding platform."
      updated="July 28, 2026"
    >
      <LegalSection heading="The service">
        <p>
          LaunchPad is a multi-tenant software-as-a-service platform for building and running
          employee onboarding journeys: templates, assignments, approvals, notifications,
          analytics, integrations, and an AI assistant grounded in your organization&rsquo;s
          content. These terms govern access to the service. If your organization has a signed
          agreement with LaunchPad, that agreement controls where it conflicts with these terms.
        </p>
      </LegalSection>

      <LegalSection heading="Accounts and responsibilities">
        <p>
          Each organization (&ldquo;customer&rdquo;) designates administrators who manage its
          workspace, users, roles, and integrations. Customers are responsible for the accuracy of
          the data they load into the service, for configuring access appropriately, and for the
          activity that occurs under their accounts.
        </p>
      </LegalSection>

      <LegalSection heading="Acceptable use">
        <p>You agree not to:</p>
        <ul className="list-disc space-y-2 pl-5">
          <li>Use the service for anything unlawful, or to infringe others&rsquo; rights.</li>
          <li>
            Attempt to access another tenant&rsquo;s data, probe or bypass security controls, or
            interfere with the service&rsquo;s operation.
          </li>
          <li>Upload malicious code or content you have no right to share.</li>
          <li>
            Exceed reasonable use of rate-limited endpoints or attempt to disrupt availability.
          </li>
        </ul>
        <p>We may suspend access where use threatens the service or other customers.</p>
      </LegalSection>

      <LegalSection heading="Your data">
        <p>
          Customers own the data they put into LaunchPad — employee records, journey content,
          assignments, and configuration. We process that data only to provide and improve the
          service, as described in our{" "}
          <Link href="/privacy" className="text-[var(--lp-brand)] hover:underline">
            Privacy Policy
          </Link>
          . You can export or delete your data at any time; see the Privacy Policy for how to make
          a request.
        </p>
      </LegalSection>

      <LegalSection heading="Subscriptions and billing">
        <p>
          Paid plans are billed as described on our pricing page or in your order form. Trials
          convert to paid plans only when you choose one. You can cancel at any time and keep
          access until the end of the paid period.
        </p>
      </LegalSection>

      <LegalSection heading="Service levels">
        <p>
          We design and operate LaunchPad for high availability. Enterprise plans include
          SLA-backed support with response and uptime commitments set out in the customer&rsquo;s
          order form. Except as committed in a signed SLA, the service is provided &ldquo;as
          is&rdquo; and we do not warrant uninterrupted or error-free operation.
        </p>
      </LegalSection>

      <LegalSection heading="Liability">
        <p>
          To the maximum extent permitted by law, neither party is liable for indirect or
          consequential damages, and each party&rsquo;s aggregate liability is limited to the
          amounts paid or payable for the service in the twelve months before the claim, except
          where liability cannot be limited by law.
        </p>
      </LegalSection>

      <LegalSection heading="Termination and changes">
        <p>
          Either party may terminate for material breach that is not cured after notice. On
          termination we make customer data available for export for a reasonable period before
          deletion. We may update these terms from time to time; the &ldquo;last updated&rdquo;
          date above reflects the current version, and material changes will be communicated to
          account administrators.
        </p>
      </LegalSection>

      <LegalSection heading="Contact">
        <p>
          Questions about these terms:{" "}
          <a
            href="mailto:legal@launchpad.example"
            className="text-[var(--lp-brand)] hover:underline"
          >
            legal@launchpad.example
          </a>
          .
        </p>
      </LegalSection>
    </LegalShell>
  );
}
