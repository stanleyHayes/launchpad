import type { Metadata } from "next";
import { buildMetadata } from "../../lib/seo";
import { LegalSection, LegalShell } from "../legal-shell";

export const metadata: Metadata = buildMetadata({
  title: "Data Processing Addendum — LaunchPad",
  description: "LaunchPad's controller–processor terms, security commitments, subprocessors, and data-subject assistance.",
  path: "/dpa",
});

export default function DPAPage() {
  return (
    <LegalShell eyebrow="Legal" title="Data Processing Addendum" intro="The terms that govern LaunchPad's processing of customer personal data." updated="July 29, 2026">
      <LegalSection heading="Scope and roles">
        <p>This addendum forms part of the customer agreement. The customer is the controller and LaunchPad is the processor for personal data submitted to the service. We process that data only to provide, secure, support, and improve the contracted service and on documented customer instructions.</p>
      </LegalSection>
      <LegalSection heading="Confidentiality and security">
        <p>Personnel with access to customer data are bound by confidentiality obligations. LaunchPad maintains role-based access, tenant isolation, encryption in transit, configurable encryption for stored secrets, audit logging, vulnerability scanning, backups, and incident-response procedures appropriate to the risk.</p>
      </LegalSection>
      <LegalSection heading="Subprocessors and transfers">
        <p>LaunchPad may use infrastructure, email, AI, and integration subprocessors described in our Privacy Policy. We remain responsible for their processing obligations and will provide reasonable notice of material changes. Cross-border transfers use a lawful transfer mechanism where required.</p>
      </LegalSection>
      <LegalSection heading="Data-subject requests and incidents">
        <p>We provide tenant export and deletion tools and will reasonably assist the customer with access, correction, restriction, portability, deletion, security assessments, and breach notifications. Customers remain responsible for validating and responding to requests from their people.</p>
      </LegalSection>
      <LegalSection heading="Deletion and audits">
        <p>At termination or documented request, LaunchPad will delete or return customer personal data except where retention is legally required. We will make relevant compliance information available and support a proportionate audit process subject to confidentiality and security controls.</p>
      </LegalSection>
      <LegalSection heading="Contact">
        <p>To execute this addendum or ask a privacy question, contact <a className="text-[var(--lp-brand)] hover:underline" href="mailto:privacy@launchpad.example">privacy@launchpad.example</a>.</p>
      </LegalSection>
    </LegalShell>
  );
}
