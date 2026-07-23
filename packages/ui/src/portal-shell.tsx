"use client";

import type { ReactNode } from "react";
import { AppSidebar, type NavGroup } from "./app-sidebar";
import { cn } from "./cn";

export type PortalShellProps = {
  pathname: string;
  onNavigate: (href: string) => void;
  brandName?: string;
  workspaceLabel?: string;
  orgLabel?: string;
  userLabel?: string;
  groups: NavGroup[];
  onLogout?: () => void;
  children: ReactNode;
  className?: string;
};

/**
 * Shared authenticated chrome: dark sidebar + sticky top bar + scrollable workspace.
 * Mirrors AuraEDU PortalShell / Back2u AdminLayout composition.
 * Framework-agnostic — apps pass pathname + onNavigate.
 */
export function PortalShell({
  pathname,
  onNavigate,
  brandName = "LaunchPad",
  workspaceLabel = "Onboarding command centre",
  orgLabel,
  userLabel,
  groups,
  onLogout,
  children,
  className = "",
}: PortalShellProps) {
  return (
    <div
      className={cn(
        "lp-portal-frame grid h-screen grid-cols-[288px_1fr] overflow-hidden max-md:grid-cols-1",
        className,
      )}
    >
      <div className="max-md:hidden">
        <AppSidebar
          pathname={pathname}
          groups={groups}
          workspaceLabel={workspaceLabel}
          onNavigate={onNavigate}
          brand={
            <div className="flex min-w-0 items-center gap-3">
              <span className="grid size-11 shrink-0 place-items-center rounded-2xl border border-white/10 bg-white/[0.07] text-sm font-bold tracking-tight text-white">
                LP
              </span>
              <span className="min-w-0">
                <span
                  className="block truncate text-lg font-semibold tracking-tight"
                  style={{ fontFamily: "var(--lp-font-display)" }}
                >
                  {brandName}
                </span>
                <span className="mt-0.5 block truncate font-mono text-[9px] font-bold uppercase tracking-[0.16em] text-white/45">
                  {orgLabel ?? "Organization"}
                </span>
              </span>
            </div>
          }
          footer={
            onLogout ? (
              <button
                type="button"
                onClick={onLogout}
                className="w-full rounded-[10px] border border-white/15 px-3 py-2 text-left text-sm font-medium text-white/80 transition hover:bg-white/5 hover:text-white"
              >
                Sign out
              </button>
            ) : null
          }
        />
      </div>

      <div className="flex min-h-0 flex-col">
        <header className="sticky top-0 z-10 flex items-center justify-between gap-4 border-b border-[var(--lp-border)] bg-white/80 px-4 py-3 backdrop-blur md:px-8">
          <div className="min-w-0">
            <p className="truncate text-sm font-semibold text-[var(--lp-ink)]">
              {orgLabel ?? "Organization"}
            </p>
            <p className="truncate text-xs text-[var(--lp-ink-muted)]">
              {userLabel ?? workspaceLabel}
            </p>
          </div>
          <nav className="flex flex-wrap gap-2 md:hidden">
            {groups.flatMap((group) =>
              group.items.map((item) => (
                <a
                  key={item.href}
                  href={item.href}
                  onClick={(event) => {
                    event.preventDefault();
                    onNavigate(item.href);
                  }}
                  className="rounded-full border border-[var(--lp-border)] bg-white px-3 py-1 text-xs font-semibold text-[var(--lp-ink)]"
                >
                  {item.label}
                </a>
              )),
            )}
          </nav>
        </header>
        <main className="lp-portal-workspace min-h-0 flex-1 overflow-y-auto">
          <div className="mx-auto w-full max-w-[var(--lp-max)] px-4 py-6 md:px-8 md:py-8">
            {children}
          </div>
        </main>
      </div>
    </div>
  );
}
