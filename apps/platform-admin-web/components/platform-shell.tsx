"use client";

import { useCallback, useEffect, useState, type ReactNode } from "react";
import { usePathname, useRouter } from "next/navigation";
import type { MeResponse } from "@launchpad/api-client";
import { ApiError } from "@launchpad/api-client";
import { Button, PortalShell, type IconName, type NavGroup } from "@launchpad/ui";
import { getClient } from "@/lib/api";
import { clearSession, getAccessToken } from "@/lib/session";

type RoleNavItem = { label: string; href: string; icon?: IconName; roles?: string[] };
type RoleNavGroup = { heading: string; items: RoleNavItem[] };

const platformRoleCodes = new Set([
  "platform_owner",
  "platform_admin",
  "support_agent",
  "billing_admin",
  "content_editor",
  "security_admin",
  "analyst",
  "read_only_auditor",
]);

function isExpectedRole(roleCode: string): boolean {
  return platformRoleCodes.has(roleCode);
}

export const platformNav: RoleNavGroup[] = [
  {
    heading: "Operations",
    items: [
      { label: "Overview", href: "/" , icon: "home" },
      { label: "Organizations", href: "/organizations", icon: "building" },
      { label: "Leads", href: "/leads", icon: "inbox" },
      { label: "Launch readiness", href: "/launch-readiness", icon: "check" },
      { label: "Audit events", href: "/audit-events", icon: "clock" },
      { label: "Jobs", href: "/jobs", icon: "clock", roles: ["platform_owner", "platform_admin", "security_admin"] },
      { label: "Security center", href: "/security", icon: "shield", roles: ["platform_owner", "platform_admin", "security_admin"] },
      { label: "Staff", href: "/staff", icon: "users", roles: ["platform_owner", "platform_admin"] },
    ],
  },
  {
    heading: "Business",
    items: [
      { label: "Feature flags", href: "/feature-flags", icon: "flag" },
      { label: "Billing", href: "/billing", icon: "credit-card" },
      { label: "Support", href: "/support", icon: "message" },
      { label: "CMS", href: "/cms", icon: "book" },
      { label: "Marketplace", href: "/marketplace", icon: "sparkles" },
      { label: "Settings", href: "/settings", icon: "settings" },
    ],
  },
];

function navForRole(groups: RoleNavGroup[], roleCode: string): NavGroup[] {
  return groups
    .map((group) => ({
      heading: group.heading,
      items: group.items.filter(
        (item) => !item.roles || item.roles.includes(roleCode),
      ),
    }))
    .filter((group) => group.items.length > 0);
}

export function PlatformShell({ children }: { children: ReactNode }) {
  const router = useRouter();
  const pathname = usePathname();
  const [me, setMe] = useState<MeResponse | null>(null);
  const [loadError, setLoadError] = useState(false);

  const signOut = useCallback(async () => {
    try {
      await getClient().logout();
    } catch {
      // Session may already be invalid.
    }
    clearSession();
    router.replace("/login");
  }, [router]);

  const load = useCallback(async () => {
    setLoadError(false);

    try {
      const profile = await getClient().me();
      if (!isExpectedRole(profile.roleCode)) {
        await signOut();
        return;
      }
      setMe(profile);
    } catch (err) {
      if (err instanceof ApiError && err.status === 401) {
        clearSession();
        router.replace("/login");
        return;
      }
      setLoadError(true);
    }
  }, [router, signOut]);

  useEffect(() => {
    if (!getAccessToken()) {
      router.replace("/login");
      return;
    }

    void load();
  }, [router, load]);

  if (loadError) {
    return (
      <main className="flex min-h-screen items-center justify-center p-8">
        <div className="lp-card w-full max-w-sm rounded-[28px] p-6 text-center shadow-[0_28px_80px_rgba(6,22,49,0.12)]">
          <h1 className="text-lg font-semibold tracking-tight">
            We couldn&apos;t load your workspace
          </h1>
          <p className="mt-2 text-sm text-[var(--lp-ink-muted)]">
            Check your connection and try again, or sign out and back in.
          </p>
          <div className="mt-5 flex justify-center gap-3">
            <Button
              onClick={() => {
                void load();
              }}
            >
              Retry
            </Button>
            <Button
              variant="secondary"
              onClick={() => {
                void signOut();
              }}
            >
              Sign out
            </Button>
          </div>
        </div>
      </main>
    );
  }

  return (
    <PortalShell
      pathname={pathname}
      onNavigate={(href) => {
        router.push(href);
      }}
      groups={me ? navForRole(platformNav, me.roleCode) : []}
      orgLabel="Platform staff"
      userLabel={me ? `${me.user.displayName} · ${me.roleCode}` : "Loading…"}
      workspaceLabel="Platform control plane"
      onLogout={() => {
        void signOut();
      }}
      user={
        me
          ? {
              name: me.user.displayName,
              email: me.user.email,
              roleLabel: me.roleCode,
            }
          : undefined
      }
      userMenuItems={[
        { icon: "settings", label: "Settings", caption: "Profile, password & appearance", href: "/settings" },
        { icon: "check", label: "Launch readiness", caption: "Go-live checklist", href: "/launch-readiness" },
        { icon: "clock", label: "Audit events", caption: "Console activity trail", href: "/audit-events" },
        { icon: "plug", label: "Feature flags", caption: "Toggle platform features", href: "/feature-flags" },
        { icon: "message", label: "Support tickets", caption: "Customer support queue", href: "/support" },
      ]}
    >
      {children}
    </PortalShell>
  );
}
