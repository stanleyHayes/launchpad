"use client";

import { useEffect, useMemo, useState, useTransition } from "react";
import { useRouter } from "next/navigation";
import type { Role } from "@launchpad/api-client";
import { ApiError, LP_PERMISSIONS } from "@launchpad/api-client";
import { EmptyState, PageHeader, Reveal, Surface } from "@launchpad/ui";
import { getClient } from "@/lib/api";
import { clearSession, getAccessToken } from "@/lib/session";

const roleLabels: Record<string, string> = {
  organization_owner: "Owner",
  hr_admin: "HR admin",
  manager: "Manager",
  employee: "Employee",
};

function roleLabel(name: string): string {
  return roleLabels[name] ?? name;
}

// Custom roles are an enterprise-plan capability (mirrors the API plan gate).
const customRolesPlan = "enterprise";

// Group the permission registry by resource for the checkbox grid.
function groupPermissions(): { resource: string; permissions: string[] }[] {
  const groups = new Map<string, string[]>();
  for (const permission of LP_PERMISSIONS) {
    const resource = permission.split(".")[0] ?? permission;
    const items = groups.get(resource) ?? [];
    items.push(permission);
    groups.set(resource, items);
  }
  return [...groups.entries()].map(([resource, permissions]) => ({ resource, permissions }));
}

const permissionGroups = groupPermissions();

