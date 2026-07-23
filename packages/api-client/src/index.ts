import { z } from "zod";

const envelopeSchema = <T extends z.ZodTypeAny>(dataSchema: T) =>
  z.object({
    data: dataSchema.optional(),
    error: z
      .object({
        code: z.string(),
        message: z.string(),
      })
      .optional(),
  });

export const userSchema = z.object({
  id: z.string(),
  email: z.string().email(),
  displayName: z.string(),
  status: z.string(),
});

export const organizationSchema = z.object({
  id: z.string(),
  name: z.string(),
  slug: z.string(),
  status: z.string(),
  planCode: z.string(),
  timezone: z.string(),
  createdAt: z.string(),
  updatedAt: z.string(),
});

export const tokenPairSchema = z.object({
  accessToken: z.string(),
  refreshToken: z.string(),
  tokenType: z.string(),
  expiresIn: z.number().int().nonnegative(),
});

export const authResultSchema = z.object({
  user: userSchema,
  organization: organizationSchema.nullable(),
  tokens: tokenPairSchema,
});

export const meSchema = z.object({
  user: userSchema,
  organization: organizationSchema.nullable(),
  roleCode: z.string(),
  sessionId: z.string(),
});

export const auditEventSchema = z.object({
  id: z.string(),
  organizationId: z.string().optional().nullable(),
  actorUserId: z.string(),
  action: z.string(),
  resourceType: z.string(),
  resourceId: z.string(),
  metadata: z.record(z.unknown()).optional(),
  createdAt: z.string(),
});

export const departmentSchema = z.object({
  id: z.string(),
  organizationId: z.string(),
  name: z.string(),
  description: z.string(),
  createdAt: z.string(),
  updatedAt: z.string(),
});

export const jobRoleSchema = z.object({
  id: z.string(),
  organizationId: z.string(),
  name: z.string(),
  description: z.string(),
  createdAt: z.string(),
  updatedAt: z.string(),
});

export const employeeSchema = z.object({
  id: z.string(),
  organizationId: z.string(),
  userId: z.string().optional(),
  employeeNumber: z.string(),
  firstName: z.string(),
  lastName: z.string(),
  workEmail: z.string().email(),
  jobRoleId: z.string().optional(),
  departmentId: z.string().optional(),
  managerEmployeeId: z.string().optional(),
  startDate: z.string(),
  status: z.string(),
  metadata: z.record(z.unknown()),
  createdAt: z.string(),
  updatedAt: z.string(),
});

export type User = z.infer<typeof userSchema>;
export type Organization = z.infer<typeof organizationSchema>;
export type TokenPair = z.infer<typeof tokenPairSchema>;
export type AuthResult = z.infer<typeof authResultSchema>;
export type MeResponse = z.infer<typeof meSchema>;
export type AuditEvent = z.infer<typeof auditEventSchema>;
export type Department = z.infer<typeof departmentSchema>;
export type JobRole = z.infer<typeof jobRoleSchema>;
export type Employee = z.infer<typeof employeeSchema>;

export class ApiError extends Error {
  readonly code: string;
  readonly status: number;

  constructor(status: number, code: string, message: string) {
    super(message);
    this.name = "ApiError";
    this.status = status;
    this.code = code;
  }
}

export type RegisterRequest = {
  email: string;
  password: string;
  displayName: string;
  organizationName: string;
  organizationSlug?: string;
  timezone?: string;
};

export type LoginRequest = {
  email: string;
  password: string;
  organizationId?: string;
};

export type CreateDepartmentRequest = {
  name: string;
  description?: string;
};

export type CreateJobRoleRequest = {
  name: string;
  description?: string;
};

export type CreateEmployeeRequest = {
  employeeNumber?: string;
  firstName: string;
  lastName: string;
  workEmail: string;
  jobRoleId?: string;
  departmentId?: string;
  managerEmployeeId?: string;
  startDate: string;
};

export const journeyTemplateSchema = z.object({
  id: z.string(),
  organizationId: z.string(),
  name: z.string(),
  description: z.string(),
  status: z.string(),
  currentVersion: z.number().int(),
  createdBy: z.string(),
  createdAt: z.string(),
  updatedAt: z.string(),
});

export const journeyStepSchema = z.object({
  id: z.string(),
  organizationId: z.string(),
  journeyTemplateId: z.string(),
  version: z.number().int(),
  stepType: z.string(),
  title: z.string(),
  instructions: z.string(),
  position: z.number().int(),
  dueOffsetDays: z.number().int(),
  config: z.record(z.unknown()).nullable(),
  createdAt: z.string(),
});

