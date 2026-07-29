"use client";

import { useEffect, useState, useTransition, type SyntheticEvent } from "react";
import { useRouter } from "next/navigation";
import type { Employee, MeResponse, UserPreferences } from "@launchpad/api-client";
import { ApiError } from "@launchpad/api-client";
import {
  Icon,
  InitialsAvatar,
  PageHeader,
  Reveal,
  Surface,
  ThemeTiles,
  ToggleSwitch,
  type IconName,
} from "@launchpad/ui";
import { getClient } from "@/lib/api";
import { clearSession, getAccessToken } from "@/lib/session";
import { MFASecurityCard } from "@/components/mfa-security-card";

const tabs: Array<{ key: Tab; label: string; icon: IconName }> = [
  { key: "profile", label: "Profile", icon: "user" },
  { key: "preferences", label: "Preferences", icon: "bell" },
  { key: "security", label: "Security", icon: "shield" },
  { key: "appearance", label: "Appearance", icon: "sparkles" },
];
const timezones = ["UTC", "Africa/Accra", "Europe/London", "Europe/Berlin", "America/New_York", "America/Chicago", "America/Los_Angeles", "Asia/Dubai", "Asia/Singapore"];
type Tab = "profile" | "preferences" | "security" | "appearance";

function field(form: FormData, name: string): string {
  return String(form.get(name) ?? "").trim();
}

function SettingsSkeleton() {
  return <div className="grid max-w-4xl gap-5 lg:grid-cols-[.72fr_1.28fr]">
    {[0, 1].map((item) => <Surface key={item} className="space-y-4">
      <div className="h-7 w-1/2 animate-pulse rounded-full bg-[var(--lp-border)]" />
      <div className="h-12 w-full animate-pulse rounded-[var(--lp-radius-input)] bg-[var(--lp-border)]" />
      <div className="h-12 w-full animate-pulse rounded-[var(--lp-radius-input)] bg-[var(--lp-border)]" />
    </Surface>)}
  </div>;
}

