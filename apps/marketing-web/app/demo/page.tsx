import type { Metadata } from "next";
import { buildMetadata } from "../../lib/seo";
import { DemoForm } from "./demo-form";

export const metadata: Metadata = buildMetadata({
  title: "Book a demo — LaunchPad",
  description:
    "See LaunchPad in action — tell us about your team and we will follow up with a tailored walkthrough.",
  path: "/demo",
});

export default function DemoPage() {
  return <DemoForm />;
}
