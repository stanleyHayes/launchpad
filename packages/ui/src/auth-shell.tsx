import type { ReactNode } from "react";
import { cn } from "./cn";

export type AuthShellProps = {
  brandName?: string;
  eyebrow?: string;
  headline: string;
  support: string;
  children: ReactNode;
  className?: string;
};

/**
 * Split auth stage: brand story panel + centered form column.
 * Pattern from AuraEDU (auth)/layout and RentOS AuthLayout.
 */
export function AuthShell({
  brandName = "LaunchPad",
  eyebrow = "Employee onboarding",
  headline,
  support,
  children,
  className = "",
}: AuthShellProps) {
  return (
    <div className={cn("grid min-h-screen lg:grid-cols-[48%_1fr]", className)}>
      <div className="lp-auth-story relative hidden flex-col justify-between overflow-hidden p-12 text-white lg:flex">
        <span
          aria-hidden="true"
          className="absolute -right-24 -top-24 size-96 rounded-full bg-[var(--lp-accent)]/25 blur-3xl"
        />
        <span
          aria-hidden="true"
          className="absolute -bottom-32 left-8 size-80 rounded-full bg-[var(--lp-signal)]/20 blur-3xl"
        />
        <p
          className="relative z-10 text-2xl font-semibold tracking-tight"
          style={{ fontFamily: "var(--lp-font-display)" }}
        >
          {brandName}
        </p>
        <div className="relative z-10">
          <p className="font-mono text-[10px] font-bold uppercase tracking-[0.2em] text-[var(--lp-signal)]">
            {eyebrow}
          </p>
          <h2
            className="mt-5 max-w-xl text-5xl font-semibold leading-[1.05] tracking-tight"
            style={{ fontFamily: "var(--lp-font-display)" }}
          >
            {headline}
          </h2>
          <p className="mt-5 max-w-lg text-lg leading-8 text-white/72">{support}</p>
          <div className="mt-8 flex gap-3">
            <span className="h-1 w-16 rounded-full bg-[var(--lp-signal)]" />
            <span className="h-1 w-8 rounded-full bg-[var(--lp-accent)]" />
            <span className="h-1 w-5 rounded-full bg-white/40" />
          </div>
        </div>
        <p className="relative z-10 text-sm opacity-75">
          © {new Date().getFullYear()} {brandName}
        </p>
      </div>
      <div className="relative flex items-center justify-center overflow-hidden bg-[var(--lp-paper)] p-6 sm:p-10">
        <span
          aria-hidden="true"
          className="absolute -right-32 -top-32 size-80 rounded-full bg-[var(--lp-accent)]/10 blur-3xl"
        />
        <div className="relative w-full max-w-[420px]">{children}</div>
      </div>
    </div>
  );
}
