import type { ReactNode } from "react";
import { cn } from "./cn";

export function Reveal({
  children,
  delay = 0,
  className = "",
}: {
  children: ReactNode;
  delay?: 0 | 1 | 2 | 3;
  className?: string;
}) {
  return (
    <div
      className={cn(
        "lp-reveal",
        delay === 1 && "lp-reveal-delay-1",
        delay === 2 && "lp-reveal-delay-2",
        delay === 3 && "lp-reveal-delay-3",
        className,
      )}
    >
      {children}
    </div>
  );
}
