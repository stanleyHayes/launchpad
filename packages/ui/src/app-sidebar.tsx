"use client";

import type { ReactNode } from "react";
import { cn } from "./cn";
import { Icon, type IconName } from "./icon";

export type NavItem = {
  label: string;
  href: string;
  icon?: IconName;
  badge?: number;
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

/**
 * Dark navy command-center sidebar (kedland pattern): icon chips in bordered
 * nodes, tree-connector lines from each group heading, and a layered active
 * state (accent left bar + signal wash + hairline).
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
      <nav className="min-h-0 flex-1 touch-pan-y space-y-5 overflow-y-scroll overscroll-contain px-3 py-4 [-webkit-overflow-scrolling:touch] [scrollbar-gutter:stable]">
        {groups.map((group) => (
          <div key={group.heading}>
            <p className="mb-2 px-2 font-mono text-[10px] font-bold uppercase tracking-[0.14em] text-white/35">
              {group.heading}
            </p>
            <ul className="lp-nav-branch space-y-1">
              {group.items.map((item) => {
                const active = isActive(pathname, item.href);
                return (
                  <li key={item.href}>
                    <a
                      href={item.href}
                      aria-current={active ? "page" : undefined}
                      onClick={(event) => {
                        if (!onNavigate) {
                          return;
                        }
                        event.preventDefault();
                        onNavigate(item.href);
                      }}
                      className={cn("lp-nav-item", active && "lp-nav-item--active")}
                    >
                      {item.icon ? (
                        <span className="lp-nav-node">
                          <Icon name={item.icon} className="h-3.5 w-3.5" />
                        </span>
                      ) : null}
                      <span className="min-w-0 flex-1 truncate">{item.label}</span>
                      {item.badge !== undefined && item.badge > 0 ? (
                        <span className="lp-nav-badge">
                          {item.badge > 99 ? "99+" : item.badge}
                        </span>
                      ) : active ? (
                        <span className="lp-nav-dot" aria-hidden="true" />
                      ) : null}
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