export const journeyAssignmentSchema = z.object({
  id: z.string(),
  organizationId: z.string(),
  employeeId: z.string(),
  journeyTemplateId: z.string(),
  templateVersion: z.number().int(),
  status: z.string(),
  startsAt: z.string(),
  dueAt: z.string().optional().nullable(),
  progressPercent: z.number(),
  completedAt: z.string().optional().nullable(),
  createdAt: z.string(),
});

export const stepAssignmentSchema = z.object({
  id: z.string(),
  organizationId: z.string(),
  journeyAssignmentId: z.string(),
  journeyStepId: z.string(),
  employeeId: z.string(),
  stepType: z.string(),
  title: z.string(),
  instructions: z.string(),
  position: z.number().int(),
  status: z.string(),
  dueAt: z.string().optional().nullable(),
  submission: z.record(z.unknown()).optional().nullable(),
  score: z.number().optional().nullable(),
  completedAt: z.string().optional().nullable(),
  createdAt: z.string(),
});

export const assignResultSchema = z.object({
  assignment: journeyAssignmentSchema,
  steps: z.array(stepAssignmentSchema),
});

export const approvalSchema = z.object({
  id: z.string(),
  organizationId: z.string(),
  stepAssignmentId: z.string(),
  approverUserId: z.string(),
  status: z.string(),
  note: z.string(),
  decidedAt: z.string().optional().nullable(),
  createdAt: z.string(),
});

export const notificationSchema = z.object({
  id: z.string(),
  organizationId: z.string(),
  userId: z.string(),
  title: z.string(),
  body: z.string(),
  readAt: z.string().optional().nullable(),
  createdAt: z.string(),
});

export type JourneyTemplate = z.infer<typeof journeyTemplateSchema>;
export type JourneyStep = z.infer<typeof journeyStepSchema>;
export type JourneyAssignment = z.infer<typeof journeyAssignmentSchema>;
export type StepAssignment = z.infer<typeof stepAssignmentSchema>;
export type AssignResult = z.infer<typeof assignResultSchema>;
export type Approval = z.infer<typeof approvalSchema>;
export type Notification = z.infer<typeof notificationSchema>;

export type CreateJourneyRequest = {
  name: string;
  description?: string;
};

export type AddJourneyStepRequest = {
  stepType: string;
  title: string;
  instructions?: string;
  dueOffsetDays?: number;
  config?: Record<string, unknown>;
};

export type AssignJourneyRequest = {
  employeeId: string;
  journeyTemplateId: string;
  startsAt?: string;
};

export type ProvisionEmployeeRequest = {
  password: string;
  displayName?: string;
};

export type CompleteStepRequest = {
  submission?: Record<string, unknown>;
  score?: number;
};

export type DecideApprovalRequest = {
  approve: boolean;
  note?: string;
};

export const platformOverviewSchema = z.object({
  totalOrgs: z.number().int(),
  trialOrgs: z.number().int(),
  activeOrgs: z.number().int(),
  suspendedOrgs: z.number().int(),
  totalLeads: z.number().int(),
});

export const leadSchema = z.object({
  id: z.string(),
  name: z.string(),
  email: z.string().email(),
  company: z.string(),
  message: z.string(),
  source: z.string(),
  status: z.string(),
  createdAt: z.string(),
});

export const organizationMembershipSchema = z.object({
  id: z.string(),
  organizationId: z.string(),
  userId: z.string(),
  roleCode: z.string(),
  status: z.string(),
  createdAt: z.string(),
});

export type PlatformOverview = z.infer<typeof platformOverviewSchema>;
export type Lead = z.infer<typeof leadSchema>;
export type OrganizationMembership = z.infer<typeof organizationMembershipSchema>;

export type CreateLeadRequest = {
  name: string;
  email: string;
  company?: string;
  message?: string;
  source?: string;
};

export type InviteMemberRequest = {
  email: string;
  displayName: string;
  password: string;
  roleCode?: string;
};

export type LaunchPadClientOptions = {
  baseUrl: string;
  getAccessToken?: () => string | null;
};

