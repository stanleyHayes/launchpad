"use client";

import { useEffect, useState } from "react";
import { apiBaseUrl } from "../env";

export function StatusCheck() {
  const [state, setState] = useState<"checking" | "operational" | "degraded">("checking");

  useEffect(() => {
    const controller = new AbortController();
    const timeout = window.setTimeout(() => controller.abort(), 5000);
    void fetch(`${apiBaseUrl}/healthz`, { signal: controller.signal, cache: "no-store" })
      .then((response) => setState(response.ok ? "operational" : "degraded"))
      .catch(() => setState("degraded"))
      .finally(() => window.clearTimeout(timeout));
    return () => {
      window.clearTimeout(timeout);
      controller.abort();
    };
  }, []);

  const label = state === "checking" ? "Checking…" : state === "operational" ? "Operational" : "Service disruption";
  const color = state === "degraded" ? "text-[var(--lp-danger)]" : "text-[var(--lp-success)]";
  return (
    <div className="lp-card mt-10 flex items-center justify-between px-6 py-5">
      <span className="font-medium">LaunchPad API</span>
      <span className={`text-sm font-semibold ${color}`} aria-live="polite">{label}</span>
    </div>
  );
}
