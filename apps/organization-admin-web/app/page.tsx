"use client";

import { useEffect } from "react";
import { useRouter } from "next/navigation";
import { getAccessToken } from "@/lib/session";

export default function HomePage() {
  const router = useRouter();

  useEffect(() => {
    if (getAccessToken()) {
      router.replace("/dashboard");
      return;
    }
    router.replace("/login");
  }, [router]);

  return (
    <main className="flex min-h-screen items-center justify-center p-8">
      <p className="text-[var(--lp-ink-muted)]">Loading LaunchPad…</p>
    </main>
  );
}
