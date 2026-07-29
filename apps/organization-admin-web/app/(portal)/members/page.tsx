"use client";

import { useEffect, useState, useTransition } from "react";
import { useRouter } from "next/navigation";
import type { MeResponse, Member, Role } from "@launchpad/api-client";
import { ApiError } from "@launchpad/api-client";
import { Select, EmptyState, PageHeader, Reveal, Surface } from "@launchpad/ui";
import { getClient } from "@/lib/api";
import { clearSession, getAccessToken } from "@/lib/session";

// Human-readable labels for built-in role codes; custom roles display their
// name as-is.
const roleLabels: Record<string, string> = {
  organization_owner: "Owner",
  hr_admin: "HR admin",
  manager: "Manager",
  employee: "Employee",
};

function roleLabel(roleCode: string): string {
  return roleLabels[roleCode] ?? roleCode;
}

export default function MembersPage() {
  const router = useRouter();
  const [pending, startTransition] = useTransition();
  const [me, setMe] = useState<MeResponse | null>(null);
  const [members, setMembers] = useState<Member[]>([]);
  const [roles, setRoles] = useState<Role[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [message, setMessage] = useState<string | null>(null);

  function reload(isStale?: () => boolean) {
    startTransition(() => {
      void (async () => {
        try {
          const client = getClient();
          const [profile, memberItems, roleItems] = await Promise.all([
            client.me(),
            client.listMembers(),
            client.listRoles(),
          ]);
          if (isStale?.()) return;
          setMe(profile);
          setMembers(memberItems);
          setRoles(roleItems);
        } catch (err) {
          if (isStale?.()) return;
          if (err instanceof ApiError && err.status === 401) {
            clearSession();
            router.replace("/login");
            return;
          }
          setError(err instanceof ApiError ? err.message : "Unable to load members");
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

  function onChangeRole(member: Member, roleCode: string) {
    if (roleCode === member.roleCode) return;
    setError(null);
    setMessage(null);
    startTransition(() => {
      void (async () => {
        try {
          await getClient().updateMemberRole(member.userId, { roleCode });
          setMessage(
            `${member.displayName ?? member.email ?? "Member"} is now ${roleLabel(roleCode)}`,
          );
          reload();
        } catch (err) {
          if (err instanceof ApiError && err.status === 401) {
            clearSession();
            router.replace("/login");
            return;
          }
          // e.g. 409 LAST_OWNER when demoting the last organization owner.
          setError(err instanceof ApiError ? err.message : "Unable to change role");
          reload();
        }
      })();
    });
  }

  return (
          <div className="space-y-8">
        <Reveal>
          <PageHeader
            eyebrow="People"
            title="Members"
            description="Everyone with access to this workspace and their role."
          />
        </Reveal>

        {error ? (
          <p className="text-[var(--lp-danger)]" role="alert">
            {error}
          </p>
        ) : null}
        {message ? <p className="text-[var(--lp-success)]">{message}</p> : null}

        <Reveal delay={1}>
          <Surface className="overflow-hidden p-0">
            <div className="border-b border-[var(--lp-border)] px-5 py-4">
              <h2 className="text-lg font-semibold">Workspace members</h2>
              <p className="text-sm text-[var(--lp-ink-muted)]">
                {pending && members.length === 0 ? "Loading…" : `${members.length} members`}
              </p>
            </div>
            {members.length === 0 ? (
              <div className="p-5">
                <EmptyState
                  dense
                  title="No members found"
                  description="Members appear here once they join the organization."
                />
              </div>
            ) : (
              <ul className="divide-y divide-[var(--lp-border)]">
                {members.map((member) => {
                  const isSelf = me?.user.id === member.userId;
                  return (
                    <li
                      key={member.membershipId}
                      className="flex flex-wrap items-center justify-between gap-3 px-5 py-4"
                    >
                      <div>
                        <p className="font-medium">
                          {member.displayName ?? member.email ?? member.userId}
                          {isSelf ? (
                            <span className="ml-2 text-xs font-semibold uppercase tracking-wide text-[var(--lp-ink-muted)]">
                              You
                            </span>
                          ) : null}
                        </p>
                        <p className="text-sm text-[var(--lp-ink-muted)]">
                          {member.email ?? "—"}
                          {member.userStatus ? ` · ${member.userStatus}` : ""}
                          {` · membership ${member.status}`}
                        </p>
                      </div>
                      <label className="flex items-center gap-2 text-sm text-[var(--lp-ink-muted)]">
                        Role
                        <Select
                          className="lp-input"
                          value={member.roleCode}
                          disabled={pending || isSelf}
                          title={isSelf ? "You cannot change your own role" : undefined}
                          onChange={(event) => {
                            onChangeRole(member, event.target.value);
                          }}
                        >
                          {roles.some((role) => role.name === member.roleCode) ? null : (
                            <option value={member.roleCode}>{roleLabel(member.roleCode)}</option>
                          )}
                          {roles.map((role) => (
                            <option key={role.id} value={role.name}>
                              {roleLabel(role.name)}
                            </option>
                          ))}
                        </Select>
                      </label>
                    </li>
                  );
                })}
              </ul>
            )}
          </Surface>
        </Reveal>
      </div>
      );
}
