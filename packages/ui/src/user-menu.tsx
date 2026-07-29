"use client";

import { useEffect, useRef, useState } from "react";
import { Icon, type IconName } from "./icon";

export type UserMenuItem = {
  icon: IconName;
  label: string;
  caption?: string;
  href: string;
};

export type UserMenuUser = {
  name: string;
  email?: string;
  roleLabel?: string;
};

function initials(name: string): string {
  const parts = name.trim().split(/\s+/).filter(Boolean);
  const first = parts[0]?.[0] ?? "";
  const second = parts[1]?.[0] ?? "";
  return (first + second || name.slice(0, 2)).toUpperCase();
}

function InitialsAvatar({ name, size }: { name: string; size: number }) {
  return (
    <span
      className="grid shrink-0 place-items-center rounded-full text-white"
      style={{
        width: size,
        height: size,
        background: "var(--lp-cta-gradient)",
        fontSize: size * 0.38,
        fontWeight: 700,
      }}
      aria-hidden="true"
    >
      {initials(name)}
    </span>
  );
}

export { InitialsAvatar };

/**
 * UserMenu is the account dropdown at the top-right of the portal header
 * (RentOS/xtiitch pattern): an initials-avatar trigger with name + role, a
 * dropdown with an identity band, icon-tile destination rows, and a danger
 * sign-out. Closes on item click, outside click, and Escape.
 */
export function UserMenu({
  user,
  items = [],
  onNavigate,
  onLogout,
}: {
  user: UserMenuUser;
  items?: UserMenuItem[];
  onNavigate: (href: string) => void;
  onLogout?: () => void;
}) {
  const [open, setOpen] = useState(false);
  const rootRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!open) return undefined;

    function onMouseDown(event: MouseEvent) {
      if (rootRef.current && !rootRef.current.contains(event.target as Node)) {
        setOpen(false);
      }
    }
    function onKeyDown(event: KeyboardEvent) {
      if (event.key === "Escape") {
        setOpen(false);
      }
    }

    document.addEventListener("mousedown", onMouseDown);
    document.addEventListener("keydown", onKeyDown);
    return () => {
      document.removeEventListener("mousedown", onMouseDown);
      document.removeEventListener("keydown", onKeyDown);
    };
  }, [open]);

  function go(href: string) {
    setOpen(false);
    onNavigate(href);
  }

  return (
    <div ref={rootRef} className="relative">
      <button
        type="button"
        onClick={() => {
          setOpen((value) => !value);
        }}
        aria-expanded={open}
        aria-haspopup="menu"
        className="flex items-center gap-2.5 rounded-[var(--lp-radius-input)] px-2 py-1.5 transition hover:bg-[color-mix(in_srgb,var(--lp-ink)_5%,transparent)]"
      >
        <InitialsAvatar name={user.name} size={32} />
        <span className="hidden min-w-0 text-left md:block">
          <span className="block truncate text-sm font-semibold text-[var(--lp-ink)]">
            {user.name}
          </span>
          {user.roleLabel ? (
            <span className="block truncate text-xs capitalize text-[var(--lp-ink-muted)]">
              {user.roleLabel.replace(/_/g, " ")}
            </span>
          ) : null}
        </span>
        <Icon
          name="chevron-down"
          className={`h-4 w-4 text-[var(--lp-ink-muted)] transition-transform ${open ? "rotate-180" : ""}`}
        />
      </button>

      {open ? (
        <div
          role="menu"
          className="lp-card absolute right-0 top-full z-50 mt-2 w-72 overflow-hidden p-0"
        >
          <div className="flex items-center gap-3 border-b border-[var(--lp-border)] bg-[var(--lp-brand-soft)] px-4 py-3.5">
            <InitialsAvatar name={user.name} size={40} />
            <div className="min-w-0">
              <p className="truncate text-sm font-semibold text-[var(--lp-ink)]">{user.name}</p>
              {user.email ? (
                <p className="truncate text-xs text-[var(--lp-ink-muted)]">{user.email}</p>
              ) : null}
              {user.roleLabel ? (
                <p className="mt-0.5 inline-block rounded-md bg-[var(--lp-paper-elevated)] px-1.5 py-0.5 text-[0.65rem] font-semibold uppercase tracking-wide text-[var(--lp-brand)]">
                  {user.roleLabel.replace(/_/g, " ")}
                </p>
              ) : null}
            </div>
          </div>

          {items.length > 0 ? (
            <div className="py-1.5">
              {items.map((item) => (
                <button
                  key={item.href + item.label}
                  type="button"
                  role="menuitem"
                  onClick={() => {
                    go(item.href);
                  }}
                  className="flex w-full items-center gap-3 px-4 py-2.5 text-left transition hover:bg-[color-mix(in_srgb,var(--lp-ink)_5%,transparent)]"
                >
                  <span
                    className="grid h-8 w-8 shrink-0 place-items-center rounded-[10px] text-[var(--lp-brand)]"
                    style={{ background: "var(--lp-brand-soft)" }}
                    aria-hidden="true"
                  >
                    <Icon name={item.icon} className="h-4 w-4" />
                  </span>
                  <span className="min-w-0">
                    <span className="block truncate text-sm font-semibold text-[var(--lp-ink)]">
                      {item.label}
                    </span>
                    {item.caption ? (
                      <span className="block truncate text-xs text-[var(--lp-ink-muted)]">
                        {item.caption}
                      </span>
                    ) : null}
                  </span>
                </button>
              ))}
            </div>
          ) : null}

          {onLogout ? (
            <div className="border-t border-[var(--lp-border)] py-1.5">
              <button
                type="button"
                role="menuitem"
                onClick={() => {
                  setOpen(false);
                  onLogout();
                }}
                className="flex w-full items-center gap-3 px-4 py-2.5 text-left transition hover:bg-[var(--lp-danger)]/10"
              >
                <span
                  className="grid h-8 w-8 shrink-0 place-items-center rounded-[10px] text-[var(--lp-danger)]"
                  style={{ background: "color-mix(in srgb, var(--lp-danger) 10%, transparent)" }}
                  aria-hidden="true"
                >
                  <Icon name="log-out" className="h-4 w-4" />
                </span>
                <span className="min-w-0">
                  <span className="block truncate text-sm font-semibold text-[var(--lp-danger)]">
                    Sign out
                  </span>
                  <span className="block truncate text-xs text-[var(--lp-ink-muted)]">
                    End your session on this device
                  </span>
                </span>
              </button>
            </div>
          ) : null}
        </div>
      ) : null}
    </div>
  );
}
