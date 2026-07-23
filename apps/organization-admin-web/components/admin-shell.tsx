"use client";

import { useEffect, useState, type ReactNode } from "react";
import { usePathname, useRouter } from "next/navigation";
import type { MeResponse } from "@launchpad/api-client";
import { ApiError } from "@launchpad/api-client";
import { PortalShell, type NavGroup } from "@launchpad/ui";
import { getClient } from "@/lib/api";
import { clearSession, getAccessToken } from "@/lib/session";

export const orgAdminNav: NavGroup[] = [
  {
    heading: "Operations",
    items: [
      { label: "Overview", href: "/dashboard" },
      { label: "Analytics", href: "/analytics" },
      { label: "Journeys", href: "/journeys" },
      { label: "Approvals", href: "/approvals" },
    ],
  },
  {
    heading: "People",
    items: [{ label: "Employees", href: "/employees" }],
  },
  {
    heading: "Account",
    items: [
      { label: "Billing", href: "/billing" },
      { label: "Support", href: "/support" },
    ],
  },
];

export function AdminShell({ children }: { children: ReactNode }) {
  const router = useRouter();
  const pathname = usePathname();
  const [me, setMe] = useState<MeResponse | null>(null);

  useEffect(() => {
    if (!getAccessToken()) {
      router.replace("/login");
      return;
    }

    void (async () => {
      try {
        setMe(await getClient().me());
      } catch (err) {
        if (err instanceof ApiError && err.status === 401) {
          clearSession();
          router.replace("/login");
        }
      }
    })();
  }, [router]);

  async function onLogout() {
    try {
      await getClient().logout();
    } catch {
      // Session may already be invalid.
    }
    clearSession();
    router.replace("/login");
  }

  return (
    <PortalShell
      pathname={pathname}
      onNavigate={(href) => {
        router.push(href);
      }}
      groups={orgAdminNav}
      orgLabel={me?.organization?.name}
      userLabel={me ? `${me.user.displayName} · ${me.roleCode}` : "Loading…"}
      workspaceLabel="Onboarding command centre"
      onLogout={() => {
        void onLogout();
      }}
    >
      {children}
    </PortalShell>
  );
}
