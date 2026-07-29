"use client";

import { useEffect, useState, type ReactNode } from "react";
import { AppSidebar, type NavGroup } from "./app-sidebar";
import { Icon } from "./icon";
import { LogoTile } from "./logo-tile";
import { UserMenu, type UserMenuItem, type UserMenuUser } from "./user-menu";
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
  user?: UserMenuUser;
  userMenuItems?: UserMenuItem[];
  workspaceSwitcher?: ReactNode;
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
  user,
  userMenuItems,
  workspaceSwitcher,
  children,
  className = "",
}: PortalShellProps) {
  const [mobileNavigationOpen, setMobileNavigationOpen] = useState(false);

  useEffect(() => {
    if (!mobileNavigationOpen) return;

    function onKeyDown(event: KeyboardEvent) {
      if (event.key === "Escape") setMobileNavigationOpen(false);
    }

    document.body.style.overflow = "hidden";
    window.addEventListener("keydown", onKeyDown);

    return () => {
      document.body.style.overflow = "";
      window.removeEventListener("keydown", onKeyDown);
    };
  }, [mobileNavigationOpen]);

  function navigate(href: string) {
    setMobileNavigationOpen(false);
    onNavigate(href);
  }

  const brand = (
    <div className="flex min-w-0 items-center gap-3">
      <LogoTile size={44} />
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
  );

  return (
    <div
      className={cn(
        "lp-portal-frame fixed inset-0 grid h-[100dvh] min-h-0 grid-cols-[288px_minmax(0,1fr)] grid-rows-[minmax(0,1fr)] overflow-hidden max-md:grid-cols-1",
        className,
      )}
    >
      <div className="min-h-0 overflow-hidden max-md:hidden">
        <AppSidebar
          pathname={pathname}
          groups={groups}
          workspaceLabel={workspaceLabel}
          onNavigate={navigate}
          brand={brand}
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

      {mobileNavigationOpen ? (
        <div className="fixed inset-0 z-50 md:hidden" role="dialog" aria-modal="true" aria-label="Navigation">
          <button
            type="button"
            className="lp-mobile-menu-backdrop absolute inset-0 bg-[#061631]/60 backdrop-blur-sm"
            aria-label="Close navigation"
            onClick={() => setMobileNavigationOpen(false)}
          />
          <div className="lp-mobile-menu relative h-full w-[min(86vw,320px)]">
            <AppSidebar
              pathname={pathname}
              groups={groups}
              workspaceLabel={workspaceLabel}
              onNavigate={navigate}
              className="w-full"
              brand={
                <div className="flex items-center justify-between gap-3">
                  {brand}
                  <button
                    type="button"
                    aria-label="Close navigation"
                    onClick={() => setMobileNavigationOpen(false)}
                    className="grid h-10 w-10 shrink-0 place-items-center rounded-xl border border-white/15 text-white/80 transition hover:bg-white/10 hover:text-white"
                  >
                    <Icon name="close" className="h-5 w-5" />
                  </button>
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
        </div>
      ) : null}

      <div className="flex h-full min-h-0 min-w-0 flex-col overflow-hidden">
        <header className="lp-portal-header sticky top-0 z-10 flex min-h-[68px] items-center justify-between gap-3 px-4 py-3 md:px-8">
          <div className="flex min-w-0 items-center gap-3">
            <button
              type="button"
              aria-label="Open navigation"
              aria-expanded={mobileNavigationOpen}
              onClick={() => setMobileNavigationOpen(true)}
              className="grid h-11 w-11 shrink-0 place-items-center rounded-xl border border-[var(--lp-border)] bg-[var(--lp-paper-elevated)] text-[var(--lp-ink)] shadow-[var(--lp-shadow)] md:hidden"
            >
              <Icon name="menu" className="h-5 w-5" />
            </button>
            <div className="min-w-0">
            {workspaceSwitcher ?? (
              <p className="truncate text-sm font-semibold text-[var(--lp-ink)]">
                {orgLabel ?? "Organization"}
              </p>
            )}
            <p className="truncate text-xs text-[var(--lp-ink-muted)]">
              {userLabel ?? workspaceLabel}
            </p>
            </div>
          </div>
          <div className="flex shrink-0 items-center gap-3">
            {user ? (
              <UserMenu user={user} items={userMenuItems} onNavigate={navigate} onLogout={onLogout} />
            ) : null}
          </div>
        </header>
        <main className="lp-portal-workspace h-0 min-h-0 flex-1 touch-pan-y overflow-x-hidden overflow-y-auto overscroll-y-contain [-webkit-overflow-scrolling:touch]">
          <div className="mx-auto w-full max-w-[var(--lp-max)] px-4 pb-[calc(5rem+env(safe-area-inset-bottom))] pt-6 md:px-8 md:pb-8 md:pt-8">
            {children}
          </div>
        </main>
      </div>
    </div>
  );
}
