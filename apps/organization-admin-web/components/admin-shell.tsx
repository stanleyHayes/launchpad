"use client";

import { useCallback, useEffect, useState, type ReactNode } from "react";
import { usePathname, useRouter } from "next/navigation";
import type { MeResponse, OrganizationChoice } from "@launchpad/api-client";
import { ApiError } from "@launchpad/api-client";
import { Select, Button, PortalShell, type IconName, type NavGroup } from "@launchpad/ui";
import { getClient } from "@/lib/api";
import { clearSession, getAccessToken } from "@/lib/session";

type RoleNavItem = { label: string; href: string; icon?: IconName; roles?: string[] };
type RoleNavGroup = { heading: string; items: RoleNavItem[] };

// Org-admin access: built-in management roles, or any custom role with real
// management permissions (employees.read is the baseline every such role holds).
const adminRoles = ["organization_owner", "hr_admin", "manager"];

function hasAdminAccess(profile: { roleCode: string; permissions: string[] }): boolean {
  return adminRoles.includes(profile.roleCode) || profile.permissions.includes("employees.read");
}

export const orgAdminNav: RoleNavGroup[] = [
  {
    heading: "Operations",
    items: [
      { label: "Overview", href: "/dashboard", icon: "home" },
      { label: "Setup", href: "/setup", icon: "sparkles" },
      { label: "Analytics", href: "/analytics", icon: "chart" },
      { label: "Journeys", href: "/journeys", icon: "workflow" },
      { label: "Assignments", href: "/assignments", icon: "workflow" },
      { label: "Assignment rules", href: "/assignment-rules", icon: "workflow" },
      { label: "Knowledge", href: "/knowledge", icon: "book" },
      { label: "Marketplace", href: "/marketplace", icon: "sparkles" },
      { label: "Assessments", href: "/assessments", icon: "check" },
      { label: "Approvals", href: "/approvals", icon: "check" },
      { label: "Requests", href: "/requests", icon: "check" },
      { label: "Meetings", href: "/meetings", icon: "users" },
      { label: "My team", href: "/manager", icon: "users" },
    ],
  },
  {
    heading: "People",
    items: [
      { label: "Employees", href: "/employees", icon: "users" },
      { label: "Members", href: "/members", icon: "user" },
      { label: "Roles", href: "/roles", icon: "shield" },
    ],
  },
  {
    heading: "Account",
    items: [
      { label: "Billing", href: "/billing", icon: "credit-card", roles: ["organization_owner"] },
      { label: "Integrations", href: "/integrations", icon: "plug" },
      { label: "Support", href: "/support", icon: "message" },
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

export function AdminShell({ children }: { children: ReactNode }) {
  const router = useRouter();
  const pathname = usePathname();
  const [me, setMe] = useState<MeResponse | null>(null);
  const [loadError, setLoadError] = useState(false);
  const [noOrg, setNoOrg] = useState(false);
  const [organizations, setOrganizations] = useState<OrganizationChoice[]>([]);
  const [switching, setSwitching] = useState(false);

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
      const [profile, choices] = await Promise.all([
        getClient().me(),
        getClient().listMyOrganizations(),
      ]);
      if (!profile.organization) {
        // Platform staff and memberless accounts have no org context; show a
        // clear explanation instead of bouncing to login with an API error.
        setNoOrg(true);
        return;
      }
      if (!hasAdminAccess(profile)) {
        await signOut();
        return;
      }
      setMe(profile);
      setOrganizations(choices);
    } catch (err) {
      if (err instanceof ApiError && err.status === 401) {
        clearSession();
        router.replace("/login");
        return;
      }
      setLoadError(true);
    }
  }, [router, signOut]);

  const switchOrganization = useCallback(async (organizationId: string) => {
    if (!organizationId || organizationId === me?.organization?.id) return;
    setSwitching(true);
    try {
      await getClient().switchOrganization(organizationId);
      window.location.assign("/dashboard");
    } catch {
      setSwitching(false);
      setLoadError(true);
    }
  }, [me?.organization?.id]);

  useEffect(() => {
    if (!getAccessToken()) {
      router.replace("/login");
      return;
    }

    void load();
  }, [router, load]);

  if (noOrg) {
    return (
      <main className="flex min-h-screen items-center justify-center p-8">
        <div className="lp-card w-full max-w-sm rounded-[28px] p-6 text-center shadow-[0_28px_80px_rgba(6,22,49,0.12)]">
          <h1 className="text-lg font-semibold tracking-tight">
            This account isn&apos;t part of an organization
          </h1>
          <p className="mt-2 text-sm text-[var(--lp-ink-muted)]">
            The organization admin portal requires an organization membership.
            Platform staff should use the platform admin portal instead.
          </p>
          <div className="mt-5 flex justify-center gap-3">
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
      groups={me ? navForRole(orgAdminNav, me.roleCode) : []}
      orgLabel={me?.organization?.name}
      workspaceSwitcher={
        organizations.length > 1 ? (
          <label>
            <span className="sr-only">Switch organization</span>
            <Select
              aria-label="Switch organization"
              disabled={switching}
              value={me?.organization?.id ?? ""}
              onChange={(event) => {
                void switchOrganization(event.target.value);
              }}
              className="block max-w-[18rem] rounded-[var(--lp-radius-input)] border border-[var(--lp-border)] bg-[var(--lp-surface)] px-2.5 py-1.5 text-sm font-semibold text-[var(--lp-ink)]"
            >
              {organizations.map((choice) => (
                <option key={choice.organization.id} value={choice.organization.id}>
                  {choice.organization.name}
                </option>
              ))}
            </Select>
          </label>
        ) : undefined
      }
      userLabel={me ? `${me.user.displayName} · ${me.roleCode}` : "Loading…"}
      workspaceLabel="Onboarding command centre"
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
        { icon: "settings", label: "Settings", caption: "Profile, password & workspace", href: "/settings" },
        { icon: "plug", label: "Integrations", caption: "GitHub and Jira connections", href: "/integrations" },
        { icon: "users", label: "Members & roles", caption: "Team access and permissions", href: "/members" },
        { icon: "shield", label: "Custom roles", caption: "Permission sets for your team", href: "/roles" },
        { icon: "chart", label: "Billing", caption: "Plan and subscription", href: "/billing" },
        { icon: "message", label: "Support", caption: "Get help from our team", href: "/support" },
      ]}
    >
      {me?.impersonation ? (
        <div
          role="alert"
          className="mb-6 rounded-[var(--lp-radius)] border border-[var(--lp-warning)] bg-[color-mix(in_srgb,var(--lp-warning)_10%,transparent)] px-4 py-3 text-sm font-medium text-[var(--lp-warning)]"
        >
          Platform support session active — a LaunchPad support agent is viewing
          this workspace with read-only access. Every action is audited and the
          session ends automatically.
        </div>
      ) : null}
      {children}
    </PortalShell>
  );
}
