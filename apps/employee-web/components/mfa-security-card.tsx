"use client";

import { useState, useTransition, type SyntheticEvent } from "react";
import type { MFAEnrollResult } from "@launchpad/api-client";
import { ApiError } from "@launchpad/api-client";
import { Button, FormField, Icon, Surface } from "@launchpad/ui";
import { getClient } from "@/lib/api";

function value(event: SyntheticEvent<HTMLFormElement>): string {
  return String(new FormData(event.currentTarget).get("code") ?? "").trim();
}

export function MFASecurityCard({
  enabled,
  onChanged,
}: {
  enabled: boolean;
  onChanged: (enabled: boolean) => void;
}) {
  const [pending, startTransition] = useTransition();
  const [enrollment, setEnrollment] = useState<MFAEnrollResult | null>(null);
  const [disabling, setDisabling] = useState(false);
  const [message, setMessage] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  function failure(cause: unknown) {
    setError(cause instanceof ApiError ? cause.message : "Unable to update MFA");
  }

  function enroll() {
    setError(null);
    startTransition(() => {
      void getClient().mfaEnroll().then(setEnrollment).catch(failure);
    });
  }

  function confirm(event: SyntheticEvent<HTMLFormElement>) {
    event.preventDefault();
    startTransition(() => {
      void getClient().mfaConfirm(value(event)).then(() => {
        setEnrollment(null);
        setMessage("Two-factor authentication is enabled");
        onChanged(true);
      }).catch(failure);
    });
  }

  function disable(event: SyntheticEvent<HTMLFormElement>) {
    event.preventDefault();
    startTransition(() => {
      void getClient().mfaDisable(value(event)).then(() => {
        setDisabling(false);
        setMessage("Two-factor authentication is disabled");
        onChanged(false);
      }).catch(failure);
    });
  }

  return (
    <Surface className="space-y-4">
      <div>
        <h2 className="flex items-center gap-2 text-lg font-semibold">
          <Icon name="shield" className="size-5 text-[var(--lp-brand)]" />
          Two-factor authentication
        </h2>
        <p className="mt-1 text-sm text-[var(--lp-ink-muted)]">
          {enabled ? "Enabled for your account." : "Add an authenticator-app code to every sign-in."}
        </p>
      </div>
      {error ? <p role="alert" className="text-sm text-[var(--lp-danger)]">{error}</p> : null}
      {message ? <p className="text-sm text-[var(--lp-success)]">{message}</p> : null}

      {enrollment ? (
        <div className="space-y-4">
          <div className="rounded-[var(--lp-radius-input)] bg-[var(--lp-paper)] p-4 shadow-[var(--lp-shadow-inset)]">
            <p className="text-xs font-bold uppercase tracking-wider text-[var(--lp-ink-muted)]">Authenticator setup key</p>
            <p className="mt-2 break-all font-mono font-semibold">{enrollment.secret}</p>
            <p className="mt-3 text-xs text-[var(--lp-ink-muted)]">Backup codes — store these now; each works once.</p>
            <div className="mt-2 grid grid-cols-2 gap-1 font-mono text-sm">
              {enrollment.backupCodes.map((code) => <span key={code}>{code}</span>)}
            </div>
          </div>
          <form onSubmit={confirm} className="space-y-3">
            <FormField label="6-digit authenticator code" name="code" required autoComplete="one-time-code" placeholder="123456" />
            <div className="flex gap-2">
              <Button disabled={pending}>Confirm and enable</Button>
              <Button type="button" variant="ghost" onClick={() => setEnrollment(null)}>Cancel</Button>
            </div>
          </form>
        </div>
      ) : enabled ? (
        disabling ? (
          <form onSubmit={disable} className="space-y-3">
            <FormField label="Authenticator or backup code" name="code" required autoComplete="one-time-code" />
            <div className="flex gap-2">
              <Button disabled={pending}>Disable MFA</Button>
              <Button type="button" variant="ghost" onClick={() => setDisabling(false)}>Cancel</Button>
            </div>
          </form>
        ) : <Button variant="secondary" onClick={() => setDisabling(true)}>Disable MFA</Button>
      ) : <Button onClick={enroll} disabled={pending}>{pending ? "Preparing…" : "Set up MFA"}</Button>}
    </Surface>
  );
}
