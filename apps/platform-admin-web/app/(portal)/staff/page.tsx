"use client";

import { useEffect, useState, useTransition, type SyntheticEvent } from "react";
import { useRouter } from "next/navigation";
import type { PlatformStaffMember } from "@launchpad/api-client";
import { ApiError } from "@launchpad/api-client";
import { Select, EmptyState, PageHeader, Reveal, Surface } from "@launchpad/ui";
import { getClient } from "@/lib/api";
import { clearSession, getAccessToken } from "@/lib/session";

const STAFF_ROLES = [
  "platform_owner",
  "platform_admin",
  "support_agent",
  "billing_admin",
  "content_editor",
  "security_admin",
  "analyst",
  "read_only_auditor",
] as const;

function formString(form: FormData, key: string): string {
  const value = form.get(key);
  return typeof value === "string" ? value.trim() : "";
}

export default function StaffPage() {
  const router = useRouter();
  const [pending, startTransition] = useTransition();
  const [staff, setStaff] = useState<PlatformStaffMember[]>([]);
  const [tempPassword, setTempPassword] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [message, setMessage] = useState<string | null>(null);

  function reload(isStale?: () => boolean) {
    startTransition(() => {
      void (async () => {
        try {
          const items = await getClient().listPlatformStaff();
          if (isStale?.()) return;
          setStaff(items);
        } catch (err) {
          if (isStale?.()) return;
          if (err instanceof ApiError && err.status === 401) {
            clearSession();
            router.replace("/login");
            return;
          }
          setError(err instanceof ApiError ? err.message : "Unable to load staff");
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
    // eslint-disable-next-line react-hooks/exhaustive-deps -- initial load only
  }, [router]);

  function onCreateStaff(event: SyntheticEvent<HTMLFormElement>) {
    event.preventDefault();
    setError(null);
    setMessage(null);
    setTempPassword(null);
    const formEl = event.currentTarget;
    const form = new FormData(formEl);

    startTransition(() => {
      void (async () => {
        try {
          const result = await getClient().createPlatformStaff({
            email: formString(form, "email"),
            displayName: formString(form, "displayName"),
            roleCode: formString(form, "roleCode"),
          });
          formEl.reset();
          if (result.invited) {
            setMessage(`Invite emailed to ${result.staff.email}`);
          } else {
            setMessage(`Staff account created for ${result.staff.email}`);
            setTempPassword(result.tempPassword ?? null);
          }
          reload();
        } catch (err) {
          setError(err instanceof ApiError ? err.message : "Unable to create staff account");
        }
      })();
    });
  }

  function changeRole(member: PlatformStaffMember, roleCode: string) {
    if (roleCode === member.roleCode) return;
    setError(null);
    setMessage(null);
    startTransition(() => {
      void (async () => {
        try {
          await getClient().updatePlatformStaffRole(member.id, { roleCode });
          setMessage(`${member.email} is now ${roleCode}`);
          reload();
        } catch (err) {
          setError(err instanceof ApiError ? err.message : "Unable to change role");
        }
      })();
    });
  }

  function setStatus(member: PlatformStaffMember, action: "deactivate" | "reactivate") {
    setError(null);
    setMessage(null);
    startTransition(() => {
      void (async () => {
        try {
          if (action === "deactivate") {
            await getClient().deactivatePlatformStaff(member.id);
          } else {
            await getClient().reactivatePlatformStaff(member.id);
          }
          setMessage(`${member.email} ${action === "deactivate" ? "deactivated" : "reactivated"}`);
          reload();
        } catch (err) {
          setError(err instanceof ApiError ? err.message : "Unable to update staff status");
        }
      })();
    });
  }

  return (
    <div className="space-y-8">
      <Reveal>
        <PageHeader
          eyebrow="Operations"
          title="Staff"
          description="Create platform staff accounts, assign internal roles, and deactivate departing staff."
        />
      </Reveal>

      {error ? (
        <p className="text-[var(--lp-danger)]" role="alert">
          {error}
        </p>
      ) : null}
      {message ? <p className="text-[var(--lp-success)]">{message}</p> : null}
      {tempPassword ? (
        <Surface className="border-[var(--lp-warning)]">
          <p className="text-sm font-semibold">Temporary password (shown once)</p>
          <p className="mt-1 font-mono text-sm">{tempPassword}</p>
          <p className="mt-1 text-sm text-[var(--lp-ink-muted)]">
            Share it securely; the staff member must change it after their first login.
          </p>
        </Surface>
      ) : null}

      <Reveal delay={1}>
        <Surface>
          <h2 className="text-lg font-semibold">Create staff account</h2>
          <form className="mt-4 grid gap-3 md:grid-cols-2" onSubmit={onCreateStaff}>
            <input
              className="lp-input"
              name="email"
              type="email"
              placeholder="Email"
              required
            />
            <input className="lp-input" name="displayName" placeholder="Display name" required />
            <Select className="lp-input md:col-span-2" name="roleCode" defaultValue="support_agent">
              {STAFF_ROLES.map((role) => (
                <option key={role} value={role}>
                  {role}
                </option>
              ))}
            </Select>
            <div className="md:col-span-2">
              <button
                type="submit"
                disabled={pending}
                className="rounded-[var(--lp-radius)] bg-[var(--lp-accent)] px-4 py-2.5 text-sm font-semibold text-white disabled:opacity-60"
              >
                Create account
              </button>
            </div>
          </form>
        </Surface>
      </Reveal>

      <Reveal delay={2}>
        <Surface className="overflow-hidden p-0">
          <div className="border-b border-[var(--lp-border)] px-5 py-4">
            <h2 className="text-lg font-semibold">All staff</h2>
            <p className="text-sm text-[var(--lp-ink-muted)]">
              {pending && staff.length === 0 ? "Loading…" : `${staff.length} accounts`}
            </p>
          </div>
          {staff.length === 0 ? (
            <div className="p-5">
              <EmptyState
                dense
                title="No staff accounts"
                description="Platform staff you create will appear here."
              />
            </div>
          ) : (
            <ul className="divide-y divide-[var(--lp-border)]">
              {staff.map((member) => (
                <li
                  key={member.id}
                  className="flex flex-wrap items-center justify-between gap-3 px-5 py-4"
                >
                  <div>
                    <p className="font-medium">{member.displayName || member.email}</p>
                    <p className="text-sm text-[var(--lp-ink-muted)]">
                      {member.email} · {member.status}
                    </p>
                    <time className="mt-1 block text-xs text-[var(--lp-ink-muted)]">
                      {new Date(member.createdAt).toLocaleString()}
                    </time>
                  </div>
                  <div className="flex flex-wrap items-center gap-2">
                    <Select
                      className="lp-input"
                      value={member.roleCode}
                      disabled={pending}
                      onChange={(event) => {
                        changeRole(member, event.target.value);
                      }}
                    >
                      {STAFF_ROLES.map((role) => (
                        <option key={role} value={role}>
                          {role}
                        </option>
                      ))}
                    </Select>
                    <button
                      type="button"
                      disabled={pending}
                      onClick={() => {
                        setStatus(member, member.status === "active" ? "deactivate" : "reactivate");
                      }}
                      className="rounded-[var(--lp-radius)] border border-[var(--lp-border)] px-3 py-2 text-sm font-semibold disabled:opacity-60"
                    >
                      {member.status === "active" ? "Deactivate" : "Reactivate"}
                    </button>
                  </div>
                </li>
              ))}
            </ul>
          )}
        </Surface>
      </Reveal>
    </div>
  );
}
