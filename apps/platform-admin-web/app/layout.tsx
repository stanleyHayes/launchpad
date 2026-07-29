import type { Metadata } from "next";
import { Outfit } from "next/font/google";
import { lpThemeInitScript } from "@launchpad/ui";
import "./globals.css";

const outfit = Outfit({
  subsets: ["latin"],
  variable: "--lp-font",
});

export const metadata: Metadata = {
  title: "LaunchPad — Platform Admin",
  description: "Manage organizations, leads, and platform operations.",
  robots: { index: false },
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="en" suppressHydrationWarning>
      <head>
        <script dangerouslySetInnerHTML={{ __html: lpThemeInitScript }} />
      </head>
      <body suppressHydrationWarning className={`${outfit.variable} antialiased`}>
        {children}
      </body>
    </html>
  );
}
