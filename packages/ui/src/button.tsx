import type { ButtonHTMLAttributes, ReactNode } from "react";

type Variant = "primary" | "secondary" | "ghost";

const styles: Record<Variant, string> = {
  primary:
    "inline-flex items-center justify-center rounded-[var(--lp-radius)] bg-[var(--lp-accent)] px-6 py-3 text-sm font-semibold text-white transition hover:bg-[var(--lp-accent-hover)] hover:-translate-y-0.5",
  secondary:
    "inline-flex items-center justify-center rounded-[var(--lp-radius)] border border-[var(--lp-border)] bg-[var(--lp-paper-elevated)] px-6 py-3 text-sm font-semibold text-[var(--lp-ink)] transition hover:-translate-y-0.5",
  ghost:
    "inline-flex items-center justify-center rounded-[var(--lp-radius)] px-4 py-2 text-sm font-semibold text-[var(--lp-ink)] transition hover:bg-black/5",
};

export function Button({
  variant = "primary",
  className = "",
  children,
  ...props
}: ButtonHTMLAttributes<HTMLButtonElement> & {
  variant?: Variant;
  children: ReactNode;
}) {
  return (
    <button className={`${styles[variant]} ${className}`} {...props}>
      {children}
    </button>
  );
}
