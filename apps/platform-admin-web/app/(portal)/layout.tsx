import { PlatformShell } from "@/components/platform-shell";

export default function PortalLayout({ children }: { children: React.ReactNode }) {
  return <PlatformShell>{children}</PlatformShell>;
}
