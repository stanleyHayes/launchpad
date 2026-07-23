import type { ReactNode } from "react";
import { cn } from "./cn";

export type PageHeaderProps = {
  eyebrow?: string;
  title: string;
  description?: string;
  actions?: ReactNode;
  className?: string;
};

export function PageHeader({
  eyebrow,
  title,
  description,
  actions,
  className = "",
}: PageHeaderProps) {
  return (
    <header
      className={cn(
        "flex flex-wrap items-end justify-between gap-4",
        className,
      )}
    >
      <div className="min-w-0">
        {eyebrow ? (
          <p className="text-sm font-semibold uppercase tracking-[0.2em] text-[var(--lp-accent)]">
            {eyebrow}
          </p>
        ) : null}
        <h1
          className="mt-2 text-3xl font-semibold tracking-tight text-[var(--lp-ink)] md:text-4xl"
          style={{ fontFamily: "var(--lp-font-display)" }}
        >
          {title}
        </h1>
        {description ? (
          <p className="mt-2 max-w-2xl text-[var(--lp-ink-muted)]">{description}</p>
        ) : null}
      </div>
      {actions ? <div className="flex flex-wrap items-center gap-2">{actions}</div> : null}
    </header>
  );
}
