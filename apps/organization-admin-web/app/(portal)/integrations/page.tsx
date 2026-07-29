"use client";

import { useEffect, useState, useTransition, type SyntheticEvent } from "react";
import { useRouter } from "next/navigation";
import type { CalendarConnection, IntegrationConnection, IntegrationProvider, SAMLConfig } from "@launchpad/api-client";
import { ApiError } from "@launchpad/api-client";
import { EmptyState, Icon, PageHeader, Reveal, Surface } from "@launchpad/ui";
import { getClient } from "@/lib/api";
import { clearSession, getAccessToken } from "@/lib/session";

const providers: { key: IntegrationProvider; name: string; description: string }[] = [
  {
    key: "github",
    name: "GitHub",
    description: "Link a GitHub account for engineering onboarding resources.",
  },
  {
    key: "jira",
    name: "Jira",
    description: "Connect your Jira site to sync onboarding tickets and tasks.",
  },
];

function formString(form: FormData, key: string): string {
  const value = form.get(key);
  return typeof value === "string" ? value.trim() : "";
}

export default function IntegrationsPage() {
  const router = useRouter();
  const [pending, startTransition] = useTransition();
  const [connections, setConnections] = useState<IntegrationConnection[]>([]);
  const [calendar, setCalendar] = useState<CalendarConnection | null>(null);
  const [microsoftCalendar, setMicrosoftCalendar] = useState<CalendarConnection | null>(null);
  const [saml, setSaml] = useState<SAMLConfig | null>(null);
  const [loaded, setLoaded] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [message, setMessage] = useState<string | null>(null);
  const [busy, setBusy] = useState<IntegrationProvider | null>(null);
  const [calendarBusy, setCalendarBusy] = useState(false);

  function reload(isStale?: () => boolean) {
    startTransition(() => {
      void (async () => {
        try {
          const client = getClient();
          const [items, calendarConnection, microsoftConnection, samlConfig] = await Promise.all([
            client.listIntegrations(),
            // 404 simply means no calendar connection yet.
            client.getCalendarConnection().catch((err: unknown) => {
              if (err instanceof ApiError && err.status === 404) return null;
              throw err;
            }),
            client.getCalendarConnection("microsoft").catch((err: unknown) => {
              if (err instanceof ApiError && err.status === 404) return null;
              throw err;
            }),
            client.getSAMLConfig(),
          ]);
          if (isStale?.()) return;
          setConnections(items);
          setCalendar(calendarConnection);
          setMicrosoftCalendar(microsoftConnection);
          setSaml(samlConfig);
          setLoaded(true);
          setError(null);
        } catch (err) {
          if (isStale?.()) return;
          if (err instanceof ApiError && err.status === 401) {
            clearSession();
            router.replace("/login");
            return;
          }
          setError(err instanceof ApiError ? err.message : "Unable to load integrations");
          setLoaded(true);
        }
      })();
    });
  }

  useEffect(() => {
    if (!getAccessToken()) {
      router.replace("/login");
      return;
    }
    let stale = false;
    reload(() => stale);
    return () => {
      stale = true;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps -- reload on route entry
  }, [router]);

  function handleActionError(err: unknown, fallback: string) {
    if (err instanceof ApiError && err.status === 401) {
      clearSession();
      router.replace("/login");
      return;
    }
    setError(err instanceof ApiError ? err.message : fallback);
  }

  function onConnect(provider: IntegrationProvider, event: SyntheticEvent<HTMLFormElement>) {
    event.preventDefault();
    setError(null);
    setMessage(null);

    const formEl = event.currentTarget;
    const form = new FormData(formEl);
    const token = formString(form, "token");
    if (!token) return;

    setBusy(provider);
    void (async () => {
      try {
        await getClient().connectIntegration(provider, {
          token,
          baseUrl: formString(form, "baseUrl") || undefined,
          email: formString(form, "email") || undefined,
        });
        formEl.reset();
        setMessage(`${provider === "github" ? "GitHub" : "Jira"} connected`);
        reload();
      } catch (err) {
        handleActionError(err, "Unable to connect — check the credential and try again");
      } finally {
        setBusy(null);
      }
    })();
  }

  function onDisconnect(connection: IntegrationConnection) {
    const name = connection.provider === "github" ? "GitHub" : "Jira";
    if (!window.confirm(`Disconnect ${name}? The stored credential will be deleted.`)) {
      return;
    }

    setError(null);
    setMessage(null);
    setBusy(connection.provider);
    void (async () => {
      try {
        await getClient().disconnectIntegration(connection.provider);
        setMessage(`${name} disconnected`);
        reload();
      } catch (err) {
        handleActionError(err, `Unable to disconnect ${name}`);
      } finally {
        setBusy(null);
      }
    })();
  }

  function onHealthCheck(connection: IntegrationConnection) {
    setError(null);
    setMessage(null);
    setBusy(connection.provider);
    void (async () => {
      try {
        await getClient().checkIntegrationHealth(connection.provider);
        setMessage("Credential re-validated — connection is healthy");
        reload();
      } catch (err) {
        // A rejected credential flips the connection to error status; refresh
        // so the card shows it, then surface the message.
        reload();
        handleActionError(err, "Health check failed");
      } finally {
        setBusy(null);
      }
    })();
  }

  function connectionFor(provider: IntegrationProvider): IntegrationConnection | undefined {
    return connections.find((connection) => connection.provider === provider);
  }

  function onConnectCalendar(provider: "google" | "microsoft") {
    setError(null);
    setMessage(null);

    setCalendarBusy(true);
    void (async () => {
      try {
        const authorizationUrl = await getClient().startCalendarOAuth(provider);
        window.location.assign(authorizationUrl);
      } catch (err) {
        handleActionError(err, "Unable to start calendar authorization");
        setCalendarBusy(false);
      }
    })();
  }

  function onDisconnectCalendar(provider: "google" | "microsoft") {
    if (!window.confirm(`Disconnect ${provider === "google" ? "Google" : "Microsoft"} Calendar? Meetings will keep working without calendar events.`)) {
      return;
    }

    setError(null);
    setMessage(null);
    setCalendarBusy(true);
    void (async () => {
      try {
        await getClient().disconnectCalendar(provider);
        setMessage(`${provider === "google" ? "Google" : "Microsoft"} Calendar disconnected`);
        reload();
      } catch (err) {
        handleActionError(err, "Unable to disconnect Google Calendar");
      } finally {
        setCalendarBusy(false);
      }
    })();
  }

  function onSaveSAML(event: SyntheticEvent<HTMLFormElement>) {
    event.preventDefault();
    const form = event.currentTarget;
    const data = new FormData(form);
    setCalendarBusy(true);
    void (async () => {
      try {
        const config = await getClient().setSAMLConfig({
          enabled: data.get("enabled") === "on",
          idpMetadataXml: formString(data, "idpMetadataXml"),
          emailAttribute: formString(data, "emailAttribute") || "email",
        });
        setSaml(config);
        setMessage("SAML configuration saved");
      } catch (err) {
        handleActionError(err, "Unable to save SAML configuration");
      } finally {
        setCalendarBusy(false);
      }
    })();
  }

  return (
    <div className="space-y-6">
      <Reveal>
        <PageHeader
          eyebrow="Account"
          title="Integrations"
          description="Connect external tools for your organization. Credentials are validated before anything is stored, and are never shown again."
        />
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

      {!loaded && pending ? (
        <Surface>
          <p className="text-sm text-[var(--lp-ink-muted)]">Loading integrations…</p>
        </Surface>
      ) : (
        <div className="grid gap-5 md:grid-cols-2">
          {providers.map((provider, index) => {
            const connection = connectionFor(provider.key);

            return (
              <Reveal key={provider.key} delay={index === 0 ? 1 : 2}>
                <Surface className="h-full">
                  <div className="flex items-start justify-between gap-3">
                    <div className="flex items-center gap-2">
                      <Icon name="plug" className="h-4 w-4 text-[var(--lp-brand)]" />
                      <h2 className="text-sm font-bold">{provider.name}</h2>
                    </div>
                    {connection ? (
                      <span
                        className={`rounded-md px-1.5 py-0.5 text-[0.65rem] font-semibold uppercase tracking-wide ${
                          connection.status === "connected"
                            ? "bg-[var(--lp-success)]/10 text-[var(--lp-success)]"
                            : "bg-[var(--lp-danger)]/10 text-[var(--lp-danger)]"
                        }`}
                      >
                        {connection.status}
                      </span>
                    ) : (
                      <span className="rounded-md bg-[var(--lp-brand-soft)] px-1.5 py-0.5 text-[0.65rem] font-semibold uppercase tracking-wide text-[var(--lp-brand)]">
                        Not connected
                      </span>
                    )}
                  </div>
                  <p className="mt-1 text-xs text-[var(--lp-ink-muted)]">{provider.description}</p>

                  {connection ? (
                    <div className="mt-4 space-y-3">
                      <dl className="space-y-1.5 text-sm">
                        <div className="flex justify-between gap-3">
                          <dt className="text-[var(--lp-ink-muted)]">Account</dt>
                          <dd className="truncate font-medium">{connection.accountHandle || "—"}</dd>
                        </div>
                        {connection.baseUrl ? (
                          <div className="flex justify-between gap-3">
                            <dt className="text-[var(--lp-ink-muted)]">Site</dt>
                            <dd className="truncate font-medium">{connection.baseUrl}</dd>
                          </div>
                        ) : null}
                        <div className="flex justify-between gap-3">
                          <dt className="text-[var(--lp-ink-muted)]">Last check</dt>
                          <dd className="font-medium">
                            {connection.lastSyncAt
                              ? new Date(connection.lastSyncAt).toLocaleString()
                              : "Never"}
                          </dd>
                        </div>
                      </dl>

                      {connection.status === "error" && connection.lastError ? (
                        <p className="rounded-[var(--lp-radius)] bg-[var(--lp-danger)]/10 px-3 py-2 text-sm text-[var(--lp-danger)]">
                          {connection.lastError}
                        </p>
                      ) : null}

                      <div className="flex flex-wrap justify-end gap-2">
                        <button
                          type="button"
                          className="lp-btn lp-btn--secondary"
                          disabled={busy !== null}
                          onClick={() => {
                            onHealthCheck(connection);
                          }}
                        >
                          {busy === provider.key ? "Checking…" : "Re-check"}
                        </button>
                        <button
                          type="button"
                          className="lp-btn lp-btn--ghost"
                          disabled={busy !== null}
                          onClick={() => {
                            onDisconnect(connection);
                          }}
                        >
                          Disconnect
                        </button>
                      </div>
                    </div>
                  ) : (
                    <form
                      onSubmit={(event) => {
                        onConnect(provider.key, event);
                      }}
                      className="mt-4 space-y-3"
                    >
                      {provider.key === "jira" ? (
                        <>
                          <label className="block text-sm font-medium">
                            Jira site URL
                            <input
                              className="lp-input mt-1.5"
                              name="baseUrl"
                              type="url"
                              placeholder="https://your-org.atlassian.net"
                              required
                            />
                          </label>
                          <label className="block text-sm font-medium">
                            Account email
                            <input
                              className="lp-input mt-1.5"
                              name="email"
                              type="email"
                              placeholder="you@company.com"
                              required
                            />
                          </label>
                        </>
                      ) : null}
                      <label className="block text-sm font-medium">
                        {provider.key === "github" ? "Personal access token" : "API token"}
                        <input
                          className="lp-input mt-1.5"
                          name="token"
                          type="password"
                          autoComplete="off"
                          required
                        />
                      </label>
                      <p className="text-xs text-[var(--lp-ink-muted)]">
                        The token is validated with {provider.name}, then stored
                        encrypted — it is never shown or returned by the API.
                      </p>
                      <div className="flex justify-end">
                        <button
                          type="submit"
                          className="lp-btn lp-btn--primary"
                          disabled={busy !== null}
                        >
                          {busy === provider.key ? "Connecting…" : `Connect ${provider.name}`}
                        </button>
                      </div>
                    </form>
                  )}
                </Surface>
              </Reveal>
            );
          })}
        </div>
      )}

      {loaded && connections.length === 0 ? (
        <Reveal delay={3}>
          <Surface>
            <EmptyState
              title="No integrations connected yet"
              description="Connect GitHub or Jira above to link your organization's tools. Tokens are validated first and stored encrypted."
            />
          </Surface>
        </Reveal>
      ) : null}

      {loaded ? (
        <Reveal delay={3}>
          <div className="grid gap-4 lg:grid-cols-2">
          {([
            ["google", "Google Calendar", calendar],
            ["microsoft", "Microsoft Outlook", microsoftCalendar],
          ] as const).map(([provider, label, connection]) => (
          <Surface key={provider}>
            <div className="flex items-start justify-between gap-3">
              <div className="flex items-center gap-2">
                <Icon name="plug" className="h-4 w-4 text-[var(--lp-brand)]" />
                <h2 className="text-sm font-bold">{label}</h2>
              </div>
              {connection ? (
                <span className="rounded-md bg-[var(--lp-success)]/10 px-1.5 py-0.5 text-[0.65rem] font-semibold uppercase tracking-wide text-[var(--lp-success)]">
                  Connected
                </span>
              ) : (
                <span className="rounded-md bg-[var(--lp-brand-soft)] px-1.5 py-0.5 text-[0.65rem] font-semibold uppercase tracking-wide text-[var(--lp-brand)]">
                  Not connected
                </span>
              )}
            </div>
            <p className="mt-1 text-xs text-[var(--lp-ink-muted)]">
              Create calendar events for scheduled meetings. Meetings work without this — the
              location stays free text.
            </p>

            {connection ? (
              <div className="mt-4 space-y-3">
                <dl className="space-y-1.5 text-sm">
                  <div className="flex justify-between gap-3">
                    <dt className="text-[var(--lp-ink-muted)]">Calendar</dt>
                    <dd className="truncate font-medium">{connection.accountHandle || "—"}</dd>
                  </div>
                  <div className="flex justify-between gap-3">
                    <dt className="text-[var(--lp-ink-muted)]">Connected</dt>
                    <dd className="font-medium">{new Date(connection.createdAt).toLocaleString()}</dd>
                  </div>
                </dl>
                <div className="flex justify-end">
                  <button
                    type="button"
                    className="lp-btn lp-btn--ghost"
                    disabled={calendarBusy}
                    onClick={() => onDisconnectCalendar(provider)}
                  >
                    Disconnect
                  </button>
                </div>
              </div>
            ) : (
              <div className="mt-4 space-y-3">
                <p className="text-xs text-[var(--lp-ink-muted)]">
                  Authorize LaunchPad using the provider&apos;s secure consent screen. Access and
                  refresh tokens are stored encrypted and never returned by the API.
                </p>
                <div className="flex justify-end">
                  <button type="button" onClick={() => onConnectCalendar(provider)} className="lp-btn lp-btn--primary" disabled={calendarBusy}>
                    {calendarBusy ? "Connecting…" : `Connect ${label}`}
                  </button>
                </div>
              </div>
            )}
          </Surface>
          ))}
          </div>
        </Reveal>
      ) : null}

      {loaded && saml ? (
        <Reveal delay={3}>
          <Surface>
            <div className="flex items-start justify-between gap-3">
              <div>
                <h2 className="text-sm font-bold">SAML enterprise SSO</h2>
                <p className="mt-1 text-xs text-[var(--lp-ink-muted)]">
                  Configure signed SAML 2.0 assertions from your identity provider.
                </p>
              </div>
              <span className={`rounded-md px-1.5 py-0.5 text-[0.65rem] font-semibold uppercase tracking-wide ${saml.enabled ? "bg-[var(--lp-success)]/10 text-[var(--lp-success)]" : "bg-[var(--lp-brand-soft)] text-[var(--lp-brand)]"}`}>
                {saml.enabled ? "Enabled" : "Disabled"}
              </span>
            </div>
            <dl className="mt-4 grid gap-2 text-xs sm:grid-cols-2">
              <div><dt className="text-[var(--lp-ink-muted)]">Entity ID / metadata URL</dt><dd className="break-all font-medium">{saml.entityId}</dd></div>
              <div><dt className="text-[var(--lp-ink-muted)]">Assertion consumer URL</dt><dd className="break-all font-medium">{saml.acsUrl}</dd></div>
            </dl>
            <form onSubmit={onSaveSAML} className="mt-4 space-y-3">
              <label className="flex items-center gap-2 text-sm font-medium">
                <input name="enabled" type="checkbox" defaultChecked={saml.enabled} />
                Enable SAML sign-in
              </label>
              <label className="block text-sm font-medium">
                Identity provider metadata XML
                <textarea className="lp-input mt-1.5 min-h-36 font-mono text-xs" name="idpMetadataXml" required={!saml.configured} placeholder={saml.configured ? "Metadata is configured. Paste XML here only to replace it." : "<EntityDescriptor …>"} />
              </label>
              <label className="block text-sm font-medium">
                Email attribute
                <input className="lp-input mt-1.5" name="emailAttribute" defaultValue={saml.emailAttribute || "email"} />
              </label>
              <div className="flex justify-end">
                <button type="submit" className="lp-btn lp-btn--primary" disabled={calendarBusy}>Save SAML</button>
              </div>
            </form>
          </Surface>
        </Reveal>
      ) : null}
    </div>
  );
}
