import Link from "next/link";
import type { Metadata } from "next";
import { buildMetadata } from "../../lib/seo";
import { getSiteInformation } from "../../lib/site-information";
import { LegalSection, LegalShell } from "../legal-shell";

export const metadata: Metadata = buildMetadata({
  title: "Security | LaunchPad",
  description:
    "How LaunchPad protects customer data: SSO, SCIM provisioning, role-based access, audit trails, tenant isolation, and encryption.",
  path: "/security",
});

export const revalidate = 60;

export default async function SecurityPage() {
  const siteInformation = await getSiteInformation();

  return (
    <LegalShell
      eyebrow="Company"
      title="Security at LaunchPad"
      intro="The controls built into the LaunchPad platform, described plainly so your security team can evaluate them."
      updated={siteInformation.securityEffectiveDate}
      icon="shield"
      sections={[
        { id: "identity", label: "Identity and access" },
        { id: "data-protection", label: "Data protection" },
        { id: "application-security", label: "Application security" },
        { id: "compliance", label: "Compliance posture" },
        { id: "reporting", label: "Report a vulnerability" },
      ]}
      highlights={[
        { icon: "lock", label: "Access", value: "SSO, SCIM and RBAC" },
        { icon: "building", label: "Architecture", value: "Tenant isolated" },
        { icon: "eye", label: "Traceability", value: "Audited actions" },
      ]}
    >
      <LegalSection id="identity" heading="Identity and access">
        <ul className="list-disc space-y-2 pl-5">
          <li>
            <strong className="text-[var(--lp-ink)]">Single sign-on (OIDC)</strong>: organizations
            authenticate users through their own identity provider on Enterprise plans.
          </li>
          <li>
            <strong className="text-[var(--lp-ink)]">SCIM 2.0 provisioning</strong>: user and group
            lifecycle managed from your identity provider, so departures lose access when you
            deprovision them.
          </li>
          <li>
            <strong className="text-[var(--lp-ink)]">Role-based access control</strong>: every
            tenant write is gated by resource-and-action permission checks; platform staff routes
            are separated from tenant routes.
          </li>
        </ul>
      </LegalSection>

      <LegalSection id="data-protection" heading="Data protection">
        <ul className="list-disc space-y-2 pl-5">
          <li>
            <strong className="text-[var(--lp-ink)]">Tenant isolation</strong>: LaunchPad is
            multi-tenant by design; data access is scoped to the requesting organization
            throughout the service.
          </li>
          <li>
            <strong className="text-[var(--lp-ink)]">Encryption in transit</strong>: traffic to
            the service is served over TLS.
          </li>
          <li>
            <strong className="text-[var(--lp-ink)]">Encrypted secrets at rest</strong>:
            integration credentials and other stored secrets are encrypted with AES-256 before
            they are written to the database.
          </li>
          <li>
            <strong className="text-[var(--lp-ink)]">Secrets management</strong>: application
            secrets live in environment configuration, never in source code.
          </li>
        </ul>
      </LegalSection>

      <LegalSection id="application-security" heading="Application security">
        <ul className="list-disc space-y-2 pl-5">
          <li>
            <strong className="text-[var(--lp-ink)]">Audit trail</strong>: privileged actions are
            recorded as audit events so administrators can review who did what.
          </li>
          <li>
            <strong className="text-[var(--lp-ink)]">Rate limiting</strong>: sensitive endpoints
            are rate-limited per client to resist abuse and credential attacks.
          </li>
          <li>
            <strong className="text-[var(--lp-ink)]">Security headers</strong>: responses carry
            hardened headers including Content-Security-Policy, Strict-Transport-Security, and
            frame and content-type protections.
          </li>
          <li>
            <strong className="text-[var(--lp-ink)]">Health and readiness gates</strong>: launch
            checks verify indexes, configuration, and connectivity before the service goes live.
          </li>
        </ul>
      </LegalSection>

      <LegalSection id="compliance" heading="Compliance posture">
        <p>
          LaunchPad is designed to support customer compliance programs, including GDPR-style data
          subject rights such as export and deletion (see our{" "}
          <Link href="/privacy" className="text-[var(--lp-brand)] hover:underline">
            Privacy Policy
          </Link>
          ). We do not currently hold SOC 2 or ISO 27001 certifications; the controls above are
          designed with those frameworks in mind, and we are happy to walk your security team
          through the details during an Enterprise evaluation.
        </p>
      </LegalSection>

      <LegalSection id="reporting" heading="Reporting a vulnerability">
        <p>
          If you believe you have found a security issue in LaunchPad, please report it to{" "}
          <a
            href={`mailto:${siteInformation.securityEmail}`}
            className="text-[var(--lp-brand)] hover:underline"
          >
            {siteInformation.securityEmail}
          </a>
          . We ask that you give us a reasonable window to investigate and fix the issue before any
          public disclosure.
        </p>
      </LegalSection>
    </LegalShell>
  );
}
