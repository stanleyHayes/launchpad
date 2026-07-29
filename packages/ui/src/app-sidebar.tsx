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
  collapsed?: boolean;
  onToggleCollapsed?: () => void;
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
  collapsed = false,
  onToggleCollapsed,
  className = "",
}: AppSidebarProps) {
  return (
    <aside
      className={cn(
        "lp-portal-sidebar flex h-full min-h-0 flex-col overflow-hidden border-r border-white/10 text-white transition-[width] duration-200",
        collapsed ? "w-[88px]" : "w-[288px]",
        className,
      )}
    >
      <div className={cn("relative flex items-center border-b border-white/10 py-5", collapsed ? "justify-center px-3" : "justify-between gap-3 px-5")}>
        {brand}
        {onToggleCollapsed ? (
          <button
            type="button"
            onClick={onToggleCollapsed}
            aria-label={collapsed ? "Expand sidebar" : "Collapse sidebar"}
            aria-expanded={!collapsed}
            title={collapsed ? "Expand sidebar" : "Collapse sidebar"}
            className={cn(
              "grid size-9 shrink-0 place-items-center rounded-xl border border-white/15 text-white/65 transition hover:bg-white/10 hover:text-white focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-white/70",
              collapsed && "absolute bottom-1 right-1 size-7 bg-[var(--lp-sidebar)] shadow-lg",
            )}
          >
            <Icon name={collapsed ? "chevron-right" : "chevron-left"} className="size-4" />
          </button>
        ) : null}
      </div>
      {!collapsed ? (
        <p className="px-5 pt-5 font-mono text-[10px] font-bold uppercase tracking-[0.16em] text-[var(--lp-sidebar-muted)]">
          {workspaceLabel}
        </p>
      ) : null}
      <nav className={cn("h-0 min-h-0 flex-1 touch-pan-y space-y-5 overflow-x-hidden overflow-y-auto overscroll-y-contain py-4 [-webkit-overflow-scrolling:touch] [scrollbar-gutter:stable]", collapsed ? "px-2" : "px-3")}>
        {groups.map((group) => (
          <div key={group.heading}>
            {!collapsed ? (
              <p className="mb-2 px-2 font-mono text-[10px] font-bold uppercase tracking-[0.14em] text-white/35">
                {group.heading}
              </p>
            ) : null}
            <ul className="lp-nav-branch space-y-1">
              {group.items.map((item) => {
                const active = isActive(pathname, item.href);
                const content = (
                  <>
                    {item.icon ? (
                      <span className="lp-nav-node">
                        <Icon name={item.icon} className="h-3.5 w-3.5" />
                      </span>
                    ) : null}
                    {!collapsed ? <span className="min-w-0 flex-1 truncate">{item.label}</span> : null}
                    {!collapsed && item.badge !== undefined && item.badge > 0 ? (
                      <span className="lp-nav-badge">
                        {item.badge > 99 ? "99+" : item.badge}
                      </span>
                    ) : active && !collapsed ? (
                      <span className="lp-nav-dot" aria-hidden="true" />
                    ) : null}
                  </>
                );
                const itemClassName = cn(
                  "lp-nav-item w-full text-left",
                  collapsed && "justify-center px-2",
                  active && "lp-nav-item--active",
                );
                return (
                  <li key={item.href}>
                    {onNavigate ? (
                      <button
                        type="button"
                        title={collapsed ? item.label : undefined}
                        aria-label={collapsed ? item.label : undefined}
                        aria-current={active ? "page" : undefined}
                        onClick={() => onNavigate(item.href)}
                        className={itemClassName}
                      >
                        {content}
                      </button>
                    ) : (
                      <a
                        href={item.href}
                        title={collapsed ? item.label : undefined}
                        aria-label={collapsed ? item.label : undefined}
                        aria-current={active ? "page" : undefined}
                        className={itemClassName}
                      >
                        {content}
                      </a>
                    )}
                  </li>
                );
              })}
            </ul>
          </div>
        ))}
      </nav>
      {footer && !collapsed ? <div className="border-t border-white/10 px-4 py-4">{footer}</div> : null}
    </aside>
  );
}
