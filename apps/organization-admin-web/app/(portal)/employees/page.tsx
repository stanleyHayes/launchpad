"use client";

import { useEffect, useState, useTransition, type SyntheticEvent } from "react";
import { useRouter } from "next/navigation";
import type { Department, Employee, Invitation, JobRole, JourneyTemplate } from "@launchpad/api-client";
import { ApiError } from "@launchpad/api-client";
import { Select, EmptyState, PageHeader, Reveal, Surface } from "@launchpad/ui";
import { getClient } from "@/lib/api";
import { clearSession, getAccessToken } from "@/lib/session";

function formString(form: FormData, key: string): string {
  const value = form.get(key);
  return typeof value === "string" ? value.trim() : "";
}

export default function EmployeesPage() {
  const router = useRouter();
  const [pending, startTransition] = useTransition();
  const [employees, setEmployees] = useState<Employee[]>([]);
  const [departments, setDepartments] = useState<Department[]>([]);
  const [jobRoles, setJobRoles] = useState<JobRole[]>([]);
  const [journeys, setJourneys] = useState<JourneyTemplate[]>([]);
  const [invitations, setInvitations] = useState<Invitation[]>([]);
  const [editingId, setEditingId] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [message, setMessage] = useState<string | null>(null);

  function reload(isStale?: () => boolean) {
    startTransition(() => {
      void (async () => {
        try {
          const client = getClient();
          const [employeeItems, departmentItems, roleItems, journeyItems, invitationItems] = await Promise.all([
            client.listEmployees(),
            client.listDepartments(),
            client.listJobRoles(),
            client.listJourneys(),
            client.listOrganizationInvitations(),
          ]);
          if (isStale?.()) return;
          setEmployees(employeeItems);
          setDepartments(departmentItems);
          setJobRoles(roleItems);
          setJourneys(journeyItems.filter((journey) => journey.status === "published"));
          setInvitations(invitationItems);
        } catch (err) {
          if (isStale?.()) return;
          if (err instanceof ApiError && err.status === 401) {
            clearSession();
            router.replace("/login");
            return;
          }
          setError(err instanceof ApiError ? err.message : "Unable to load employees");
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

  function onCreateDepartment(event: SyntheticEvent<HTMLFormElement>) {
    event.preventDefault();
    setError(null);
    setMessage(null);
    const formEl = event.currentTarget;
    const form = new FormData(formEl);
    startTransition(() => {
      void (async () => {
        try {
          await getClient().createDepartment({ name: formString(form, "name") });
          formEl.reset();
          setMessage("Department created");
          reload();
        } catch (err) {
          setError(err instanceof ApiError ? err.message : "Unable to create department");
        }
      })();
    });
  }

  function onCreateJobRole(event: SyntheticEvent<HTMLFormElement>) {
    event.preventDefault();
    setError(null);
    setMessage(null);
    const formEl = event.currentTarget;
    const form = new FormData(formEl);
    startTransition(() => {
      void (async () => {
        try {
          await getClient().createJobRole({ name: formString(form, "name") });
          formEl.reset();
          setMessage("Job role created");
          reload();
        } catch (err) {
          setError(err instanceof ApiError ? err.message : "Unable to create job role");
        }
      })();
    });
  }

  function onCreateEmployee(event: SyntheticEvent<HTMLFormElement>) {
    event.preventDefault();
    setError(null);
    setMessage(null);
    const formEl = event.currentTarget;
    const form = new FormData(formEl);
    startTransition(() => {
      void (async () => {
        try {
          await getClient().createEmployee({
            firstName: formString(form, "firstName"),
            lastName: formString(form, "lastName"),
            workEmail: formString(form, "workEmail"),
            mobilePhone: formString(form, "mobilePhone") || undefined,
            employeeNumber: formString(form, "employeeNumber") || undefined,
            departmentId: formString(form, "departmentId") || undefined,
            jobRoleId: formString(form, "jobRoleId") || undefined,
            buddyEmployeeId: formString(form, "buddyEmployeeId") || undefined,
            team: formString(form, "team") || undefined,
            location: formString(form, "location") || undefined,
            startDate: formString(form, "startDate"),
          });
          formEl.reset();
          setMessage("Employee invited");
          reload();
        } catch (err) {
          setError(err instanceof ApiError ? err.message : "Unable to create employee");
        }
      })();
    });
  }

  function onImportEmployees(event: SyntheticEvent<HTMLFormElement>) {
    event.preventDefault();
    const input = event.currentTarget.elements.namedItem("csv") as HTMLInputElement;
    const file = input.files?.[0];
    if (!file) return;
    startTransition(() => {
      void (async () => {
        try {
          const result = await getClient().importEmployeesCSV(await file.text());
          setMessage(`Imported ${result.created} employees${result.failed ? `; ${result.failed} rows need attention` : ""}`);
          input.value = "";
          reload();
        } catch (err) {
          setError(err instanceof ApiError ? err.message : "Unable to import employees");
        }
      })();
    });
  }

  function onUpdateEmployee(event: SyntheticEvent<HTMLFormElement>) {
    event.preventDefault();
    setError(null);
    setMessage(null);
    const formEl = event.currentTarget;
    const form = new FormData(formEl);
    const employeeId = formString(form, "employeeId");
    startTransition(() => {
      void (async () => {
        try {
          await getClient().updateEmployee(employeeId, {
            firstName: formString(form, "firstName") || undefined,
            lastName: formString(form, "lastName") || undefined,
            employeeNumber: formString(form, "employeeNumber"),
            departmentId: formString(form, "departmentId"),
            jobRoleId: formString(form, "jobRoleId"),
            buddyEmployeeId: formString(form, "buddyEmployeeId"),
            team: formString(form, "team"),
            mobilePhone: formString(form, "mobilePhone"),
            location: formString(form, "location"),
          });
          setEditingId(null);
          setMessage("Employee updated");
          reload();
        } catch (err) {
          setError(err instanceof ApiError ? err.message : "Unable to update employee");
        }
      })();
    });
  }

  function onOffboard(employee: Employee) {
    if (
      !window.confirm(
        `Offboard ${employee.firstName} ${employee.lastName}? They will be excluded from future journey assignments.`,
      )
    ) {
      return;
    }
    setError(null);
    setMessage(null);
    startTransition(() => {
      void (async () => {
        try {
          await getClient().updateEmployee(employee.id, { status: "offboarded" });
          setEditingId(null);
          setMessage("Employee offboarded");
          reload();
        } catch (err) {
          setError(err instanceof ApiError ? err.message : "Unable to offboard employee");
        }
      })();
    });
  }

  function onProvision(event: SyntheticEvent<HTMLFormElement>) {
    event.preventDefault();
    setError(null);
    setMessage(null);
    const formEl = event.currentTarget;
    const form = new FormData(formEl);
    startTransition(() => {
      void (async () => {
        try {
          await getClient().provisionEmployee(formString(form, "employeeId"), {
            password: formString(form, "password"),
          });
          formEl.reset();
          setMessage("Portal access provisioned");
          reload();
        } catch (err) {
          setError(err instanceof ApiError ? err.message : "Unable to provision access");
        }
      })();
    });
  }

  function onAssign(event: SyntheticEvent<HTMLFormElement>) {
    event.preventDefault();
    setError(null);
    setMessage(null);
    const formEl = event.currentTarget;
    const form = new FormData(formEl);
    startTransition(() => {
      void (async () => {
        try {
          await getClient().assignJourney({
            employeeId: formString(form, "employeeId"),
            journeyTemplateId: formString(form, "journeyTemplateId"),
          });
          formEl.reset();
          setMessage("Journey assigned");
        } catch (err) {
          setError(err instanceof ApiError ? err.message : "Unable to assign journey");
        }
      })();
    });
  }

  function onInviteAdmin(event: SyntheticEvent<HTMLFormElement>) {
    event.preventDefault();
    setError(null);
    setMessage(null);
    const formEl = event.currentTarget;
    const form = new FormData(formEl);
    startTransition(() => {
      void (async () => {
        try {
          await getClient().issueOrganizationInvitation({
            email: formString(form, "email"),
            displayName: formString(form, "displayName"),
            role: "hr_admin",
          });
          formEl.reset();
          setMessage("HR admin invited");
        } catch (err) {
          setError(err instanceof ApiError ? err.message : "Unable to invite HR admin");
        }
      })();
    });
  }

  return (
          <div className="space-y-8">
        <Reveal>
          <PageHeader
            eyebrow="People"
            title="Employees"
            description="Manage departments, roles, roster, portal access, and journey assignments."
          />
        </Reveal>

        {error ? (
          <p className="text-[var(--lp-danger)]" role="alert">
            {error}
          </p>
        ) : null}
        {message ? <p className="text-[var(--lp-success)]">{message}</p> : null}

        <Reveal delay={1}>
          <section className="grid gap-6 lg:grid-cols-2">
            <Surface>
              <h2 className="text-lg font-semibold">Add department</h2>
              <form onSubmit={onCreateDepartment} className="mt-4 space-y-3">
                <input className="lp-input" name="name" placeholder="Engineering" required />
                <button
                  type="submit"
                  disabled={pending}
                  className="rounded-[var(--lp-radius)] bg-[var(--lp-accent)] px-4 py-2.5 text-sm font-semibold text-white disabled:opacity-60"
                >
                  Create department
                </button>
              </form>
            </Surface>
            <Surface>
              <h2 className="text-lg font-semibold">Add job role</h2>
              <form onSubmit={onCreateJobRole} className="mt-4 space-y-3">
                <input className="lp-input" name="name" placeholder="Software Engineer" required />
                <button
                  type="submit"
                  disabled={pending}
                  className="rounded-[var(--lp-radius)] bg-[var(--lp-accent)] px-4 py-2.5 text-sm font-semibold text-white disabled:opacity-60"
                >
                  Create job role
                </button>
              </form>
            </Surface>
          </section>
        </Reveal>

        <Reveal delay={2}>
          <Surface>
            <h2 className="text-lg font-semibold">Invite employee</h2>
            <p className="mt-1 text-sm text-[var(--lp-ink-muted)]">
              Creates an invited employee record ready for journey assignment.
            </p>
            <form onSubmit={onCreateEmployee} className="mt-4 grid gap-3 md:grid-cols-2">
              <input className="lp-input" name="firstName" placeholder="First name" required />
              <input className="lp-input" name="lastName" placeholder="Last name" required />
              <input className="lp-input" name="workEmail" type="email" placeholder="Work email" required />
              <input className="lp-input" name="mobilePhone" type="tel" placeholder="Mobile phone (+233…)" />
              <input className="lp-input" name="employeeNumber" placeholder="Employee number" />
              <input className="lp-input" name="team" placeholder="Team" />
              <input className="lp-input" name="location" placeholder="Location" />
              <input className="lp-input" name="startDate" type="date" required />
              <Select className="lp-input" name="departmentId" defaultValue="">
                <option value="">No department</option>
                {departments.map((department) => (
                  <option key={department.id} value={department.id}>
                    {department.name}
                  </option>
                ))}
              </Select>
              <Select className="lp-input" name="jobRoleId" defaultValue="">
                <option value="">No job role</option>
                {jobRoles.map((role) => (
                  <option key={role.id} value={role.id}>
                    {role.name}
                  </option>
                ))}
              </Select>
              <Select className="lp-input" name="buddyEmployeeId" defaultValue="">
                <option value="">No buddy</option>
                {employees.map((employee) => (
                  <option key={employee.id} value={employee.id}>
                    {employee.firstName} {employee.lastName}
                  </option>
                ))}
              </Select>
              <button
                type="submit"
                disabled={pending}
                className="rounded-[var(--lp-radius)] bg-[var(--lp-accent)] px-4 py-2.5 text-sm font-semibold text-white disabled:opacity-60 md:col-span-2"
              >
                Invite employee
              </button>
            </form>
          </Surface>
        </Reveal>

        <Reveal delay={2}>
          <Surface>
            <h2 className="text-lg font-semibold">Bulk CSV import</h2>
            <p className="mt-1 text-sm text-[var(--lp-ink-muted)]">
              Up to 1,000 rows. Required headers: firstName, lastName, workEmail, startDate. Optional: mobilePhone.
            </p>
            <form onSubmit={onImportEmployees} className="mt-4 flex flex-wrap items-center gap-3">
              <input className="lp-input flex-1" name="csv" type="file" accept=".csv,text/csv" required />
              <button type="submit" className="lp-btn lp-btn--primary" disabled={pending}>Import CSV</button>
            </form>
          </Surface>
        </Reveal>

        <section className="grid gap-6 lg:grid-cols-2">
          <Surface>
            <h2 className="text-lg font-semibold">Provision portal access</h2>
            <form onSubmit={onProvision} className="mt-4 space-y-3">
              <Select className="lp-input" name="employeeId" required defaultValue="">
                <option value="" disabled>
                  Select employee
                </option>
                {employees
                  .filter((employee) => !employee.userId)
                  .map((employee) => (
                    <option key={employee.id} value={employee.id}>
                      {employee.firstName} {employee.lastName} · {employee.workEmail}
                    </option>
                  ))}
              </Select>
              <input
                className="lp-input"
                name="password"
                type="password"
                minLength={10}
                placeholder="Temporary password"
                required
              />
              <button
                type="submit"
                disabled={pending}
                className="rounded-[var(--lp-radius)] bg-[var(--lp-accent)] px-4 py-2.5 text-sm font-semibold text-white disabled:opacity-60"
              >
                Provision access
              </button>
            </form>
          </Surface>
          <Surface>
            <h2 className="text-lg font-semibold">Assign journey</h2>
            <form onSubmit={onAssign} className="mt-4 space-y-3">
              <Select className="lp-input" name="employeeId" required defaultValue="">
                <option value="" disabled>
                  Select employee
                </option>
                {employees.map((employee) => (
                  <option key={employee.id} value={employee.id}>
                    {employee.firstName} {employee.lastName}
                  </option>
                ))}
              </Select>
              <Select className="lp-input" name="journeyTemplateId" required defaultValue="">
                <option value="" disabled>
                  Select published journey
                </option>
                {journeys.map((journey) => (
                  <option key={journey.id} value={journey.id}>
                    {journey.name}
                  </option>
                ))}
              </Select>
              <button
                type="submit"
                disabled={pending}
                className="rounded-[var(--lp-radius)] bg-[var(--lp-accent)] px-4 py-2.5 text-sm font-semibold text-white disabled:opacity-60"
              >
                Assign journey
              </button>
            </form>
          </Surface>
        </section>

        <Surface className="overflow-hidden p-0">
          <div className="border-b border-[var(--lp-border)] px-5 py-4">
            <h2 className="text-lg font-semibold">Roster</h2>
            <p className="text-sm text-[var(--lp-ink-muted)]">{employees.length} employees</p>
          </div>
          {employees.length === 0 ? (
            <div className="p-5">
              <EmptyState dense title="No employees yet" description="Invite your first teammate to begin." />
            </div>
          ) : (
            <ul className="divide-y divide-[var(--lp-border)]">
              {employees.map((employee) => (
                <li key={employee.id} className="px-5 py-4">
                  <div className="flex flex-wrap items-center justify-between gap-2">
                    <div>
                      <p className="font-medium">
                        {employee.firstName} {employee.lastName}
                      </p>
                      <p className="text-sm text-[var(--lp-ink-muted)]">{employee.workEmail}</p>
                      {employee.mobilePhone ? <p className="text-xs text-[var(--lp-ink-muted)]">{employee.mobilePhone}</p> : null}
                    </div>
                    <div className="flex items-center gap-3">
                      <p className="text-sm text-[var(--lp-ink-muted)]">
                        {employee.status}
                        {employee.userId ? " · portal ready" : ""}
                      </p>
                      <button
                        type="button"
                        disabled={pending}
                        onClick={() => setEditingId(editingId === employee.id ? null : employee.id)}
                        className="rounded-[var(--lp-radius)] border border-[var(--lp-border)] px-3 py-1.5 text-sm font-medium disabled:opacity-60"
                      >
                        {editingId === employee.id ? "Close" : "Edit"}
                      </button>
                    </div>
                  </div>
                  {editingId === employee.id ? (
                    <div className="mt-4 space-y-3">
                      <form onSubmit={onUpdateEmployee} className="grid gap-3 md:grid-cols-2">
                        <input type="hidden" name="employeeId" value={employee.id} />
                        <input
                          className="lp-input"
                          name="firstName"
                          defaultValue={employee.firstName}
                          placeholder="First name"
                          required
                        />
                        <input
                          className="lp-input"
                          name="lastName"
                          defaultValue={employee.lastName}
                          placeholder="Last name"
                          required
                        />
                        <input
                          className="lp-input"
                          name="employeeNumber"
                          defaultValue={employee.employeeNumber}
                          placeholder="Employee number"
                        />
                        <input className="lp-input" name="team" defaultValue={employee.team ?? ""} placeholder="Team" />
                        <input className="lp-input" name="mobilePhone" type="tel" defaultValue={employee.mobilePhone ?? ""} placeholder="Mobile phone (+233…)" />
                        <input className="lp-input" name="location" defaultValue={employee.location ?? ""} placeholder="Location" />
                        <Select className="lp-input" name="departmentId" defaultValue={employee.departmentId ?? ""}>
                          <option value="">No department</option>
                          {departments.map((department) => (
                            <option key={department.id} value={department.id}>
                              {department.name}
                            </option>
                          ))}
                        </Select>
                        <Select className="lp-input" name="jobRoleId" defaultValue={employee.jobRoleId ?? ""}>
                          <option value="">No job role</option>
                          {jobRoles.map((role) => (
                            <option key={role.id} value={role.id}>
                              {role.name}
                            </option>
                          ))}
                        </Select>
                        <Select
                          className="lp-input"
                          name="buddyEmployeeId"
                          defaultValue={employee.buddyEmployeeId ?? ""}
                        >
                          <option value="">No buddy</option>
                          {employees
                            .filter((buddy) => buddy.id !== employee.id)
                            .map((buddy) => (
                              <option key={buddy.id} value={buddy.id}>
                                {buddy.firstName} {buddy.lastName}
                              </option>
                            ))}
                        </Select>
                        <button
                          type="submit"
                          disabled={pending}
                          className="rounded-[var(--lp-radius)] bg-[var(--lp-accent)] px-4 py-2.5 text-sm font-semibold text-white disabled:opacity-60 md:col-span-2"
                        >
                          Save changes
                        </button>
                      </form>
                      {employee.status !== "offboarded" ? (
                        <button
                          type="button"
                          disabled={pending}
                          onClick={() => onOffboard(employee)}
                          className="rounded-[var(--lp-radius)] border border-[var(--lp-danger)] px-3 py-1.5 text-sm font-medium text-[var(--lp-danger)] disabled:opacity-60"
                        >
                          Offboard employee
                        </button>
                      ) : null}
                    </div>
                  ) : null}
                </li>
              ))}
            </ul>
          )}
        </Surface>

        <Reveal delay={3}>
          <Surface>
            <h2 className="text-lg font-semibold">Invite HR admin</h2>
            <p className="mt-1 text-sm text-[var(--lp-ink-muted)]">
              Add another HR administrator who can manage employees and journeys.
            </p>
            <form onSubmit={onInviteAdmin} className="mt-4 grid gap-3 md:grid-cols-2">
              <input className="lp-input" name="displayName" placeholder="Full name" required />
              <input className="lp-input" name="email" type="email" placeholder="Work email" required />
              <button
                type="submit"
                disabled={pending}
                className="rounded-[var(--lp-radius)] bg-[var(--lp-accent)] px-4 py-2.5 text-sm font-semibold text-white disabled:opacity-60 md:col-span-2"
              >
                Invite HR admin
              </button>
            </form>
            {invitations.length ? (
              <ul className="mt-5 divide-y divide-[var(--lp-line)]">
                {invitations.map((invitation) => (
                  <li key={invitation.id} className="flex flex-wrap items-center justify-between gap-3 py-3 text-sm">
                    <span><strong>{invitation.email}</strong> · {invitation.roleCode} · expires {new Date(invitation.expiresAt).toLocaleString()}</span>
                    <span className="flex gap-3">
                      <button type="button" className="text-[var(--lp-accent)]" onClick={() => {
                        startTransition(() => void getClient().resendOrganizationInvitation(invitation.id).then(() => {
                          setMessage("Invitation resent"); reload();
                        }).catch((err: unknown) => setError(err instanceof ApiError ? err.message : "Unable to resend invitation")));
                      }}>Resend</button>
                      <button type="button" className="text-[var(--lp-danger)]" onClick={() => {
                        startTransition(() => void getClient().revokeOrganizationInvitation(invitation.id).then(() => {
                          setMessage("Invitation revoked"); reload();
                        }).catch((err: unknown) => setError(err instanceof ApiError ? err.message : "Unable to revoke invitation")));
                      }}>Revoke</button>
                    </span>
                  </li>
                ))}
              </ul>
            ) : null}
          </Surface>
        </Reveal>
      </div>
      );
}
