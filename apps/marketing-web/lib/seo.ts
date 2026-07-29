import type { Metadata } from "next";
import { siteUrl } from "../app/env";

export const siteName = "LaunchPad";

/**
 * Shared metadata builder for marketing routes: absolute canonical URL plus
 * matching Open Graph and Twitter cards, so every page carries the same
 * SEO shape without repeating boilerplate.
 */
export function buildMetadata({
  title,
  description,
  path = "/",
}: {
  title: string;
  description: string;
  path?: string;
}): Metadata {
  const url = new URL(path, siteUrl).toString();
  return {
    title,
    description,
    alternates: { canonical: url },
    openGraph: {
      title,
      description,
      url,
      siteName,
      type: "website",
    },
    twitter: {
      card: "summary_large_image",
      title,
      description,
    },
  };
}
