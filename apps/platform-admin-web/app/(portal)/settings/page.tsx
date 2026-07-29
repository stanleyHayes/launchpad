"use client";

import { useEffect, useState, useTransition, type SyntheticEvent } from "react";
import { useRouter } from "next/navigation";
import type { MeResponse } from "@launchpad/api-client";
import { ApiError } from "@launchpad/api-client";
import { Icon, InitialsAvatar, PageHeader, Reveal, Surface, ThemeTiles } from "@launchpad/ui";
import { getClient } from "@/lib/api";
import { clearSession, getAccessToken } from "@/lib/session";
import { MFASecurityCard } from "@/components/mfa-security-card";

const tabs = [
  { key: "profile", label: "Profile", icon: "user" },
  { key: "appearance", label: "Appearance", icon: "sparkles" },
  { key: "security", label: "Security", icon: "shield" },
] as const;

type TabKey = (typeof tabs)[number]["key"];

export default function SettingsPage() {
  const router = useRouter();
  const [pending, startTransition] = useTransition();
  const [tab, setTab] = useState<TabKey>("profile");
  const [me, setMe] = useState<MeResponse | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [message, setMessage] = useState<string | null>(null);

  useEffect(() => {
    if (!getAccessToken()) {
      router.replace("/login");
      return;
    }

    let stale = false;
    startTransition(() => {
      void (async () => {
        try {
          const profile = await getClient().me();
          if (stale) return;
          setMe(profile);
        } catch (err) {
          if (stale) return;
          if (err instanceof ApiError && err.status === 401) {
            clearSession();
            router.replace("/login");
            return;
          }
          setError(err instanceof ApiError ? err.message : "Unable to load settings");
        }
      })();
    });
    return () => {
      stale = true;
    };
  }, [router]);

  function saveProfile(event: SyntheticEvent<HTMLFormElement>) {
    event.preventDefault();
    const displayName = String(new FormData(event.currentTarget).get("displayName") ?? "").trim();
    setError(null);
    setMessage(null);
    startTransition(() => {
      void getClient().updateMyProfile(displayName).then((user) => {
        setMe((current) => current ? { ...current, user } : current);
        setMessage("Profile updated");
      }).catch((cause: unknown) => {
        setError(cause instanceof ApiError ? cause.message : "Unable to update profile");
      });
    });
  }

  function changePassword(event: SyntheticEvent<HTMLFormElement>) {
    event.preventDefault();
    const form = new FormData(event.currentTarget);
    const currentPassword = String(form.get("currentPassword") ?? "");
    const newPassword = String(form.get("newPassword") ?? "");
    if (newPassword !== String(form.get("confirmPassword") ?? "")) {
      setError("New passwords do not match");
      return;
    }
    setError(null);
    setMessage(null);
    startTransition(() => {
      void getClient().changeMyPassword(currentPassword, newPassword).then(() => {
        clearSession();
        router.replace("/login?passwordChanged=1");
      }).catch((cause: unknown) => {
        setError(cause instanceof ApiError ? cause.message : "Unable to change password");
      });
    });
  }

  return (
    <div className="space-y-6">
      <Reveal>
        <PageHeader
          eyebrow="Account"
          title="Settings"
          description="Your platform staff profile, console appearance, and account security."
        />
      </Reveal>

      <Reveal delay={1}>
        <div className="lp-card flex w-fit gap-1 p-1">
          {tabs.map((item) => (
            <button
              key={item.key}
              type="button"
              onClick={() => {
                setTab(item.key);
              }}
              className={`flex items-center gap-2 rounded-[var(--lp-radius-input)] px-4 py-2 text-sm font-medium transition ${
                tab === item.key
                  ? "bg-[var(--lp-brand)] text-white"
                  : "text-[var(--lp-ink-muted)] hover:text-[var(--lp-ink)]"
              }`}
            >
              <Icon name={item.icon} className="h-4 w-4" />
              <span className="hidden sm:inline">{item.label}</span>
            </button>
          ))}
        </div>
      </Reveal>

      {error ? (
        <p className="rounded-[var(--lp-radius)] bg-[var(--lp-danger)]/10 px-3 py-2 text-sm text-[var(--lp-danger)]" role="alert">
          {error}
        </p>
      ) : null}
      {message ? (
        <p className="rounded-[var(--lp-radius)] bg-[var(--lp-success)]/10 px-3 py-2 text-sm text-[var(--lp-success)]">
          {message}
        </p>
      ) : null}

      {tab === "profile" ? (
        <Reveal delay={2}>
          <div className="grid max-w-4xl gap-5 lg:grid-cols-[.72fr_1.28fr]">
            <Surface>
              <div className="flex items-center gap-4">
                <InitialsAvatar name={me?.user.displayName ?? "?"} size={56} />
                <div className="min-w-0">
                  <p className="truncate text-lg font-semibold">{me?.user.displayName ?? "…"}</p>
                  <p className="truncate text-sm text-[var(--lp-ink-muted)]">{me?.user.email}</p>
                  <p className="mt-1 inline-block rounded-md bg-[var(--lp-brand-soft)] px-1.5 py-0.5 text-[0.65rem] font-semibold uppercase tracking-wide text-[var(--lp-brand)]">
                    {me?.roleCode.replace(/_/g, " ")}
                  </p>
                </div>
              </div>
              <div className="mt-5 grid grid-cols-2 gap-3 md:grid-cols-3">
                {[
                  { label: "Console", value: "Platform staff" },
                  { label: "Role", value: me?.roleCode.replace(/_/g, " ") ?? "…" },
                  { label: "Status", value: me?.user.status ?? "…" },
                ].map((stat) => (
                  <div
                    key={stat.label}
                    className="rounded-[var(--lp-radius-input)] bg-[var(--lp-paper)] p-3 text-center shadow-[var(--lp-shadow-inset)]"
                  >
                    <p className="text-[10px] font-semibold uppercase tracking-wider text-[var(--lp-ink-muted)]">
                      {stat.label}
                    </p>
                    <p className="mt-1 truncate text-sm font-semibold capitalize">{stat.value}</p>
                  </div>
                ))}
              </div>
            </Surface>
            <Surface>
              <h2 className="flex items-center gap-2 text-lg font-semibold">
                <Icon name="user" className="h-5 w-5 text-[var(--lp-brand)]" />
                Personal profile
              </h2>
              <p className="mt-1 text-sm text-[var(--lp-ink-muted)]">
                Update the name shown in the platform console and audit activity.
              </p>
              <form onSubmit={saveProfile} className="mt-5 space-y-4">
                <label className="block text-sm font-semibold">
                  Display name
                  <input
                    className="lp-input mt-1.5"
                    name="displayName"
                    defaultValue={me?.user.displayName}
                    minLength={2}
                    maxLength={100}
                    required
                  />
                </label>
                <label className="block text-sm font-semibold">
                  Sign-in email
                  <input className="lp-input mt-1.5 opacity-70" value={me?.user.email ?? ""} disabled />
                  <span className="mt-1 block text-xs font-normal text-[var(--lp-ink-muted)]">
                    Sign-in email changes require another platform owner.
                  </span>
                </label>
                <button className="lp-btn lp-btn--primary" disabled={pending}>
                  {pending ? "Saving…" : "Save profile"}
                </button>
              </form>
            </Surface>
          </div>
        </Reveal>
      ) : null}

      {tab === "appearance" ? (
        <Reveal delay={2}>
          <div className="max-w-3xl">
            <Surface>
              <h2 className="flex items-center gap-2 text-sm font-bold">
                <Icon name="sparkles" className="h-4 w-4 text-[var(--lp-brand)]" />
                Design system
              </h2>
              <p className="mt-1 text-xs text-[var(--lp-ink-muted)]">
                Applies to every LaunchPad app on this device.
              </p>
              <div className="mt-4">
                <ThemeTiles />
              </div>
            </Surface>
          </div>
        </Reveal>
      ) : null}

      {tab === "security" ? (
        <Reveal delay={2}>
          <div className="grid max-w-4xl gap-5 lg:grid-cols-2">
            <MFASecurityCard
              mfaEnabled={me?.mfaEnabled ?? false}
              onChanged={(enabled) => {
                setMe((current) => (current ? { ...current, mfaEnabled: enabled } : current));
              }}
            />
            <Surface>
              <h2 className="flex items-center gap-2 text-lg font-semibold">
                <Icon name="lock" className="h-5 w-5 text-[var(--lp-brand)]" />
                Change password
              </h2>
              <p className="mt-1 text-sm text-[var(--lp-ink-muted)]">
                Changing your password signs out every active session.
              </p>
              <form onSubmit={changePassword} className="mt-5 space-y-4">
                <label className="block text-sm font-semibold">
                  Current password
                  <input className="lp-input mt-1.5" name="currentPassword" type="password" autoComplete="current-password" required />
                </label>
                <label className="block text-sm font-semibold">
                  New password
                  <input className="lp-input mt-1.5" name="newPassword" type="password" autoComplete="new-password" minLength={10} required />
                </label>
                <label className="block text-sm font-semibold">
                  Confirm new password
                  <input className="lp-input mt-1.5" name="confirmPassword" type="password" autoComplete="new-password" minLength={10} required />
                </label>
                <button className="lp-btn lp-btn--primary" disabled={pending}>
                  {pending ? "Updating…" : "Update password"}
                </button>
              </form>
            </Surface>
          </div>
        </Reveal>
      ) : null}
    </div>
  );
}
