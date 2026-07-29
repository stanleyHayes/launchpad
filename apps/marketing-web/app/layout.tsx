import type { Metadata } from "next";
import { Outfit } from "next/font/google";
import { lpThemeInitScript } from "@launchpad/ui";
import { siteUrl } from "./env";
import { buildMetadata, siteName } from "../lib/seo";
import "./globals.css";
import { PrivacyControls } from "./privacy-controls";
import { Suspense } from "react";

const outfit = Outfit({
  subsets: ["latin"],
  variable: "--lp-font",
});

export const metadata: Metadata = {
  metadataBase: new URL(siteUrl),
  ...buildMetadata({
    title: "LaunchPad — Employee onboarding, orchestrated",
    description:
      "Build guided onboarding journeys, automate setup, and measure time-to-productivity.",
  }),
};

const jsonLd = JSON.stringify({
  "@context": "https://schema.org",
  "@graph": [
    {
      "@type": "Organization",
      name: siteName,
      url: siteUrl,
      logo: `${siteUrl}/icon.svg`,
    },
    {
      "@type": "WebSite",
      name: siteName,
      url: siteUrl,
    },
  ],
}).replace(/</g, "\\u003c");

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="en" suppressHydrationWarning>
      <head>
        <script dangerouslySetInnerHTML={{ __html: lpThemeInitScript }} />
        <script
          type="application/ld+json"
          dangerouslySetInnerHTML={{ __html: jsonLd }}
        />
      </head>
      <body className={`${outfit.variable} antialiased`}>
        {children}
        <Suspense fallback={null}>
          <PrivacyControls />
        </Suspense>
      </body>
    </html>
  );
}
