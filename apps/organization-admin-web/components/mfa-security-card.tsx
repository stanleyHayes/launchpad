"use client";

import { useState, useTransition, type SyntheticEvent } from "react";
import type { MFAEnrollResult } from "@launchpad/api-client";
import { ApiError } from "@launchpad/api-client";
import { Button, FormField, Icon, Surface } from "@launchpad/ui";
import { getClient } from "@/lib/api";

function formString(form: FormData, key: string): string {
  const value = form.get(key);
  return typeof value === "string" ? value.trim() : "";
}

/**
 * MFASecurityCard manages the account's TOTP second factor: an enable flow
 * (secret + one-time backup codes + confirm) and a code-gated disable flow.
 * The secret and backup codes are shown exactly once — the API stores only
 * the encrypted secret and the code hashes.
 */
export function MFASecurityCard({
  mfaEnabled,
  onChanged,
}: {
  mfaEnabled: boolean;
  onChanged: (enabled: boolean) => void;
}) {
  const [pending, startTransition] = useTransition();
  const [enrollment, setEnrollment] = useState<MFAEnrollResult | null>(null);
  const [disabling, setDisabling] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [message, setMessage] = useState<string | null>(null);

  function fail(err: unknown, fallback: string) {
    setError(err instanceof ApiError ? err.message : fallback);
  }

  function onEnroll() {
    setError(null);
    setMessage(null);

    startTransition(() => {
      void (async () => {
        try {
          setEnrollment(await getClient().mfaEnroll());
        } catch (err) {
          fail(err, "Unable to start MFA setup. Try again.");
        }
      })();
    });
  }

  function onConfirm(event: SyntheticEvent<HTMLFormElement>) {
    event.preventDefault();
    setError(null);

    const code = formString(new FormData(event.currentTarget), "code");

    startTransition(() => {
      void (async () => {
        try {
          await getClient().mfaConfirm(code);
          setEnrollment(null);
          setMessage("Two-factor authentication is now enabled.");
          onChanged(true);
        } catch (err) {
          fail(err, "Unable to verify the code. Try again.");
        }
      })();
    });
  }

  function onDisable(event: SyntheticEvent<HTMLFormElement>) {
    event.preventDefault();
    setError(null);
    setMessage(null);

    const code = formString(new FormData(event.currentTarget), "code");

    startTransition(() => {
      void (async () => {
        try {
          await getClient().mfaDisable(code);
          setDisabling(false);
          setMessage("Two-factor authentication has been disabled.");
          onChanged(false);
        } catch (err) {
          fail(err, "Unable to disable MFA. Check the code and try again.");
        }
      })();
    });
  }

  return (
    <Surface>
      <h2 className="flex items-center gap-2 text-sm font-bold">
        <Icon name="shield" className="h-4 w-4 text-[var(--lp-brand)]" />
        Two-factor authentication
      </h2>
      <p className="mt-1 text-xs text-[var(--lp-ink-muted)]">
        {mfaEnabled
          ? "Enabled — sign-in requires a code from your authenticator app."
          : "Require an authenticator-app code at sign-in. Recommended (and expected) for privileged roles."}
      </p>

      {error ? (
        <p
          className="mt-3 rounded-[var(--lp-radius)] bg-[var(--lp-danger)]/10 px-3 py-2 text-sm text-[var(--lp-danger)]"
          role="alert"
        >
          {error}
        </p>
      ) : null}
      {message ? (
        <p
          className="mt-3 rounded-[var(--lp-radius)] bg-[var(--lp-success)]/10 px-3 py-2 text-sm text-[var(--lp-success)]"
          role="status"
        >
          {message}
        </p>
      ) : null}

      {enrollment ? (
        <div className="mt-4 space-y-4">
          <div className="rounded-[var(--lp-radius-input)] bg-[var(--lp-paper)] p-3 shadow-[var(--lp-shadow-inset)]">
            <p className="text-[10px] font-semibold uppercase tracking-wider text-[var(--lp-ink-muted)]">
              Setup key
            </p>
            <p className="mt-1 break-all font-mono text-sm font-semibold">{enrollment.secret}</p>
            <p className="mt-2 text-xs text-[var(--lp-ink-muted)]">
              Add this key to your authenticator app (or scan a QR generated from the otpauth URL).
            </p>
            <p className="mt-1 break-all font-mono text-[0.65rem] text-[var(--lp-ink-muted)]">
              {enrollment.otpauthUrl}
            </p>
          </div>

          <div className="rounded-[var(--lp-radius-input)] bg-[var(--lp-paper)] p-3 shadow-[var(--lp-shadow-inset)]">
            <p className="text-[10px] font-semibold uppercase tracking-wider text-[var(--lp-ink-muted)]">
              Backup codes — shown once
            </p>
            <ul className="mt-2 grid grid-cols-2 gap-1 font-mono text-sm">
              {enrollment.backupCodes.map((code) => (
                <li key={code}>{code}</li>
              ))}
            </ul>
            <p className="mt-2 text-xs text-[var(--lp-ink-muted)]">
              Store these somewhere safe. Each works once if you lose the authenticator.
            </p>
          </div>

          <form onSubmit={onConfirm} className="space-y-3">
            <FormField
              label="Enter the 6-digit code from your app"
              name="code"
              type="text"
              required
              startIcon="lock"
              autoComplete="one-time-code"
              placeholder="123456"
            />
            <div className="flex gap-2">
              <Button type="submit" disabled={pending}>
                {pending ? "Verifying…" : "Confirm and enable"}
              </Button>
              <Button
                type="button"
                variant="ghost"
                onClick={() => {
                  setEnrollment(null);
                  setError(null);
                }}
              >
                Cancel
              </Button>
            </div>
          </form>
        </div>
      ) : mfaEnabled ? (
        <div className="mt-4">
          {disabling ? (
            <form onSubmit={onDisable} className="space-y-3">
              <FormField
                label="Confirm with an authenticator or backup code"
                name="code"
                type="text"
                required
                startIcon="lock"
                autoComplete="one-time-code"
                placeholder="123456"
              />
              <div className="flex gap-2">
                <Button type="submit" disabled={pending}>
                  {pending ? "Disabling…" : "Disable MFA"}
                </Button>
                <Button
                  type="button"
                  variant="ghost"
                  onClick={() => {
                    setDisabling(false);
                    setError(null);
                  }}
                >
                  Cancel
                </Button>
              </div>
            </form>
          ) : (
            <Button
              type="button"
              variant="secondary"
              onClick={() => {
                setDisabling(true);
                setMessage(null);
              }}
            >
              Disable MFA
            </Button>
          )}
        </div>
      ) : (
        <div className="mt-4">
          <Button type="button" onClick={onEnroll} disabled={pending}>
            {pending ? "Generating…" : "Enable MFA"}
          </Button>
        </div>
      )}
    </Surface>
  );
}