async function parseEnvelope<T>(
  response: Response,
  dataSchema: z.ZodType<T>,
): Promise<T> {
  const body: unknown = await response.json();
  const parsed = envelopeSchema(dataSchema).safeParse(body);

  if (!parsed.success) {
    throw new ApiError(response.status, "INVALID_RESPONSE", "Unexpected API response shape");
  }

  if (!response.ok || parsed.data.error) {
    throw new ApiError(
      response.status,
      parsed.data.error?.code ?? "REQUEST_FAILED",
      parsed.data.error?.message ?? "Request failed",
    );
  }

  if (parsed.data.data === undefined) {
    throw new ApiError(response.status, "INVALID_RESPONSE", "Missing response data");
  }

  return parsed.data.data;
}

export function createLaunchPadClient(options: LaunchPadClientOptions) {
  const baseUrl = options.baseUrl.replace(/\/$/, "");

  async function request<T>(
    path: string,
    init: RequestInit,
    dataSchema: z.ZodType<T>,
  ): Promise<T> {
    const headers = new Headers(init.headers);
    headers.set("Accept", "application/json");

    if (init.body && !headers.has("Content-Type")) {
      headers.set("Content-Type", "application/json");
    }

    const token = options.getAccessToken?.();
    if (token) {
      headers.set("Authorization", `Bearer ${token}`);
    }

    const response = await fetch(`${baseUrl}${path}`, {
      ...init,
      headers,
    });

    return parseEnvelope(response, dataSchema);
  }

  return {
    register(payload: RegisterRequest): Promise<AuthResult> {
      return request("/api/v1/auth/register", {
        method: "POST",
        body: JSON.stringify(payload),
      }, authResultSchema);
    },

    login(payload: LoginRequest): Promise<AuthResult> {
      return request("/api/v1/auth/login", {
        method: "POST",
        body: JSON.stringify(payload),
      }, authResultSchema);
    },

    me(): Promise<MeResponse> {
      return request("/api/v1/auth/me", { method: "GET" }, meSchema);
    },

    getCurrentOrganization(): Promise<Organization> {
      return request("/api/v1/organizations/current", { method: "GET" }, organizationSchema);
    },

    listAuditEvents(limit = 20): Promise<AuditEvent[]> {
      return request(
        `/api/v1/audit-events?limit=${String(limit)}`,
        { method: "GET" },
        z.array(auditEventSchema),
      );
    },

    logout(): Promise<{ status: string }> {
      return request(
        "/api/v1/auth/logout",
        { method: "POST" },
        z.object({ status: z.string() }),
      );
    },

    listDepartments(): Promise<Department[]> {
      return request("/api/v1/departments", { method: "GET" }, z.array(departmentSchema));
    },

    createDepartment(payload: CreateDepartmentRequest): Promise<Department> {
      return request("/api/v1/departments", {
        method: "POST",
        body: JSON.stringify(payload),
      }, departmentSchema);
    },

    listJobRoles(): Promise<JobRole[]> {
      return request("/api/v1/job-roles", { method: "GET" }, z.array(jobRoleSchema));
    },

    createJobRole(payload: CreateJobRoleRequest): Promise<JobRole> {
      return request("/api/v1/job-roles", {
        method: "POST",
        body: JSON.stringify(payload),
      }, jobRoleSchema);
    },

    listEmployees(limit = 50): Promise<Employee[]> {
      return request(
        `/api/v1/employees?limit=${String(limit)}`,
        { method: "GET" },
        z.array(employeeSchema),
      );
    },

    createEmployee(payload: CreateEmployeeRequest): Promise<Employee> {
      return request("/api/v1/employees", {
        method: "POST",
        body: JSON.stringify(payload),
      }, employeeSchema);
    },

    getEmployee(employeeId: string): Promise<Employee> {
      return request(`/api/v1/employees/${employeeId}`, { method: "GET" }, employeeSchema);
    },

    provisionEmployee(employeeId: string, payload: ProvisionEmployeeRequest): Promise<Employee> {
      return request(`/api/v1/employees/${employeeId}/provision`, {
        method: "POST",
        body: JSON.stringify(payload),
      }, employeeSchema);
    },

    listJourneys(): Promise<JourneyTemplate[]> {
      return request("/api/v1/journeys", { method: "GET" }, z.array(journeyTemplateSchema));
    },

    createJourney(payload: CreateJourneyRequest): Promise<JourneyTemplate> {
      return request("/api/v1/journeys", {
        method: "POST",
        body: JSON.stringify(payload),
      }, journeyTemplateSchema);
    },

    getJourney(journeyId: string): Promise<JourneyTemplate> {
      return request(`/api/v1/journeys/${journeyId}`, { method: "GET" }, journeyTemplateSchema);
    },

    listJourneySteps(journeyId: string): Promise<JourneyStep[]> {
      return request(
        `/api/v1/journeys/${journeyId}/steps`,
        { method: "GET" },
        z.array(journeyStepSchema),
      );
    },

    addJourneyStep(journeyId: string, payload: AddJourneyStepRequest): Promise<JourneyStep> {
      return request(`/api/v1/journeys/${journeyId}/steps`, {
        method: "POST",
        body: JSON.stringify(payload),
      }, journeyStepSchema);
    },

    publishJourney(journeyId: string): Promise<JourneyTemplate> {
      return request(
        `/api/v1/journeys/${journeyId}/publish`,
        { method: "POST" },
        journeyTemplateSchema,
      );
    },

    listAssignments(): Promise<JourneyAssignment[]> {
      return request("/api/v1/assignments", { method: "GET" }, z.array(journeyAssignmentSchema));
    },

    assignJourney(payload: AssignJourneyRequest): Promise<AssignResult> {
      return request("/api/v1/assignments", {
        method: "POST",
        body: JSON.stringify(payload),
      }, assignResultSchema);
    },

    getAssignment(assignmentId: string): Promise<JourneyAssignment> {
      return request(
        `/api/v1/assignments/${assignmentId}`,
        { method: "GET" },
        journeyAssignmentSchema,
      );
    },

    listAssignmentSteps(assignmentId: string): Promise<StepAssignment[]> {
      return request(
        `/api/v1/assignments/${assignmentId}/steps`,
        { method: "GET" },
        z.array(stepAssignmentSchema),
      );
    },

    listMyAssignments(): Promise<JourneyAssignment[]> {
      return request(
        "/api/v1/me/assignments",
        { method: "GET" },
        z.array(journeyAssignmentSchema),
      );
    },

    completeStep(stepAssignmentId: string, payload: CompleteStepRequest = {}): Promise<StepAssignment> {
      return request(`/api/v1/step-assignments/${stepAssignmentId}/complete`, {
        method: "POST",
        body: JSON.stringify(payload),
      }, stepAssignmentSchema);
    },

    listApprovals(): Promise<Approval[]> {
      return request("/api/v1/approvals", { method: "GET" }, z.array(approvalSchema));
    },

    decideApproval(approvalId: string, payload: DecideApprovalRequest): Promise<Approval> {
      return request(`/api/v1/approvals/${approvalId}/decide`, {
        method: "POST",
        body: JSON.stringify(payload),
      }, approvalSchema);
    },

    listNotifications(): Promise<Notification[]> {
      return request("/api/v1/notifications", { method: "GET" }, z.array(notificationSchema));
    },

    markNotificationRead(notificationId: string): Promise<Notification> {
      return request(
        `/api/v1/notifications/${notificationId}/read`,
        { method: "POST" },
        notificationSchema,
      );
    },

    inviteOrganizationMember(payload: InviteMemberRequest): Promise<OrganizationMembership> {
      return request("/api/v1/organizations/current/members", {
        method: "POST",
        body: JSON.stringify(payload),
      }, organizationMembershipSchema);
    },

    createLead(payload: CreateLeadRequest): Promise<Lead> {
      return request("/api/v1/leads", {
        method: "POST",
        body: JSON.stringify(payload),
      }, leadSchema);
    },

    platformOverview(): Promise<PlatformOverview> {
      return request("/api/v1/platform/overview", { method: "GET" }, platformOverviewSchema);
    },

    listPlatformOrganizations(): Promise<Organization[]> {
      return request(
        "/api/v1/platform/organizations",
        { method: "GET" },
        z.array(organizationSchema),
      );
    },

    getPlatformOrganization(organizationId: string): Promise<Organization> {
      return request(
        `/api/v1/platform/organizations/${organizationId}`,
        { method: "GET" },
        organizationSchema,
      );
    },

    suspendOrganization(organizationId: string): Promise<Organization> {
      return request(
        `/api/v1/platform/organizations/${organizationId}/suspend`,
        { method: "POST" },
        organizationSchema,
      );
    },

    activateOrganization(organizationId: string): Promise<Organization> {
      return request(
        `/api/v1/platform/organizations/${organizationId}/activate`,
        { method: "POST" },
        organizationSchema,
      );
    },

    listPlatformLeads(): Promise<Lead[]> {
      return request("/api/v1/platform/leads", { method: "GET" }, z.array(leadSchema));
    },
  };
}

export type LaunchPadClient = ReturnType<typeof createLaunchPadClient>;
