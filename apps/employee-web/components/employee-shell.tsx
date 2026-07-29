"use client";

import { useCallback, useEffect, useState, type ReactNode } from "react";
import { usePathname, useRouter } from "next/navigation";
import type { MeResponse, OrganizationChoice } from "@launchpad/api-client";
import { ApiError } from "@launchpad/api-client";
import { Button, PortalShell, type IconName, type NavGroup } from "@launchpad/ui";
import { getClient } from "@/lib/api";
import { clearSession, getAccessToken } from "@/lib/session";

type RoleNavItem = { label: string; href: string; icon?: IconName; roles?: string[] };
type RoleNavGroup = { heading: string; items: RoleNavItem[] };

// This portal is for any member of an organization; the API scopes data to
// the caller's own assignments regardless of role.

export const employeeNav: RoleNavGroup[] = [
  {
    heading: "Operations",
    items: [
      { label: "Home", href: "/" , icon: "home" },
      { label: "My journey", href: "/assignments", icon: "workflow" },
      { label: "Assistant", href: "/assistant", icon: "sparkles" },
      { label: "Requests", href: "/requests", icon: "check" },
      { label: "Meetings", href: "/meetings", icon: "users" },
      { label: "Support", href: "/support", icon: "message" },
      { label: "Notifications", href: "/notifications", icon: "bell" },
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

export function EmployeeShell({ children }: { children: ReactNode }) {
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
        // Platform staff and memberless accounts have no org context; explain
        // instead of bouncing to login with an API error.
        setNoOrg(true);
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
      window.location.assign("/");
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
            The employee portal requires an organization membership. Platform
            staff should use the platform admin portal instead.
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
      groups={me ? navForRole(employeeNav, me.roleCode) : []}
      orgLabel={me?.organization?.name}
      workspaceSwitcher={
        organizations.length > 1 ? (
          <label>
            <span className="sr-only">Switch organization</span>
            <select
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
            </select>
          </label>
        ) : undefined
      }
      userLabel={me ? `${me.user.displayName} · ${me.roleCode}` : "Loading…"}
      workspaceLabel="Employee workspace"
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
        { icon: "check", label: "My assignments", caption: "Your onboarding journey", href: "/" },
        { icon: "message", label: "Support", caption: "Tickets with HR and IT", href: "/support" },
        { icon: "bell", label: "Notifications", caption: "Updates and approvals", href: "/notifications" },
      ]}
    >
      {children}
    </PortalShell>
  );
}