export default function SettingsPage() {
  const router = useRouter();
  const [pending, startTransition] = useTransition();
  const [loading, setLoading] = useState(true);
  const [tab, setTab] = useState<Tab>("profile");
  const [me, setMe] = useState<MeResponse | null>(null);
  const [employee, setEmployee] = useState<Employee | null>(null);
  const [preferences, setPreferences] = useState<UserPreferences | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [message, setMessage] = useState<string | null>(null);

  useEffect(() => {
    if (!getAccessToken()) {
      router.replace("/login");
      return;
    }
    void Promise.all([
      getClient().me(),
      getClient().getMyEmployeeProfile().catch((cause: unknown) => {
        if (cause instanceof ApiError && cause.status === 404) return null;
        throw cause;
      }),
    ]).then(([profile, employeeProfile]) => {
      setMe(profile);
      setEmployee(employeeProfile);
      setPreferences(profile.user.preferences);
    }).catch((cause: unknown) => {
      if (cause instanceof ApiError && cause.status === 401) {
        clearSession();
        router.replace("/login");
        return;
      }
      setError(cause instanceof ApiError ? cause.message : "Unable to load account settings");
    }).finally(() => setLoading(false));
  }, [router]);

  function saveProfile(event: SyntheticEvent<HTMLFormElement>) {
    event.preventDefault();
    const displayName = field(new FormData(event.currentTarget), "displayName");
    const mobilePhone = field(new FormData(event.currentTarget), "mobilePhone");
    setError(null);
    startTransition(() => {
      const employeeUpdate = employee
        ? getClient().updateMyEmployeeProfile(mobilePhone)
        : Promise.resolve(null);
      void Promise.all([getClient().updateMyProfile(displayName), employeeUpdate]).then(([user, employeeProfile]) => {
        setMe((current) => current ? { ...current, user } : current);
        if (employeeProfile) setEmployee(employeeProfile);
        setMessage("Profile updated");
      }).catch((cause: unknown) => setError(cause instanceof ApiError ? cause.message : "Unable to update profile"));
    });
  }

  function savePreferences(event: SyntheticEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!preferences) return;
    const form = new FormData(event.currentTarget);
    const next = {
      ...preferences,
      digestFrequency: field(form, "digestFrequency") as UserPreferences["digestFrequency"],
      locale: field(form, "locale") as UserPreferences["locale"],
      timezone: field(form, "timezone"),
    };
    startTransition(() => {
      void getClient().updateMyPreferences(next).then((saved) => {
        setPreferences(saved);
        setMessage("Preferences updated");
      }).catch((cause: unknown) => setError(cause instanceof ApiError ? cause.message : "Unable to save preferences"));
    });
  }

  function changePassword(event: SyntheticEvent<HTMLFormElement>) {
    event.preventDefault();
    const formElement = event.currentTarget;
    const form = new FormData(formElement);
    const currentPassword = field(form, "currentPassword");
    const newPassword = field(form, "newPassword");
    if (newPassword !== field(form, "confirmPassword")) {
      setError("New passwords do not match");
      return;
    }
    startTransition(() => {
      void getClient().changeMyPassword(currentPassword, newPassword).then(() => {
        formElement.reset();
        clearSession();
        router.replace("/login?passwordChanged=1");
      }).catch((cause: unknown) => setError(cause instanceof ApiError ? cause.message : "Unable to change password"));
    });
  }

  return (
    <div className="space-y-7">
      <Reveal><PageHeader eyebrow="My account" title="Settings" description="Manage your profile, sign-in security, preferences, and workspace appearance." /></Reveal>

      <Reveal delay={1}>
        <div className="lp-card flex max-w-full gap-1 overflow-x-auto p-1">
          {tabs.map((item) => <button key={item.key} type="button" onClick={() => { setTab(item.key); setMessage(null); setError(null); }} className={`flex shrink-0 items-center gap-2 rounded-[var(--lp-radius-input)] px-4 py-2.5 text-sm font-semibold transition ${tab === item.key ? "bg-[var(--lp-brand)] text-white" : "text-[var(--lp-ink-muted)] hover:text-[var(--lp-ink)]"}`}>
            <Icon name={item.icon} className="size-4" />{item.label}
          </button>)}
        </div>
      </Reveal>

      {error ? <p role="alert" className="rounded-[var(--lp-radius-input)] bg-[var(--lp-danger)]/10 px-4 py-3 text-sm text-[var(--lp-danger)]">{error}</p> : null}
      {message ? <p className="rounded-[var(--lp-radius-input)] bg-[var(--lp-success)]/10 px-4 py-3 text-sm text-[var(--lp-success)]">{message}</p> : null}
      {loading ? <SettingsSkeleton /> : null}

      {!loading && tab === "profile" && me ? <Reveal delay={2}>
        <div className="grid max-w-4xl gap-5 lg:grid-cols-[.72fr_1.28fr]">
          <Surface className="relative overflow-hidden">
            <div className="flex items-center gap-4">
              <InitialsAvatar name={me.user.displayName} size={64} />
              <div className="min-w-0">
                <h2 className="truncate text-xl font-semibold">{me.user.displayName}</h2>
                <p className="truncate text-sm text-[var(--lp-ink-muted)]">{me.user.email}</p>
              </div>
            </div>
            <dl className="mt-7 space-y-4 text-sm">
              <div><dt className="text-[var(--lp-ink-muted)]">Organization</dt><dd className="font-semibold">{me.organization?.name}</dd></div>
              <div><dt className="text-[var(--lp-ink-muted)]">Account role</dt><dd className="font-semibold">{me.roleCode.replaceAll("_", " ")}</dd></div>
              <div><dt className="text-[var(--lp-ink-muted)]">Email address</dt><dd className="break-all font-semibold">{me.user.email}</dd></div>
              <div><dt className="text-[var(--lp-ink-muted)]">Team and location</dt><dd className="font-semibold">{[employee?.team, employee?.location].filter(Boolean).join(" · ") || "Not assigned"}</dd></div>
            </dl>
          </Surface>
          <Surface>
            <h2 className="text-lg font-semibold">Personal profile</h2>
            <p className="mt-1 text-sm text-[var(--lp-ink-muted)]">This name appears to managers and teammates throughout LaunchPad.</p>
            <form onSubmit={saveProfile} className="mt-5 space-y-4">
              <label className="block text-sm font-semibold">Display name
                <input className="lp-input mt-1.5" name="displayName" defaultValue={me.user.displayName} minLength={2} maxLength={100} required />
              </label>
              <label className="block text-sm font-semibold">Email
                <input className="lp-input mt-1.5 opacity-70" value={me.user.email} disabled />
                <span className="mt-1 block text-xs font-normal text-[var(--lp-ink-muted)]">Contact your organization administrator to change your sign-in email.</span>
              </label>
              {employee ? <label className="block text-sm font-semibold">Mobile phone
                <input className="lp-input mt-1.5" name="mobilePhone" type="tel" defaultValue={employee?.mobilePhone} placeholder="+233…" />
                <span className="mt-1 block text-xs font-normal text-[var(--lp-ink-muted)]">Use international format, including the leading +.</span>
              </label> : null}
              <button className="lp-btn lp-btn--primary" disabled={pending}>{pending ? "Saving…" : "Save profile"}</button>
            </form>
          </Surface>
        </div>
      </Reveal> : null}

      {!loading && tab === "preferences" && preferences ? <Reveal delay={2}>
        <Surface className="max-w-4xl">
          <h2 className="text-lg font-semibold">Communication and regional preferences</h2>
          <form onSubmit={savePreferences} className="mt-5 grid gap-5 sm:grid-cols-2">
            <div className="flex items-center justify-between gap-4 rounded-[var(--lp-radius-input)] bg-[var(--lp-paper)] p-4 shadow-[var(--lp-shadow-inset)]">
              <div><p className="font-semibold">In-app notifications</p><p className="text-xs text-[var(--lp-ink-muted)]">Assignment, approval, and reminder alerts.</p></div>
              <ToggleSwitch label="In-app notifications" checked={preferences.inAppNotifications} onChange={(inAppNotifications) => setPreferences({ ...preferences, inAppNotifications })} />
            </div>
            <div className="flex items-center justify-between gap-4 rounded-[var(--lp-radius-input)] bg-[var(--lp-paper)] p-4 shadow-[var(--lp-shadow-inset)]">
              <div><p className="font-semibold">Email notifications</p><p className="text-xs text-[var(--lp-ink-muted)]">Receive important updates by email.</p></div>
              <ToggleSwitch label="Email notifications" checked={preferences.emailNotifications} onChange={(emailNotifications) => setPreferences({ ...preferences, emailNotifications })} />
            </div>
            <label className="text-sm font-semibold">Email digest
              <select className="lp-input mt-1.5" name="digestFrequency" defaultValue={preferences.digestFrequency}>
                <option value="instant">As updates happen</option><option value="daily">Daily digest</option><option value="weekly">Weekly digest</option><option value="off">No digest</option>
              </select>
            </label>
            <label className="text-sm font-semibold">Language
              <select className="lp-input mt-1.5" name="locale" defaultValue={preferences.locale}><option value="en">English</option><option value="fr">Français</option></select>
            </label>
            <label className="text-sm font-semibold sm:col-span-2">Timezone
              <select className="lp-input mt-1.5" name="timezone" defaultValue={preferences.timezone}>{timezones.map((timezone) => <option key={timezone}>{timezone}</option>)}</select>
            </label>
            <button className="lp-btn lp-btn--primary sm:w-fit" disabled={pending}>{pending ? "Saving…" : "Save preferences"}</button>
          </form>
        </Surface>
      </Reveal> : null}

      {!loading && tab === "security" && me ? <Reveal delay={2}>
        <div className="grid max-w-4xl gap-5 lg:grid-cols-2">
          <MFASecurityCard enabled={me.mfaEnabled} onChanged={(mfaEnabled) => setMe({ ...me, mfaEnabled })} />
          <Surface>
            <h2 className="flex items-center gap-2 text-lg font-semibold"><Icon name="lock" className="size-5 text-[var(--lp-brand)]" />Change password</h2>
            <p className="mt-1 text-sm text-[var(--lp-ink-muted)]">Changing your password signs out every active session.</p>
            <form onSubmit={changePassword} className="mt-5 space-y-4">
              <label className="block text-sm font-semibold">Current password<input className="lp-input mt-1.5" name="currentPassword" type="password" autoComplete="current-password" required /></label>
              <label className="block text-sm font-semibold">New password<input className="lp-input mt-1.5" name="newPassword" type="password" autoComplete="new-password" minLength={10} required /></label>
              <label className="block text-sm font-semibold">Confirm new password<input className="lp-input mt-1.5" name="confirmPassword" type="password" autoComplete="new-password" minLength={10} required /></label>
              <button className="lp-btn lp-btn--primary" disabled={pending}>{pending ? "Updating…" : "Update password"}</button>
            </form>
          </Surface>
        </div>
      </Reveal> : null}

      {!loading && tab === "appearance" ? <Reveal delay={2}>
        <Surface className="max-w-4xl">
          <h2 className="flex items-center gap-2 text-lg font-semibold"><Icon name="sparkles" className="size-5 text-[var(--lp-brand)]" />Appearance</h2>
          <p className="mt-1 text-sm text-[var(--lp-ink-muted)]">Your selection applies to LaunchPad on this device.</p>
          <div className="mt-5"><ThemeTiles /></div>
        </Surface>
      </Reveal> : null}
    </div>
  );
}
