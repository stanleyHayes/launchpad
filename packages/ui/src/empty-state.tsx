import type { ReactNode } from "react";
import { cn } from "./cn";

export type EmptyStateProps = {
  title: string;
  description?: string;
  action?: ReactNode;
  className?: string;
  dense?: boolean;
};

export function EmptyState({
  title,
  description,
  action,
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
      <div className="mx-auto mb-4 grid size-12 place-items-center rounded-full bg-[var(--lp-accent)]/10 text-[var(--lp-accent)]">
        <span aria-hidden="true" className="text-lg font-semibold">
          ·
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
