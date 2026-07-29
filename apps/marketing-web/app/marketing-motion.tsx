"use client";

import { usePathname } from "next/navigation";
import type { ReactNode } from "react";

export function MarketingMotion({ children }: { children: ReactNode }) {
  const pathname = usePathname();

  return (
    <div key={pathname} className="lp-marketing-route">
      {children}
    </div>
  );
}
