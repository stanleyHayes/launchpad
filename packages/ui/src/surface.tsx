import type { ReactNode } from "react";
import { cn } from "./cn";

export function Surface({
  children,
  className = "",
}: {
  children: ReactNode;
  className?: string;
}) {
  return <div className={cn("lp-card p-5 md:p-6", className)}>{children}</div>;
}
