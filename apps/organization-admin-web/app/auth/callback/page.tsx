"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { saveSession } from "@/lib/session";

export default function AuthCallbackPage() {
  const router = useRouter();
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const hash = window.location.hash.replace(/^#/, "");
    const params = new URLSearchParams(hash);
    const accessToken = params.get("accessToken");
    const refreshToken = params.get("refreshToken");

    if (!accessToken || !refreshToken) {
      setError("Missing session tokens. Please sign in again.");
      return;
    }

    saveSession(accessToken, refreshToken);
    window.history.replaceState(null, "", "/auth/callback");
    router.replace("/dashboard");
  }, [router]);

  return (
    <main className="flex min-h-screen items-center justify-center p-8">
      {error ? (
        <p className="text-[var(--lp-danger)]" role="alert">
          {error}
        </p>
      ) : (
        <p className="text-[var(--lp-ink-muted)]">Finishing sign-up…</p>
      )}
    </main>
  );
}
