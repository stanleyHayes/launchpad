import { cache } from "react";
import { z } from "zod";
import { createLaunchPadClient } from "@launchpad/api-client";
import { apiBaseUrl } from "../app/env";

const siteInformationSchema = z.object({
  salesEmail: z.email(),
  supportEmail: z.email(),
  securityEmail: z.email(),
  privacyEmail: z.email(),
  legalEmail: z.email(),
  responseTime: z.string().min(1).max(80),
  securityEffectiveDate: z.string().min(1).max(40),
  termsEffectiveDate: z.string().min(1).max(40),
  privacyEffectiveDate: z.string().min(1).max(40),
  dpaEffectiveDate: z.string().min(1).max(40),
});

export type SiteInformation = z.infer<typeof siteInformationSchema>;

export const defaultSiteInformation: SiteInformation = {
  salesEmail: "sales@launchpad.example",
  supportEmail: "support@launchpad.example",
  securityEmail: "security@launchpad.example",
  privacyEmail: "privacy@launchpad.example",
  legalEmail: "legal@launchpad.example",
  responseTime: "One business day",
  securityEffectiveDate: "July 28, 2026",
  termsEffectiveDate: "July 28, 2026",
  privacyEffectiveDate: "July 28, 2026",
  dpaEffectiveDate: "July 29, 2026",
};

export const getSiteInformation = cache(async (): Promise<SiteInformation> => {
  try {
    const client = createLaunchPadClient({ baseUrl: apiBaseUrl });
    const page = await client.getPublishedCMSPage("site-information");
    return siteInformationSchema.parse(JSON.parse(page.body));
  } catch {
    // Public pages must stay available while the API is offline or before the
    // settings record is first published.
    return defaultSiteInformation;
  }
});
