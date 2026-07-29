"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";

export function FooterLink({
  href,
  children,
}: {
  href: string;
  children: React.ReactNode;
}) {
  const pathname = usePathname();
  const isInternal = href.startsWith("/");
  const active =
    isInternal && (pathname === href || pathname.startsWith(`${href}/`));

  return (
    <Link
      href={href}
      aria-current={active ? "page" : undefined}
      className={`relative inline-flex items-center py-0.5 transition ${
        active
          ? "font-semibold text-[var(--lp-brand)]"
          : "text-[var(--lp-ink-muted)] hover:text-[var(--lp-brand)]"
      }`}
    >
      {active ? (
        <span
          aria-hidden="true"
          className="mr-2 h-1.5 w-1.5 rounded-full bg-[var(--lp-brand)]"
        />
      ) : null}
      {children}
    </Link>
  );
}
