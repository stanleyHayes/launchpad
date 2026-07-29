"use client";

import { useEffect, useState, useTransition, type SyntheticEvent } from "react";
import { useRouter } from "next/navigation";
import type { ChannelStatus, MeResponse, Organization } from "@launchpad/api-client";
import { ApiError } from "@launchpad/api-client";
import { Select,
  Icon,
  InitialsAvatar,
  PageHeader,
  Reveal,
  Surface,
  ThemeTiles,
} from "@launchpad/ui";
import { getClient } from "@/lib/api";
import { clearSession, getAccessToken } from "@/lib/session";
import { MFASecurityCard } from "@/components/mfa-security-card";

const tabs = [
  { key: "profile", label: "Profile", icon: "user" },
  { key: "appearance", label: "Appearance", icon: "sparkles" },
  { key: "notifications", label: "Notifications", icon: "bell" },
  { key: "security", label: "Security", icon: "shield" },
] as const;

type TabKey = (typeof tabs)[number]["key"];

const timezones = ["UTC", "Africa/Accra", "Europe/London", "Europe/Berlin", "America/New_York", "America/Chicago", "America/Los_Angeles", "Asia/Dubai", "Asia/Singapore"];

function formString(form: FormData, key: string): string {
  const value = form.get(key);
  return typeof value === "string" ? value.trim() : "";
}

