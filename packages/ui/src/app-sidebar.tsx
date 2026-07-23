"use client";

import type { ReactNode } from "react";
import { cn } from "./cn";

export type NavItem = {
  label: string;
  href: string;
};

export type NavGroup = {
  heading: string;
  items: NavItem[];
};

export type AppSidebarProps = {
  brand: ReactNode;
  groups: NavGroup[];
  pathname: string;
  workspaceLabel?: string;
  footer?: ReactNode;
  onNavigate?: (href: string) => void;
  className?: string;
};

function isActive(pathname: string, href: string): boolean {
  return pathname === href || pathname.startsWith(`${href}/`);
}

function Tick({ className }: { className?: string }) {
  return (
    <svg viewBox="0 0 16 12" className={className} aria-hidden="true">
      <path
        d="M1 6.5 5.2 10.5 15 1"
        fill="none"
        stroke="currentColor"
        strokeWidth={2.4}
        strokeLinecap="round"
        strokeLinejoin="round"
      />
    </svg>
  );
}

/**
 * Dark navy command-center sidebar with grouped nav and accent ticks.
 * Pattern shared with AuraEDU / Back2u / Oguaaman admin shells.
 */
export function AppSidebar({
  brand,
  groups,
  pathname,
  workspaceLabel = "Workspace",
  footer,
  onNavigate,
  className = "",
}: AppSidebarProps) {
  return (
    <aside
      className={cn(
        "lp-portal-sidebar flex h-full w-[288px] flex-col border-r border-white/10 text-white",
        className,
      )}
    >
      <div className="border-b border-white/10 px-5 py-5">{brand}</div>
      <p className="px-5 pt-5 font-mono text-[10px] font-bold uppercase tracking-[0.16em] text-[var(--lp-sidebar-muted)]">
        {workspaceLabel}
      </p>
      <nav className="flex-1 space-y-5 overflow-y-auto px-3 py-4">
        {groups.map((group) => (
          <div key={group.heading}>
            <p className="mb-2 px-2 font-mono text-[10px] font-bold uppercase tracking-[0.14em] text-white/35">
              {group.heading}
            </p>
            <ul className="space-y-1">
              {group.items.map((item) => {
                const active = isActive(pathname, item.href);
                return (
                  <li key={item.href}>
                    <a
                      href={item.href}
                      onClick={(event) => {
                        if (!onNavigate) {
                          return;
                        }
                        event.preventDefault();
                        onNavigate(item.href);
                      }}
                      className={cn(
                        "relative flex items-center justify-between rounded-[10px] px-3 py-2.5 text-sm font-medium transition",
                        active
                          ? "bg-white/10 text-white shadow-[inset_3px_0_0_var(--lp-signal)]"
                          : "text-white/70 hover:bg-white/5 hover:text-white",
                      )}
                    >
                      <span>{item.label}</span>
                      {active ? <Tick className="size-3.5 text-[var(--lp-signal)]" /> : null}
                    </a>
                  </li>
                );
              })}
            </ul>
          </div>
        ))}
      </nav>
      {footer ? <div className="border-t border-white/10 px-4 py-4">{footer}</div> : null}
    </aside>
  );
}
