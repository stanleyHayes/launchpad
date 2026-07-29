import type { Metadata } from "next";
import { buildMetadata } from "../../lib/seo";
import { getSiteInformation } from "../../lib/site-information";
import { LegalSection, LegalShell } from "../legal-shell";

export const metadata: Metadata = buildMetadata({
  title: "Privacy Policy | LaunchPad",
  description:
    "What data LaunchPad collects, which processors handle it, how long we keep it, and the rights you have over it.",
  path: "/privacy",
});

export const revalidate = 60;

export default async function PrivacyPage() {
  const siteInformation = await getSiteInformation();

  return (
    <LegalShell
      eyebrow="Legal"
      title="Privacy Policy"
      intro="How LaunchPad collects, uses, and protects personal data when you use our onboarding platform."
      updated={siteInformation.privacyEffectiveDate}
      icon="lock"
      sections={[
        { id: "scope", label: "Who we are" },
        { id: "data-collected", label: "Data we collect" },
        { id: "data-use", label: "How we use data" },
        { id: "processors", label: "Processors and subprocessors" },
        { id: "retention", label: "Retention" },
        { id: "rights", label: "Your rights" },
        { id: "cookies", label: "Cookies" },
        { id: "changes", label: "Changes and contact" },
      ]}
      highlights={[
        { icon: "users", label: "Our role", value: "Data processor" },
        { icon: "eye-off", label: "Advertising", value: "No third-party trackers" },
        { icon: "check", label: "Your rights", value: "Access, export, delete" },
      ]}
    >
      <LegalSection id="scope" heading="Who we are and what this covers">
        <p>
          LaunchPad, Inc. (&ldquo;LaunchPad&rdquo;, &ldquo;we&rdquo;) operates a multi-tenant
          employee onboarding platform. This policy describes how we handle personal data on our
          marketing site and inside the LaunchPad service. When an organization uses LaunchPad to
          onboard its employees, that organization is the data controller for its employees&rsquo;
          data and LaunchPad acts as a processor on its behalf.
        </p>
      </LegalSection>

      <LegalSection id="data-collected" heading="Data we collect">
        <ul className="list-disc space-y-2 pl-5">
          <li>
            <strong className="text-[var(--lp-ink)]">Account data</strong>: names, work email
            addresses, and roles for the people your organization invites to LaunchPad.
          </li>
          <li>
            <strong className="text-[var(--lp-ink)]">Organization data</strong>: departments,
            journeys, assignments, approvals, and the content your team authors in the platform.
          </li>
          <li>
            <strong className="text-[var(--lp-ink)]">Usage data</strong>: audit and analytics
            events (sign-ins, actions taken, feature usage) used to operate, secure, and improve
            the service.
          </li>
          <li>
            <strong className="text-[var(--lp-ink)]">Integration data</strong>: employee records
            synced from HRIS systems your organization connects, such as BambooHR, at your
            organization&rsquo;s direction.
          </li>
          <li>
            <strong className="text-[var(--lp-ink)]">Communications</strong>: messages you send us
            through demos, support, or security contacts.
          </li>
        </ul>
      </LegalSection>

      <LegalSection id="data-use" heading="How we use data">
        <p>
          We use personal data to provide the service (running journeys, assignments, approvals,
          and notifications), to authenticate users, to keep the platform secure, and to produce
          aggregated onboarding analytics for your organization. AI assistant answers are generated
          from your organization&rsquo;s own knowledge content. We do not sell personal data.
        </p>
      </LegalSection>

      <LegalSection id="processors" heading="Processors and subprocessors">
        <p>LaunchPad relies on the following categories of service providers to operate:</p>
        <ul className="list-disc space-y-2 pl-5">
          <li>MongoDB: primary datastore for the service.</li>
          <li>Redis: caching and transient queue state.</li>
          <li>
            Anthropic: generates AI assistant answers; relevant knowledge content may be sent to
            Anthropic to produce a response.
          </li>
          <li>
            BambooHR and other HRIS providers: employee data sync, only when your organization
            connects them.
          </li>
          <li>
            OIDC identity providers: your organization&rsquo;s identity provider handles
            single sign-on authentication when SSO is configured.
          </li>
        </ul>
      </LegalSection>

      <LegalSection id="retention" heading="Retention">
        <p>
          We retain personal data for as long as your organization&rsquo;s account is active or as
          needed to provide the service. When an organization deletes data or closes its account,
          we delete or de-identify the associated personal data within a reasonable period, except
          where we must keep records to meet legal obligations. Audit events are retained to
          support the integrity of the audit trail.
        </p>
      </LegalSection>

      <LegalSection id="rights" heading="Your rights">
        <p>
          Depending on your jurisdiction, you may have rights to access, correct, export, or delete
          your personal data, and to object to or restrict certain processing. Employees of a
          customer organization should direct requests to their organization&rsquo;s administrator
          first, since that organization controls the data. You can also reach us at{" "}
          <a
            href={`mailto:${siteInformation.privacyEmail}`}
            className="text-[var(--lp-brand)] hover:underline"
          >
            {siteInformation.privacyEmail}
          </a>{" "}
          and we will route export and deletion requests to the right place.
        </p>
      </LegalSection>

      <LegalSection id="cookies" heading="Cookies">
        <p>
          LaunchPad uses strictly necessary cookies for sign-in sessions and interface preferences
          (such as theme). We do not use third-party advertising trackers on the marketing site or
          in the product.
        </p>
      </LegalSection>

      <LegalSection id="changes" heading="Changes and contact">
        <p>
          We may update this policy as the product evolves; material changes will be reflected with
          a new &ldquo;last updated&rdquo; date above. Questions about this policy go to{" "}
          <a
            href={`mailto:${siteInformation.privacyEmail}`}
            className="text-[var(--lp-brand)] hover:underline"
          >
            {siteInformation.privacyEmail}
          </a>
          .
        </p>
      </LegalSection>
    </LegalShell>
  );
}
