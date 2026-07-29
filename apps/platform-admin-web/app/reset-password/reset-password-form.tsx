"use client";
import Link from "next/link";
import { useState, type FormEvent } from "react";
import { ApiError } from "@launchpad/api-client";
import { AuthShell, Button, FormField, LogoTile } from "@launchpad/ui";
import { getClient } from "@/lib/api";

export default function ResetPasswordForm({ token }: { token: string }) {
  const [pending, setPending] = useState(false); const [message, setMessage] = useState(""); const [error, setError] = useState("");
  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault(); const data = new FormData(event.currentTarget); const password = String(data.get("password") ?? "");
    if (password !== String(data.get("confirm") ?? "")) { setError("Passwords do not match."); return; }
    setPending(true); setError("");
    try { await getClient().confirmPasswordReset(token, password); setMessage("Password updated. You can now sign in."); }
    catch (err) { setError(err instanceof ApiError ? err.message : "Unable to reset the password."); } finally { setPending(false); }
  }
  return <AuthShell headline="Choose a new password." support="Completing this reset signs out every existing session.">
    <div className="lp-card rounded-[28px] p-6"><LogoTile size={36} /><h1 className="mt-4 text-2xl font-semibold">Reset password</h1>
      {!token ? <p className="mt-4 text-sm text-[var(--lp-danger)]">This reset link is incomplete.</p> :
        <form onSubmit={submit} className="mt-6 space-y-4"><FormField label="New password" name="password" type="password" required minLength={10} autoComplete="new-password" startIcon="lock" />
          <FormField label="Confirm password" name="confirm" type="password" required minLength={10} autoComplete="new-password" startIcon="lock" />
          {message ? <p className="text-sm text-[var(--lp-success)]">{message}</p> : null}{error ? <p className="text-sm text-[var(--lp-danger)]">{error}</p> : null}
          <Button className="w-full" disabled={pending}>{pending ? "Updating…" : "Update password"}</Button></form>}
      <Link href="/login" className="mt-5 block text-sm text-[var(--lp-accent)]">Back to sign in</Link></div>
  </AuthShell>;
}
