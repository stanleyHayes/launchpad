"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { getClient } from "@/lib/api";
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

    // Strip tokens from the URL immediately so they cannot leak via history,
    // referrer, or screenshots while validation runs.
    window.history.replaceState(null, "", "/auth/callback");

    void (async () => {
      try {
        const client = getClient();

        // Persist nothing until the API confirms the handed-over token.
        await client.meWithToken(accessToken);

        // Exchange the refresh token for the HttpOnly cookie session.
        await client.refresh(refreshToken);

        saveSession();
        router.replace("/dashboard");
      } catch {
        setError("This sign-in link is invalid or expired. Please sign in again.");
      }
    })();
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
