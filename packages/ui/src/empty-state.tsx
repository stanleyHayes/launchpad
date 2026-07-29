import type { ReactNode } from "react";
import { cn } from "./cn";
import { Icon, type IconName } from "./icon";

export type EmptyStateProps = {
  title: string;
  description?: string;
  action?: ReactNode;
  icon?: IconName;
  className?: string;
  dense?: boolean;
};

export function EmptyState({
  title,
  description,
  action,
  icon = "inbox",
  className = "",
  dense = false,
}: EmptyStateProps) {
  return (
    <div
      className={cn(
        "lp-card text-center",
        dense ? "px-5 py-8" : "px-6 py-14",
        className,
      )}
    >
      <div className="lp-empty-state-icon mx-auto mb-5 grid size-16 place-items-center rounded-[22px] bg-[var(--lp-accent)]/10 text-[var(--lp-accent)]">
        <span className="lp-empty-state-icon__halo" aria-hidden="true" />
        <span className="relative grid size-10 place-items-center rounded-2xl bg-[var(--lp-paper-elevated)] shadow-[var(--lp-shadow)]">
          <Icon name={icon} className="size-5" />
        </span>
      </div>
      <h3
        className="text-xl font-semibold text-[var(--lp-ink)]"
        style={{ fontFamily: "var(--lp-font-display)" }}
      >
        {title}
      </h3>
      {description ? (
        <p className="mx-auto mt-2 max-w-md text-sm text-[var(--lp-ink-muted)]">{description}</p>
      ) : null}
      {action ? <div className="mt-5 flex justify-center gap-2">{action}</div> : null}
    </div>
  );
}
