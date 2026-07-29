import type { Metadata } from "next";
import { buildMetadata } from "../../lib/seo";
import { SignupForm } from "./signup-form";

export const metadata: Metadata = buildMetadata({
  title: "Start free trial — LaunchPad",
  description:
    "Create your organization and launch your first onboarding journey today — no credit card required.",
  path: "/signup",
});

export default function SignupPage() {
  return <SignupForm />;
}
