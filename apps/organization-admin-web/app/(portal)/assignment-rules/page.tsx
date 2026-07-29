"use client";

import { useEffect, useState, useTransition, type FormEvent } from "react";
import { useRouter } from "next/navigation";
import type { AssignmentRule, Department, JobRole, JourneyTemplate } from "@launchpad/api-client";
import { ApiError } from "@launchpad/api-client";
import { Select, EmptyState, PageHeader, Reveal, Surface } from "@launchpad/ui";
import { getClient } from "@/lib/api";
import { clearSession, getAccessToken } from "@/lib/session";

export default function AssignmentRulesPage() {
  const router = useRouter();
  const [pending, startTransition] = useTransition();
  const [rules, setRules] = useState<AssignmentRule[]>([]);
  const [journeys, setJourneys] = useState<JourneyTemplate[]>([]);
  const [departments, setDepartments] = useState<Department[]>([]);
  const [jobRoles, setJobRoles] = useState<JobRole[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [message, setMessage] = useState<string | null>(null);

  const publishedJourneys = journeys.filter((journey) => journey.status === "published");

  function lookupName(items: { id: string; name: string }[], id?: string): string {
    if (!id) return "All";
    return items.find((item) => item.id === id)?.name ?? id;
  }

  function reload(isStale?: () => boolean) {
    startTransition(() => {
      void (async () => {
        try {
          const [ruleItems, journeyItems, departmentItems, jobRoleItems] = await Promise.all([
            getClient().listAssignmentRules(),
            getClient().listJourneys(),
            getClient().listDepartments(),
            getClient().listJobRoles(),
          ]);
          if (isStale?.()) return;
          setRules(ruleItems);
          setJourneys(journeyItems);
          setDepartments(departmentItems);
          setJobRoles(jobRoleItems);
        } catch (err) {
          if (isStale?.()) return;
          if (err instanceof ApiError && err.status === 401) {
            clearSession();
            router.replace("/login");
            return;
          }
          setError(err instanceof ApiError ? err.message : "Unable to load assignment rules");
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

  function onCreateRule(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const form = event.currentTarget;
    const data = new FormData(form);
    setError(null);
    setMessage(null);
    startTransition(() => {
      void (async () => {
        try {
          await getClient().createAssignmentRule({
            name: String(data.get("name") ?? ""),
            journeyTemplateId: String(data.get("journeyTemplateId") ?? ""),
            departmentId: String(data.get("departmentId") ?? "") || undefined,
            jobRoleId: String(data.get("jobRoleId") ?? "") || undefined,
          });
          form.reset();
          setMessage("Rule created — it now applies to every new matching employee.");
          reload();
        } catch (err) {
          setError(err instanceof ApiError ? err.message : "Unable to create rule");
        }
      })();
    });
  }

  function toggleRule(rule: AssignmentRule) {
    setError(null);
    setMessage(null);
    startTransition(() => {
      void (async () => {
        try {
          await getClient().updateAssignmentRule(rule.id, {
            name: rule.name,
            journeyTemplateId: rule.journeyTemplateId,
            departmentId: rule.departmentId,
            jobRoleId: rule.jobRoleId,
            active: !rule.active,
          });
          reload();
        } catch (err) {
          setError(err instanceof ApiError ? err.message : "Unable to update rule");
        }
      })();
    });
  }

  function runRule(rule: AssignmentRule) {
    setError(null);
    setMessage(null);
    startTransition(() => {
      void (async () => {
        try {
          const result = await getClient().runAssignmentRule(rule.id);
          setMessage(
            `Ran "${rule.name}": ${result.assigned} assigned, ${result.skipped} skipped (already assigned).`,
          );
          reload();
        } catch (err) {
          setError(err instanceof ApiError ? err.message : "Unable to run rule");
        }
      })();
    });
  }

  function deleteRule(rule: AssignmentRule) {
    setError(null);
    setMessage(null);
    startTransition(() => {
      void (async () => {
        try {
          await getClient().deleteAssignmentRule(rule.id);
          setMessage(`Deleted "${rule.name}". Existing assignments were left untouched.`);
          reload();
        } catch (err) {
          setError(err instanceof ApiError ? err.message : "Unable to delete rule");
        }
      })();
    });
  }

  return (
    <div className="space-y-8">
      <Reveal>
        <PageHeader
          eyebrow="Operations"
          title="Assignment rules"
          description="Auto-assign published journeys to new employees by department and job role."
        />
      </Reveal>

      {error ? (
        <p className="text-[var(--lp-danger)]" role="alert">
          {error}
        </p>
      ) : null}
      {message ? <p className="text-[var(--lp-success)]">{message}</p> : null}

      <Reveal delay={1}>
        <Surface>
          <h2 className="text-lg font-semibold">New rule</h2>
          <p className="mt-1 text-sm text-[var(--lp-ink-muted)]">
            Matching rules assign the journey automatically when an employee is created or synced
            from your HRIS. Empty criteria match everyone.
          </p>
          <form onSubmit={onCreateRule} className="mt-4 grid gap-3 md:grid-cols-2">
            <input className="lp-input" name="name" placeholder="Engineering onboarding" required />
            <Select className="lp-input" name="journeyTemplateId" defaultValue="" required>
              <option value="" disabled>
                Select a published journey…
              </option>
              {publishedJourneys.map((journey) => (
                <option key={journey.id} value={journey.id}>
                  {journey.name}
                </option>
              ))}
            </Select>
            <Select className="lp-input" name="departmentId" defaultValue="">
              <option value="">All departments</option>
              {departments.map((department) => (
                <option key={department.id} value={department.id}>
                  {department.name}
                </option>
              ))}
            </Select>
            <Select className="lp-input" name="jobRoleId" defaultValue="">
              <option value="">All job roles</option>
              {jobRoles.map((role) => (
                <option key={role.id} value={role.id}>
                  {role.name}
                </option>
              ))}
            </Select>
            <div>
              <button
                type="submit"
                disabled={pending}
                className="rounded-[var(--lp-radius)] bg-[var(--lp-accent)] px-4 py-2.5 text-sm font-semibold text-white disabled:opacity-60"
              >
                Create rule
              </button>
            </div>
          </form>
        </Surface>
      </Reveal>

      <Reveal delay={2}>
        <Surface className="overflow-hidden p-0">
          <div className="border-b border-[var(--lp-border)] px-5 py-4">
            <h2 className="text-lg font-semibold">Rules</h2>
            <p className="text-sm text-[var(--lp-ink-muted)]">{rules.length} rules</p>
          </div>
          {rules.length === 0 ? (
            <div className="p-5">
              <EmptyState
                dense
                title="No assignment rules yet"
                description="Create a rule to auto-assign journeys to new hires."
              />
            </div>
          ) : (
            <ul className="divide-y divide-[var(--lp-border)]">
              {rules.map((rule) => (
                <li
                  key={rule.id}
                  className="flex flex-wrap items-center justify-between gap-3 px-5 py-4"
                >
                  <div>
                    <p className="font-medium">{rule.name}</p>
                    <p className="text-sm text-[var(--lp-ink-muted)]">
                      {lookupName(journeys, rule.journeyTemplateId)} ·{" "}
                      {lookupName(departments, rule.departmentId)} ·{" "}
                      {lookupName(jobRoles, rule.jobRoleId)}
                    </p>
                  </div>
                  <div className="flex items-center gap-2">
                    <button
                      type="button"
                      disabled={pending}
                      onClick={() => toggleRule(rule)}
                      className="rounded-[var(--lp-radius)] border border-[var(--lp-border)] px-3 py-1.5 text-sm font-semibold disabled:opacity-60"
                    >
                      {rule.active ? "Active — pause" : "Paused — activate"}
                    </button>
                    <button
                      type="button"
                      disabled={pending || !rule.active}
                      onClick={() => runRule(rule)}
                      className="rounded-[var(--lp-radius)] bg-[var(--lp-accent)] px-3 py-1.5 text-sm font-semibold text-white disabled:opacity-60"
                    >
                      Run now
                    </button>
                    <button
                      type="button"
                      disabled={pending}
                      onClick={() => deleteRule(rule)}
                      className="rounded-[var(--lp-radius)] border border-[var(--lp-border)] px-3 py-1.5 text-sm font-semibold text-[var(--lp-danger)] disabled:opacity-60"
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
    </div>
  );
}