export default function RolesPage() {
  const router = useRouter();
  const [pending, startTransition] = useTransition();
  const [roles, setRoles] = useState<Role[]>([]);
  const [planCode, setPlanCode] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [message, setMessage] = useState<string | null>(null);

  // Form state: editingId null means "create".
  const [formOpen, setFormOpen] = useState(false);
  const [editingId, setEditingId] = useState<string | null>(null);
  const [formName, setFormName] = useState("");
  const [formPermissions, setFormPermissions] = useState<string[]>([]);

  const builtinRoles = useMemo(() => roles.filter((role) => role.builtin), [roles]);
  const customRoles = useMemo(() => roles.filter((role) => !role.builtin), [roles]);
  const canCreateCustom = planCode === customRolesPlan;

  function reload(isStale?: () => boolean) {
    startTransition(() => {
      void (async () => {
        try {
          const client = getClient();
          const [profile, roleItems] = await Promise.all([
            client.me(),
            client.listRoles(),
          ]);
          if (isStale?.()) return;
          setPlanCode(profile.organization?.planCode ?? null);
          setRoles(roleItems);
        } catch (err) {
          if (isStale?.()) return;
          if (err instanceof ApiError && err.status === 401) {
            clearSession();
            router.replace("/login");
            return;
          }
          setError(err instanceof ApiError ? err.message : "Unable to load roles");
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

  function openCreate() {
    setEditingId(null);
    setFormName("");
    setFormPermissions([]);
    setFormOpen(true);
    setError(null);
    setMessage(null);
  }

  function openEdit(role: Role) {
    setEditingId(role.id);
    setFormName(role.name);
    setFormPermissions(role.permissions);
    setFormOpen(true);
    setError(null);
    setMessage(null);
  }

  function closeForm() {
    setFormOpen(false);
    setEditingId(null);
  }

  function togglePermission(permission: string) {
    setFormPermissions((current) =>
      current.includes(permission)
        ? current.filter((item) => item !== permission)
        : [...current, permission],
    );
  }

  function onSubmit() {
    setError(null);
    setMessage(null);
    const name = formName.trim();
    if (!editingId && !/^[a-z0-9]+(?:[-_][a-z0-9]+)*$/.test(name)) {
      setError("Role name must be code-like, e.g. team_lead");
      return;
    }
    if (formPermissions.length === 0) {
      setError("Select at least one permission");
      return;
    }
    startTransition(() => {
      void (async () => {
        try {
          const client = getClient();
          if (editingId) {
            await client.updateRole(editingId, { permissions: formPermissions });
            setMessage(`Role ${roleLabel(name)} updated`);
          } else {
            await client.createRole({ name, permissions: formPermissions });
            setMessage(`Role ${name} created`);
          }
          closeForm();
          reload();
        } catch (err) {
          if (err instanceof ApiError && err.status === 401) {
            clearSession();
            router.replace("/login");
            return;
          }
          // e.g. 409 ROLE_NAME_TAKEN, 403 PLAN_NOT_ELIGIBLE.
          setError(err instanceof ApiError ? err.message : "Unable to save role");
        }
      })();
    });
  }

  function onDelete(role: Role) {
    if (!window.confirm(`Delete the custom role "${role.name}"? Members on it lose its permissions.`)) {
      return;
    }
    setError(null);
    setMessage(null);
    startTransition(() => {
      void (async () => {
        try {
          await getClient().deleteRole(role.id);
          setMessage(`Role ${role.name} deleted`);
          if (editingId === role.id) closeForm();
          reload();
        } catch (err) {
          if (err instanceof ApiError && err.status === 401) {
            clearSession();
            router.replace("/login");
            return;
          }
          setError(err instanceof ApiError ? err.message : "Unable to delete role");
        }
      })();
    });
  }

  return (
          <div className="space-y-8">
        <Reveal>
          <PageHeader
            eyebrow="People"
            title="Roles"
            description="Built-in roles and the custom permission sets for your organization."
          />
        </Reveal>

        {error ? (
          <p className="text-[var(--lp-danger)]" role="alert">
            {error}
          </p>
        ) : null}
        {message ? <p className="text-[var(--lp-success)]">{message}</p> : null}

        <Reveal delay={1}>
          <section>
            <div className="mb-4">
              <h2 className="text-lg font-semibold">Built-in roles</h2>
              <p className="text-sm text-[var(--lp-ink-muted)]">
                Fixed permission sets managed by LaunchPad.
              </p>
            </div>
            <ul className="grid gap-5 md:grid-cols-2">
              {builtinRoles.map((role) => (
                <li key={role.id}>
                  <Surface>
                    <div className="flex items-center justify-between gap-2">
                      <p className="font-medium">{roleLabel(role.name)}</p>
                      <span className="text-xs font-semibold uppercase tracking-wide text-[var(--lp-ink-muted)]">
                        Built-in
                      </span>
                    </div>
                    <ul className="mt-3 flex flex-wrap gap-1.5">
                      {role.permissions.map((permission) => (
                        <li
                          key={permission}
                          className="rounded-full border border-[var(--lp-border)] px-2.5 py-1 text-xs text-[var(--lp-ink-muted)]"
                        >
                          {permission}
                        </li>
                      ))}
                    </ul>
                  </Surface>
                </li>
              ))}
            </ul>
          </section>
        </Reveal>

        <Reveal delay={2}>
          <Surface className="overflow-hidden p-0">
            <div className="flex flex-wrap items-center justify-between gap-3 border-b border-[var(--lp-border)] px-5 py-4">
              <div>
                <h2 className="text-lg font-semibold">Custom roles</h2>
                <p className="text-sm text-[var(--lp-ink-muted)]">
                  {pending && roles.length === 0 ? "Loading…" : `${customRoles.length} custom roles`}
                </p>
              </div>
              {canCreateCustom && !formOpen ? (
                <button
                  type="button"
                  onClick={openCreate}
                  className="rounded-[var(--lp-radius)] bg-[var(--lp-accent)] px-4 py-2.5 text-sm font-semibold text-white disabled:opacity-60"
                >
                  New role
                </button>
              ) : null}
            </div>

            {!canCreateCustom ? (
              <div className="p-5">
                <EmptyState
                  dense
                  title="Custom roles are an Enterprise feature"
                  description={`Your organization is on the ${planCode ?? "current"} plan. Upgrade to Enterprise to define custom permission sets.`}
                />
              </div>
            ) : customRoles.length === 0 && !formOpen ? (
              <div className="p-5">
                <EmptyState
                  dense
                  title="No custom roles yet"
                  description="Create a role to give teammates a tailored permission set."
                />
              </div>
            ) : (
              <ul className="divide-y divide-[var(--lp-border)]">
                {customRoles.map((role) => (
                  <li
                    key={role.id}
                    className="flex flex-wrap items-center justify-between gap-3 px-5 py-4"
                  >
                    <div>
                      <p className="font-medium">{role.name}</p>
                      <p className="text-sm text-[var(--lp-ink-muted)]">
                        {role.permissions.length} permissions
                      </p>
                    </div>
                    <div className="flex gap-2">
                      <button
                        type="button"
                        disabled={pending}
                        onClick={() => {
                          openEdit(role);
                        }}
                        className="rounded-[var(--lp-radius)] border border-[var(--lp-border)] px-3 py-2 text-sm font-semibold disabled:opacity-60"
                      >
                        Edit
                      </button>
                      <button
                        type="button"
                        disabled={pending}
                        onClick={() => {
                          onDelete(role);
                        }}
                        className="rounded-[var(--lp-radius)] border border-[var(--lp-border)] px-3 py-2 text-sm font-semibold text-[var(--lp-danger)] disabled:opacity-60"
                      >
                        Delete
                      </button>
                    </div>
                  </li>
                ))}
              </ul>
            )}
          </Surface>
        </Reveal>

        {formOpen ? (
          <Reveal delay={3}>
            <Surface>
              <h2 className="text-lg font-semibold">
                {editingId ? `Edit role ${formName}` : "Create custom role"}
              </h2>
              <p className="mt-1 text-sm text-[var(--lp-ink-muted)]">
                {editingId
                  ? "Replace the permission set for this role."
                  : "Pick a code-like name and the permissions it grants."}
              </p>
              <div className="mt-4 space-y-4">
                {!editingId ? (
                  <input
                    className="lp-input"
                    value={formName}
                    onChange={(event) => {
                      setFormName(event.target.value);
                    }}
                    placeholder="team_lead"
                    maxLength={64}
                  />
                ) : null}
                <div className="grid gap-4 md:grid-cols-2">
                  {permissionGroups.map((group) => (
                    <fieldset
                      key={group.resource}
                      className="rounded-[var(--lp-radius)] border border-[var(--lp-border)] p-4"
                    >
                      <legend className="px-1 text-sm font-semibold capitalize">
                        {group.resource}
                      </legend>
                      <ul className="mt-1 space-y-2">
                        {group.permissions.map((permission) => (
                          <li key={permission}>
                            <label className="flex items-center gap-2 text-sm">
                              <input
                                type="checkbox"
                                checked={formPermissions.includes(permission)}
                                onChange={() => {
                                  togglePermission(permission);
                                }}
                              />
                              {permission}
                            </label>
                          </li>
                        ))}
                      </ul>
                    </fieldset>
                  ))}
                </div>
                <div className="flex gap-3">
                  <button
                    type="button"
                    disabled={pending}
                    onClick={onSubmit}
                    className="rounded-[var(--lp-radius)] bg-[var(--lp-accent)] px-4 py-2.5 text-sm font-semibold text-white disabled:opacity-60"
                  >
                    {editingId ? "Save changes" : "Create role"}
                  </button>
                  <button
                    type="button"
                    disabled={pending}
                    onClick={closeForm}
                    className="rounded-[var(--lp-radius)] border border-[var(--lp-border)] px-4 py-2.5 text-sm font-semibold disabled:opacity-60"
                  >
                    Cancel
                  </button>
                </div>
              </div>
            </Surface>
          </Reveal>
        ) : null}
      </div>
      );
}