export default function SettingsPage() {
  const router = useRouter();
  const [pending, startTransition] = useTransition();
  const [tab, setTab] = useState<TabKey>("profile");
  const [me, setMe] = useState<MeResponse | null>(null);
  const [org, setOrg] = useState<Organization | null>(null);
  const [channels, setChannels] = useState<ChannelStatus | null>(null);
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
          const client = getClient();
          const [profile, organization, channelStatus] = await Promise.all([
            client.me(),
            client.getCurrentOrganization(),
            client.getNotificationChannels(),
          ]);
          if (stale) return;
          setMe(profile);
          setOrg(organization);
          setChannels(channelStatus);
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

  function onSaveOrganization(event: SyntheticEvent<HTMLFormElement>) {
    event.preventDefault();
    setError(null);
    setMessage(null);

    const formEl = event.currentTarget;
    const form = new FormData(formEl);
    startTransition(() => {
      void (async () => {
        try {
          const updated = await getClient().updateCurrentOrganization({
            name: formString(form, "name") || undefined,
            timezone: formString(form, "timezone") || undefined,
            branding: {
              primaryColor: formString(form, "primaryColor") || undefined,
              accentColor: formString(form, "accentColor") || undefined,
            },
            customDomain: formString(form, "customDomain"),
          });
          setOrg(updated);
          setMessage("Organization updated");
        } catch (err) {
          setError(err instanceof ApiError ? err.message : "Unable to save organization");
        }
      })();
    });
  }

  function onSaveProfile(event: SyntheticEvent<HTMLFormElement>) {
    event.preventDefault();
    const displayName = formString(new FormData(event.currentTarget), "displayName");
    setError(null);
    setMessage(null);
    startTransition(() => {
      void getClient().updateMyProfile(displayName).then((user) => {
        setMe((current) => current ? { ...current, user } : current);
        setMessage("Personal profile updated");
      }).catch((cause: unknown) => {
        setError(cause instanceof ApiError ? cause.message : "Unable to update profile");
      });
    });
  }

  function onChangePassword(event: SyntheticEvent<HTMLFormElement>) {
    event.preventDefault();
    const form = new FormData(event.currentTarget);
    const currentPassword = formString(form, "currentPassword");
    const newPassword = formString(form, "newPassword");
    if (newPassword !== formString(form, "confirmPassword")) {
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

  function onSaveChannels(event: SyntheticEvent<HTMLFormElement>) {
    event.preventDefault();
    setError(null);
    setMessage(null);

    const formEl = event.currentTarget;
    const form = new FormData(formEl);
    startTransition(() => {
      void (async () => {
        try {
          const updated = await getClient().setNotificationChannels({
            slackWebhookUrl: formString(form, "slackWebhookUrl") || undefined,
            teamsWebhookUrl: formString(form, "teamsWebhookUrl") || undefined,
          });
          setChannels(updated);
          formEl.reset();
          setMessage("Notification channels updated");
        } catch (err) {
          setError(err instanceof ApiError ? err.message : "Unable to save channels");
        }
      })();
    });
  }

  return (
    <div className="space-y-6">
      <Reveal>
        <PageHeader
          eyebrow="Account"
          title="Settings"
          description="Your profile, workspace appearance, and notification channels."
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
          <div className="max-w-3xl space-y-5">
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
              <div className="mt-5 grid grid-cols-2 gap-3 md:grid-cols-4">
                {[
                  { label: "Organization", value: org?.name ?? "…" },
                  { label: "Slug", value: org?.slug ?? "…" },
                  { label: "Plan", value: org?.planCode ?? "…" },
                  { label: "Timezone", value: org?.timezone ?? "…" },
                ].map((stat) => (
                  <div
                    key={stat.label}
                    className="rounded-[var(--lp-radius-input)] bg-[var(--lp-paper)] p-3 text-center shadow-[var(--lp-shadow-inset)]"
                  >
                    <p className="text-[10px] font-semibold uppercase tracking-wider text-[var(--lp-ink-muted)]">
                      {stat.label}
                    </p>
                    <p className="mt-1 truncate text-sm font-semibold">{stat.value}</p>
                  </div>
                ))}
              </div>
            </Surface>

            <Surface>
              <h2 className="flex items-center gap-2 text-sm font-bold">
                <Icon name="user" className="h-4 w-4 text-[var(--lp-brand)]" />
                Personal profile
              </h2>
              <p className="mt-1 text-xs text-[var(--lp-ink-muted)]">
                This is your personal account name, separate from the organization name.
              </p>
              <form onSubmit={onSaveProfile} className="mt-4 space-y-4">
                <label className="block text-sm font-medium">
                  Display name
                  <input className="lp-input mt-1.5" name="displayName" defaultValue={me?.user.displayName} minLength={2} maxLength={100} required />
                </label>
                <label className="block text-sm font-medium">
                  Sign-in email
                  <input className="lp-input mt-1.5 opacity-70" value={me?.user.email ?? ""} disabled />
                </label>
                <button type="submit" disabled={pending} className="lp-btn lp-btn--primary">
                  {pending ? "Saving…" : "Save personal profile"}
                </button>
              </form>
            </Surface>

            <Surface>
              <h2 className="flex items-center gap-2 text-sm font-bold">
                <Icon name="building" className="h-4 w-4 text-[var(--lp-brand)]" />
                Edit organization
              </h2>
              <form onSubmit={onSaveOrganization} className="mt-4 grid gap-4 sm:grid-cols-2">
                <label className="block text-sm font-medium">
                  Organization name
                  <input className="lp-input mt-1.5" name="name" defaultValue={org?.name} required />
                </label>
                <label className="block text-sm font-medium sm:col-span-2">
                  Custom domain
                  <input
                    className="lp-input mt-1.5"
                    name="customDomain"
                    defaultValue={org?.customDomain}
                    placeholder="onboarding.example.com"
                  />
                </label>
                <label className="block text-sm font-medium">
                  Timezone
                  <Select className="lp-input mt-1.5" name="timezone" defaultValue={org?.timezone ?? "UTC"}>
                    {timezones.map((tz) => (
                      <option key={tz} value={tz}>
                        {tz}
                      </option>
                    ))}
                  </Select>
                </label>
                <label className="block text-sm font-medium">
                  Brand primary color
                  <input
                    className="lp-input mt-1.5"
                    name="primaryColor"
                    type="color"
                    defaultValue={org?.branding?.primaryColor ?? "#16386e"}
                  />
                </label>
                <label className="block text-sm font-medium">
                  Brand accent color
                  <input
                    className="lp-input mt-1.5"
                    name="accentColor"
                    type="color"
                    defaultValue={org?.branding?.accentColor ?? "#2e5bb0"}
                  />
                </label>
                <div className="flex justify-end sm:col-span-2">
                  <button type="submit" disabled={pending} className="lp-btn lp-btn--primary">
                    {pending ? "Saving…" : "Save changes"}
                  </button>
                </div>
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

      {tab === "notifications" ? (
        <Reveal delay={2}>
          <div className="max-w-3xl">
            <Surface>
              <h2 className="flex items-center gap-2 text-sm font-bold">
                <Icon name="bell" className="h-4 w-4 text-[var(--lp-brand)]" />
                Chat notifications
              </h2>
              <p className="mt-1 text-xs text-[var(--lp-ink-muted)]">
                Webhook URLs are write-only credentials — the API never returns them.
              </p>
              <form onSubmit={onSaveChannels} className="mt-4 space-y-4">
                <label className="block text-sm font-medium">
                  Slack webhook {channels?.slackConfigured ? "(configured)" : ""}
                  <input
                    className="lp-input mt-1.5"
                    name="slackWebhookUrl"
                    type="url"
                    placeholder={channels?.slackConfigured ? "Configured — enter a new URL to replace" : "https://hooks.slack.com/…"}
                  />
                </label>
                <label className="block text-sm font-medium">
                  Microsoft Teams webhook {channels?.teamsConfigured ? "(configured)" : ""}
                  <input
                    className="lp-input mt-1.5"
                    name="teamsWebhookUrl"
                    type="url"
                    placeholder={channels?.teamsConfigured ? "Configured — enter a new URL to replace" : "https://…webhook.office.com/…"}
                  />
                </label>
                <div className="flex justify-end">
                  <button type="submit" disabled={pending} className="lp-btn lp-btn--primary">
                    {pending ? "Saving…" : "Save channels"}
                  </button>
                </div>
              </form>
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
              <form onSubmit={onChangePassword} className="mt-5 space-y-4">
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
                <button type="submit" disabled={pending} className="lp-btn lp-btn--primary">
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
