"use client";

import { useEffect, useState, useTransition, type SyntheticEvent } from "react";
import { useRouter } from "next/navigation";
import type { Department, Employee, JobRole, JourneyTemplate } from "@launchpad/api-client";
import { ApiError } from "@launchpad/api-client";
import { EmptyState, PageHeader, Reveal, Surface } from "@launchpad/ui";
import { AdminShell } from "@/components/admin-shell";
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
  const [error, setError] = useState<string | null>(null);
  const [message, setMessage] = useState<string | null>(null);

  function reload() {
    startTransition(() => {
      void (async () => {
        try {
          const client = getClient();
          const [employeeItems, departmentItems, roleItems, journeyItems] = await Promise.all([
            client.listEmployees(),
            client.listDepartments(),
            client.listJobRoles(),
            client.listJourneys(),
          ]);
          setEmployees(employeeItems);
          setDepartments(departmentItems);
          setJobRoles(roleItems);
          setJourneys(journeyItems.filter((journey) => journey.status === "published"));
        } catch (err) {
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
    reload();
    // eslint-disable-next-line react-hooks/exhaustive-deps -- initial load only
  }, [router]);

  function onCreateDepartment(event: SyntheticEvent<HTMLFormElement>) {
    event.preventDefault();
    setError(null);
    setMessage(null);
    const form = new FormData(event.currentTarget);
    startTransition(() => {
      void (async () => {
        try {
          await getClient().createDepartment({ name: formString(form, "name") });
          event.currentTarget.reset();
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
    const form = new FormData(event.currentTarget);
    startTransition(() => {
      void (async () => {
        try {
          await getClient().createJobRole({ name: formString(form, "name") });
          event.currentTarget.reset();
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
    const form = new FormData(event.currentTarget);
    startTransition(() => {
      void (async () => {
        try {
          await getClient().createEmployee({
            firstName: formString(form, "firstName"),
            lastName: formString(form, "lastName"),
            workEmail: formString(form, "workEmail"),
            employeeNumber: formString(form, "employeeNumber") || undefined,
            departmentId: formString(form, "departmentId") || undefined,
            jobRoleId: formString(form, "jobRoleId") || undefined,
            startDate: formString(form, "startDate"),
          });
          event.currentTarget.reset();
          setMessage("Employee invited");
          reload();
        } catch (err) {
          setError(err instanceof ApiError ? err.message : "Unable to create employee");
        }
      })();
    });
  }

  function onProvision(event: SyntheticEvent<HTMLFormElement>) {
    event.preventDefault();
    setError(null);
    setMessage(null);
    const form = new FormData(event.currentTarget);
    startTransition(() => {
      void (async () => {
        try {
          await getClient().provisionEmployee(formString(form, "employeeId"), {
            password: formString(form, "password"),
          });
          event.currentTarget.reset();
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
    const form = new FormData(event.currentTarget);
    startTransition(() => {
      void (async () => {
        try {
          await getClient().assignJourney({
            employeeId: formString(form, "employeeId"),
            journeyTemplateId: formString(form, "journeyTemplateId"),
          });
          event.currentTarget.reset();
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
    const form = new FormData(event.currentTarget);
    startTransition(() => {
      void (async () => {
        try {
          await getClient().inviteOrganizationMember({
            email: formString(form, "email"),
            displayName: formString(form, "displayName"),
            password: formString(form, "password"),
            roleCode: "hr_admin",
          });
          event.currentTarget.reset();
          setMessage("HR admin invited");
        } catch (err) {
          setError(err instanceof ApiError ? err.message : "Unable to invite HR admin");
        }
      })();
    });
  }

  return (
    <AdminShell>
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
              <input className="lp-input" name="employeeNumber" placeholder="Employee number" />
              <input className="lp-input" name="startDate" type="date" required />
              <select className="lp-input" name="departmentId" defaultValue="">
                <option value="">No department</option>
                {departments.map((department) => (
                  <option key={department.id} value={department.id}>
                    {department.name}
                  </option>
                ))}
              </select>
              <select className="lp-input" name="jobRoleId" defaultValue="">
                <option value="">No job role</option>
                {jobRoles.map((role) => (
                  <option key={role.id} value={role.id}>
                    {role.name}
                  </option>
                ))}
              </select>
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

        <section className="grid gap-6 lg:grid-cols-2">
          <Surface>
            <h2 className="text-lg font-semibold">Provision portal access</h2>
            <form onSubmit={onProvision} className="mt-4 space-y-3">
              <select className="lp-input" name="employeeId" required defaultValue="">
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
              </select>
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
              <select className="lp-input" name="employeeId" required defaultValue="">
                <option value="" disabled>
                  Select employee
                </option>
                {employees.map((employee) => (
                  <option key={employee.id} value={employee.id}>
                    {employee.firstName} {employee.lastName}
                  </option>
                ))}
              </select>
              <select className="lp-input" name="journeyTemplateId" required defaultValue="">
                <option value="" disabled>
                  Select published journey
                </option>
                {journeys.map((journey) => (
                  <option key={journey.id} value={journey.id}>
                    {journey.name}
                  </option>
                ))}
              </select>
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
                <li
                  key={employee.id}
                  className="flex flex-wrap items-center justify-between gap-2 px-5 py-4"
                >
                  <div>
                    <p className="font-medium">
                      {employee.firstName} {employee.lastName}
                    </p>
                    <p className="text-sm text-[var(--lp-ink-muted)]">{employee.workEmail}</p>
                  </div>
                  <p className="text-sm text-[var(--lp-ink-muted)]">
                    {employee.status}
                    {employee.userId ? " · portal ready" : ""}
                  </p>
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
              <input
                className="lp-input md:col-span-2"
                name="password"
                type="password"
                minLength={10}
                placeholder="Temporary password"
                required
              />
              <button
                type="submit"
                disabled={pending}
                className="rounded-[var(--lp-radius)] bg-[var(--lp-accent)] px-4 py-2.5 text-sm font-semibold text-white disabled:opacity-60 md:col-span-2"
              >
                Invite HR admin
              </button>
            </form>
          </Surface>
        </Reveal>
      </div>
    </AdminShell>
  );
}
