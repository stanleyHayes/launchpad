"use client";
import Link from "next/link";
import { useState, type FormEvent } from "react";
import { ApiError } from "@launchpad/api-client";
import { AuthShell, Button, FormField, LogoTile } from "@launchpad/ui";
import { getClient } from "@/lib/api";

export default function Page() {
  const [status, setStatus] = useState<"idle" | "pending" | "sent">("idle");
  const [error, setError] = useState("");
  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault(); setStatus("pending"); setError("");
    try {
      await getClient().requestPasswordReset(String(new FormData(event.currentTarget).get("email") ?? "").trim());
      setStatus("sent");
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Unable to request a reset."); setStatus("idle");
    }
  }
  return <AuthShell headline="Reset access securely." support="We will send a single-use link that expires automatically.">
    <div className="lp-card rounded-[28px] p-6"><LogoTile size={36} /><h1 className="mt-4 text-2xl font-semibold">Forgot password</h1>
      <form onSubmit={submit} className="mt-6 space-y-4"><FormField label="Work email" name="email" type="email" required autoComplete="email" startIcon="mail" />
        {status === "sent" ? <p className="text-sm text-[var(--lp-success)]">If an account exists, a reset link is on its way.</p> : null}
        {error ? <p className="text-sm text-[var(--lp-danger)]">{error}</p> : null}
        <Button className="w-full" disabled={status === "pending"}>{status === "pending" ? "Sending…" : "Send reset link"}</Button></form>
      <Link href="/login" className="mt-5 block text-sm text-[var(--lp-accent)]">Back to sign in</Link></div>
  </AuthShell>;
}
