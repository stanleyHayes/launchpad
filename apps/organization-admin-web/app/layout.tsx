import type { Metadata } from "next";
import { Fraunces, Sora } from "next/font/google";
import "./globals.css";

const display = Fraunces({
  subsets: ["latin"],
  variable: "--lp-font-display",
});

const body = Sora({
  subsets: ["latin"],
  variable: "--lp-font-body",
});

export const metadata: Metadata = {
  title: "LaunchPad — Organization Admin",
  description: "Manage onboarding journeys, employees, and organization settings.",
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="en">
      <body className={`${display.variable} ${body.variable} antialiased`}>
        {children}
      </body>
    </html>
  );
}
