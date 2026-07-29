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

export const brandingSchema = z.object({
  primaryColor: z.string().optional(),
  primaryHoverColor: z.string().optional(),
  accentColor: z.string().optional(),
  logoUrl: z.string().optional(),
});

export const channelStatusSchema = z.object({
  organizationId: z.string(),
  slackConfigured: z.boolean(),
  teamsConfigured: z.boolean(),
  updatedAt: z.string(),
});

export const assistantQuestionStatSchema = z.object({
  question: z.string(),
  count: z.number().int().nonnegative(),
});

export const assistantReportSchema = z.object({
  totalQuestions: z.number().int().nonnegative(),
  refusalCount: z.number().int().nonnegative(),
  refusalRate: z.number().nonnegative(),
  feedbackCount: z.number().int().nonnegative(),
  helpfulCount: z.number().int().nonnegative(),
  helpfulRate: z.number().nonnegative(),
  topRefusedQuestions: z.array(assistantQuestionStatSchema),
  generatedAt: z.string(),
});

export const journeyVersionSummarySchema = z.object({
  version: z.number().int(),
  status: z.string(),
  stepCount: z.number().int(),
});

export const onboardingBreakdownRowSchema = z.object({
  id: z.string(),
  name: z.string(),
  employeeCount: z.number().int().nonnegative(),
  assignmentCount: z.number().int().nonnegative(),
  completedAssignmentCount: z.number().int().nonnegative(),
  completionRate: z.number().nonnegative(),
});

export const onboardingBreakdownSchema = z.object({
  by: z.string(),
  rows: z.array(onboardingBreakdownRowSchema),
  generatedAt: z.string(),
});

export type AssistantQuestionStat = z.infer<typeof assistantQuestionStatSchema>;
export type AssistantReport = z.infer<typeof assistantReportSchema>;
export type JourneyVersionSummary = z.infer<typeof journeyVersionSummarySchema>;
export type OnboardingBreakdown = z.infer<typeof onboardingBreakdownSchema>;
export const funnelReportSchema = z.object({
  milestones: z.array(z.object({
    stepTitle: z.string(), position: z.number().int(), reached: z.number().int(),
    total: z.number().int(), rate: z.number(),
  })),
  dropOff: z.array(z.object({
    stepTitle: z.string(), position: z.number().int(), count: z.number().int(),
  })),
  generatedAt: z.string(),
});
export type FunnelReport = z.infer<typeof funnelReportSchema>;
export type OnboardingBreakdownGroupBy = "department" | "jobRole";
export type OnboardingBreakdownRow = z.infer<typeof onboardingBreakdownRowSchema>;
export type SubmitStepRequest = {
  submission: Record<string, unknown>;
};

export type UpdateOrganizationRequest = {
  name?: string;
  timezone?: string;
  branding?: z.infer<typeof brandingSchema>;
  customDomain?: string;
};

export type SetChannelsRequest = {
  slackWebhookUrl?: string;
  teamsWebhookUrl?: string;
};

export type ChannelStatus = z.infer<typeof channelStatusSchema>;

export const organizationSchema = z.object({
  id: z.string(),
  name: z.string(),
  slug: z.string(),
  status: z.string(),
  planCode: z.string(),
  timezone: z.string(),
  customDomain: z.string().optional(),
  branding: brandingSchema.optional(),
  setupStep: z.number().int().min(0).max(10).optional(),
  setupCompletedAt: z.string().optional(),
  createdAt: z.string(),
  updatedAt: z.string(),
});

export const organizationPageSchema = z.object({
  items: z.array(organizationSchema),
  total: z.number().int().nonnegative(),
  offset: z.number().int().nonnegative(),
  limit: z.number().int().positive(),
});

export const entitlementUsageItemSchema = z.object({
  resource: z.string(),
  used: z.number().int().nonnegative(),
  limit: z.number().int(),
});

export const entitlementUsageSchema = z.object({
  planCode: z.string(),
  items: z.array(entitlementUsageItemSchema),
});

export const platformOrganizationDetailSchema = z.object({
  organization: organizationSchema,
  usage: entitlementUsageSchema,
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

export const organizationChoiceSchema = z.object({
  organization: organizationSchema,
  roleCode: z.string(),
});

export const mfaRequiredSchema = z.object({
  mfaRequired: z.literal(true),
  mfaTicket: z.string(),
});

export const loginResponseSchema = z.union([mfaRequiredSchema, authResultSchema]);

export const mfaEnrollResultSchema = z.object({
  secret: z.string(),
  otpauthUrl: z.string(),
  backupCodes: z.array(z.string()),
});

export const meSchema = z.object({
  user: userSchema,
  organization: organizationSchema.nullable(),
  roleCode: z.string(),
  sessionId: z.string(),
  mfaEnabled: z.boolean().default(false),
  permissions: z.array(z.string()).default([]),
  // Present only when the session runs under a platform support
  // impersonation token; the tenant portal banners this state.
  impersonation: z
    .object({
      sessionId: z.string(),
      agentUserId: z.string(),
    })
    .optional(),
});

export const supportSessionSchema = z.object({
  id: z.string(),
  organizationId: z.string(),
  agentUserId: z.string(),
  reason: z.string(),
  createdAt: z.string(),
  expiresAt: z.string(),
  endedAt: z.string().optional(),
  endReason: z.string().optional(),
});

export const supportSessionCreatedSchema = z.object({
  session: supportSessionSchema,
  token: z.string(),
  tokenExpiresAt: z.string(),
});

export const auditEventSchema = z.object({
  id: z.string(),
  organizationId: z.string().optional().nullable(),
  actorUserId: z.string(),
  actorType: z.string(),
  action: z.string(),
  resourceType: z.string(),
  resourceId: z.string(),
  metadata: z.record(z.string(), z.unknown()).optional(),
  before: z.unknown().optional(),
  after: z.unknown().optional(),
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
  mobilePhone: z.string().optional(),
  jobRoleId: z.string().optional(),
  departmentId: z.string().optional(),
  managerEmployeeId: z.string().optional(),
  buddyEmployeeId: z.string().optional(),
  team: z.string().optional(),
  location: z.string().optional(),
  startDate: z.string(),
  status: z.string(),
  metadata: z.record(z.string(), z.unknown()),
  createdAt: z.string(),
  updatedAt: z.string(),
});

export type User = z.infer<typeof userSchema>;
export type Organization = z.infer<typeof organizationSchema>;
export type OrganizationPage = z.infer<typeof organizationPageSchema>;
export type EntitlementUsageItem = z.infer<typeof entitlementUsageItemSchema>;
export type EntitlementUsage = z.infer<typeof entitlementUsageSchema>;
export type PlatformOrganizationDetail = z.infer<typeof platformOrganizationDetailSchema>;
export type TokenPair = z.infer<typeof tokenPairSchema>;
export type AuthResult = z.infer<typeof authResultSchema>;
export type OrganizationChoice = z.infer<typeof organizationChoiceSchema>;
export type MFARequired = z.infer<typeof mfaRequiredSchema>;
export type LoginResponse = z.infer<typeof loginResponseSchema>;
export type MFAEnrollResult = z.infer<typeof mfaEnrollResultSchema>;
export type MeResponse = z.infer<typeof meSchema>;
export type AuditEvent = z.infer<typeof auditEventSchema>;
export type SupportSession = z.infer<typeof supportSessionSchema>;
export type SupportSessionCreated = z.infer<typeof supportSessionCreatedSchema>;
export type Department = z.infer<typeof departmentSchema>;
export type JobRole = z.infer<typeof jobRoleSchema>;
export type Employee = z.infer<typeof employeeSchema>;
export const employeeContactSchema = z.object({
  id: z.string(),
  kind: z.enum(["manager", "buddy"]),
  name: z.string(),
  workEmail: z.string().email(),
  team: z.string().optional(),
  location: z.string().optional(),
});
export type EmployeeContact = z.infer<typeof employeeContactSchema>;

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

export type MFALoginRequest = {
  ticket: string;
  code: string;
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
  mobilePhone?: string;
  jobRoleId?: string;
  departmentId?: string;
  managerEmployeeId?: string;
  buddyEmployeeId?: string;
  team?: string;
  location?: string;
  startDate: string;
};

export type UpdateEmployeeRequest = {
  firstName?: string;
  lastName?: string;
  employeeNumber?: string;
  mobilePhone?: string;
  jobRoleId?: string;
  departmentId?: string;
  managerEmployeeId?: string;
  buddyEmployeeId?: string;
  team?: string;
  location?: string;
  status?: "invited" | "active" | "offboarded";
};

export const employeeImportResultSchema = z.object({
  created: z.number().int(),
  failed: z.number().int(),
  errors: z.array(z.object({ row: z.number().int(), message: z.string() })),
});
export type EmployeeImportResult = z.infer<typeof employeeImportResultSchema>;
export const invitationSchema = z.object({
  id: z.string(),
  userId: z.string(),
  organizationId: z.string(),
  roleCode: z.string(),
  email: z.string().email(),
  expiresAt: z.string(),
  createdAt: z.string(),
});
export type Invitation = z.infer<typeof invitationSchema>;

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
  businessDays: z.boolean(),
  stage: z.string().optional(),
  parallelGroup: z.string().optional(),
  prerequisiteStepIds: z.array(z.string()).optional(),
  locale: z.string().optional(),
  config: z.record(z.string(), z.unknown()).nullable(),
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

export const quizQuestionViewSchema = z.object({
  id: z.string(),
  text: z.string(),
  options: z.array(z.string()),
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
  stage: z.string().optional(),
  parallelGroup: z.string().optional(),
  prerequisiteStepIds: z.array(z.string()).optional(),
  locale: z.string().optional(),
  status: z.string(),
  dueAt: z.string().optional().nullable(),
  startedAt: z.string().optional().nullable(),
  submission: z.record(z.string(), z.unknown()).optional().nullable(),
  score: z.number().optional().nullable(),
  attemptCount: z.number().int().nonnegative().optional(),
  maxAttempts: z.number().int().nonnegative().optional(),
  escalatedAt: z.string().optional().nullable(),
  overrideAction: z.string().optional(),
  overrideReason: z.string().optional(),
  overrideByUserId: z.string().optional(),
  overriddenAt: z.string().optional().nullable(),
  quizQuestions: z.array(quizQuestionViewSchema).optional().nullable(),
  assessmentId: z.string().optional().nullable(),
  completedAt: z.string().optional().nullable(),
  createdAt: z.string(),
});

export const assignResultSchema = z.object({
  assignment: journeyAssignmentSchema,
  steps: z.array(stepAssignmentSchema),
});

export const assignDepartmentResultSchema = z.object({
  employees: z.number(),
  assigned: z.number(),
  skipped: z.number(),
});

export const assignmentRuleSchema = z.object({
  id: z.string(),
  organizationId: z.string(),
  name: z.string(),
  journeyTemplateId: z.string(),
  departmentId: z.string().optional(),
  jobRoleId: z.string().optional(),
  active: z.boolean(),
  createdBy: z.string(),
  createdAt: z.string(),
});

export type AssignmentRule = z.infer<typeof assignmentRuleSchema>;
export type RunAssignmentRuleResult = z.infer<typeof assignDepartmentResultSchema>;

export type CreateAssignmentRuleRequest = {
  name: string;
  journeyTemplateId: string;
  departmentId?: string;
  jobRoleId?: string;
};

export type UpdateAssignmentRuleRequest = {
  name: string;
  journeyTemplateId: string;
  departmentId?: string;
  jobRoleId?: string;
  active: boolean;
};

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

export const blockerCategorySchema = z.enum(["hr", "it", "manager", "other"]);

export const blockerSchema = z.object({
  id: z.string(),
  organizationId: z.string(),
  employeeId: z.string(),
  reportedByUserId: z.string(),
  stepAssignmentId: z.string().optional(),
  category: blockerCategorySchema,
  message: z.string(),
  ticketId: z.string(),
  createdAt: z.string(),
});

export const managerTeamReportSchema = z.object({
  employeeId: z.string(),
  name: z.string(),
  activeAssignments: z.number().int().nonnegative(),
  completedAssignments: z.number().int().nonnegative(),
  overdueSteps: z.number().int().nonnegative(),
  pendingApprovals: z.number().int().nonnegative(),
});

export const notificationSchema = z.object({
  id: z.string(),
  organizationId: z.string(),
  userId: z.string(),
  type: z.string(),
  title: z.string(),
  body: z.string(),
  link: z.string().optional().nullable(),
  readAt: z.string().optional().nullable(),
  createdAt: z.string(),
});

export type JourneyTemplate = z.infer<typeof journeyTemplateSchema>;
export type JourneyStep = z.infer<typeof journeyStepSchema>;
export type JourneyAssignment = z.infer<typeof journeyAssignmentSchema>;
export type StepAssignment = z.infer<typeof stepAssignmentSchema>;
export type QuizQuestionView = z.infer<typeof quizQuestionViewSchema>;
export type AssignResult = z.infer<typeof assignResultSchema>;
export type Approval = z.infer<typeof approvalSchema>;
export type Blocker = z.infer<typeof blockerSchema>;
export type BlockerCategory = z.infer<typeof blockerCategorySchema>;
export type ManagerTeamReport = z.infer<typeof managerTeamReportSchema>;
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
  businessDays?: boolean;
  stage?: string;
  parallelGroup?: string;
  prerequisiteStepIds?: string[];
  locale?: string;
  config?: Record<string, unknown>;
};

// A single-choice quiz question with its answer key (builder side only).
export type QuizQuestion = {
  id: string;
  text: string;
  options: string[];
  correctOption: number;
};

// The typed shape of a quiz step's config.
export type QuizConfig = {
  questions: QuizQuestion[];
};

// Maps questionId -> selected option index; submitted as submission.answers.
export type QuizAnswers = Record<string, number>;

export type AssignJourneyRequest = {
  employeeId: string;
  journeyTemplateId: string;
  startsAt?: string;
};

export type AssignDepartmentRequest = {
  departmentId: string;
  journeyTemplateId: string;
  startsAt?: string;
};

export type AssignDepartmentResult = z.infer<typeof assignDepartmentResultSchema>;

export type ProvisionEmployeeRequest = {
  password: string;
  displayName?: string;
};

export type CompleteStepRequest = {
  submission?: Record<string, unknown>;
  score?: number;
};

export type OverrideStepRequest = {
  action: "complete" | "skip" | "reopen";
  reason: string;
};

export type DecideApprovalRequest = {
  approve: boolean;
  note?: string;
};

export type ReportBlockerRequest = {
  stepAssignmentId?: string;
  category: BlockerCategory;
  message: string;
};

export const platformOverviewSchema = z.object({
  totalOrgs: z.number().int(),
  trialOrgs: z.number().int(),
  activeOrgs: z.number().int(),
  suspendedOrgs: z.number().int(),
  totalLeads: z.number().int(),
  openTicketCount: z.number().int(),
  overdueTicketCount: z.number().int(),
  urgentTicketCount: z.number().int(),
  mrrTotalCents: z.number().int(),
  arrTotalCents: z.number().int(),
  activeSubscriptions: z.number().int(),
});

export const featureFlagSchema = z.object({
  key: z.string(),
  description: z.string(),
  enabled: z.boolean(),
  planCodes: z.array(z.string()).nullish().transform((value) => value ?? []),
  rolloutPercentage: z.number().int().min(1).max(100).default(100),
  cohortUserIds: z.array(z.string()).nullish().transform((value) => value ?? []),
  expiresAt: z.string().optional(),
  createdAt: z.string(),
  updatedAt: z.string(),
});

export const featureFlagHistorySchema = z.object({
  id: z.string(),
  key: z.string(),
  action: z.string(),
  actorUserId: z.string(),
  organizationId: z.string().optional(),
  snapshot: featureFlagSchema,
  createdAt: z.string(),
});

export const orgFeatureFlagsSchema = z.object({
  flags: z.record(z.string(), z.boolean()),
});

export const marketplaceStepSchema = z.object({
  stepType: z.string(),
  title: z.string(),
  instructions: z.string(),
  dueOffsetDays: z.number().int(),
  config: z.record(z.string(), z.unknown()),
});

export const marketplaceTemplateSchema = z.object({
  id: z.string(),
  name: z.string(),
  slug: z.string(),
  description: z.string(),
  category: z.string(),
  status: z.string(),
  official: z.boolean(),
  featured: z.boolean(),
  version: z.number().int(),
  submittedByOrganizationId: z.string().optional(),
  steps: z.array(marketplaceStepSchema),
  installationCount: z.number().int(),
  ratingAverage: z.number(),
  ratingCount: z.number().int(),
  priceCents: z.number().int().nonnegative(),
  currency: z.string(),
  createdBy: z.string(),
  createdAt: z.string(),
  updatedAt: z.string(),
});

export const marketplacePurchaseSchema = z.object({
  id: z.string(),
  templateId: z.string(),
  organizationId: z.string(),
  buyerUserId: z.string(),
  sellerOrganizationId: z.string(),
  amountCents: z.number().int(),
  currency: z.string(),
  platformFeeCents: z.number().int(),
  sellerEarningsCents: z.number().int(),
  reference: z.string(),
  status: z.string(),
  installationId: z.string().optional(),
  journeyTemplateId: z.string().optional(),
  paidAt: z.string().optional(),
  createdAt: z.string(),
  updatedAt: z.string(),
});

export const marketplaceCheckoutSchema = z.object({
  authorizationUrl: z.string().url(),
  reference: z.string(),
  purchase: marketplacePurchaseSchema,
});

export const marketplaceInstallationSchema = z.object({
  id: z.string(),
  templateId: z.string(),
  templateVersion: z.number().int(),
  organizationId: z.string(),
  journeyTemplateId: z.string(),
  installedBy: z.string(),
  installedAt: z.string(),
});

export const planSchema = z.object({
  code: z.string(),
  name: z.string(),
  description: z.string(),
  priceMonthlyCents: z.number().int(),
  currency: z.string(),
  features: z.array(z.string()),
  active: z.boolean(),
  createdAt: z.string(),
  updatedAt: z.string(),
});

export const subscriptionSchema = z.object({
  id: z.string(),
  organizationId: z.string(),
  planCode: z.string(),
  status: z.string(),
  currentPeriodEnd: z.string().optional().nullable(),
  createdAt: z.string(),
  updatedAt: z.string(),
});

export const invoiceSchema = z.object({
  id: z.string(),
  organizationId: z.string(),
  number: z.string(),
  subscriptionId: z.string(),
  planCode: z.string(),
  amountCents: z.number().int(),
  subtotalCents: z.number().int(),
  taxCents: z.number().int(),
  discountCents: z.number().int(),
  couponCode: z.string().optional(),
  currency: z.string(),
  status: z.string(),
  periodStart: z.string(),
  periodEnd: z.string(),
  dueAt: z.string(),
  paidAt: z.string().optional(),
  dunningAttempts: z.number().int(),
  lastDunningAt: z.string().optional(),
  refundedAt: z.string().optional(),
  refundAmountCents: z.number().int(),
  refundReason: z.string().optional(),
  createdAt: z.string(),
});
export const couponSchema = z.object({
  code: z.string(),
  percentOffBasisPoints: z.number().int(),
  amountOffCents: z.number().int(),
  currency: z.string().optional(),
  maxRedemptions: z.number().int(),
  redemptionCount: z.number().int(),
  expiresAt: z.string().optional(),
  active: z.boolean(),
  createdAt: z.string(),
});

export const supportTicketSchema = z.object({
  id: z.string(),
  organizationId: z.string(),
  createdByUserId: z.string(),
  subject: z.string(),
  body: z.string(),
  priority: z.string(),
  category: z.string().optional(),
  status: z.string(),
  assigneeUserId: z.string().optional(),
  slaDueAt: z.string().optional(),
  firstResponseAt: z.string().optional(),
  resolvedAt: z.string().optional(),
  escalationCount: z.number().int().optional(),
  tags: z.array(z.string()).optional(),
  messages: z.array(z.object({
    id: z.string(),
    authorUserId: z.string(),
    body: z.string(),
    internal: z.boolean(),
    createdAt: z.string(),
  })).optional(),
  createdAt: z.string(),
  updatedAt: z.string(),
});
export const supportSummarySchema = z.object({
  open: z.number().int(),
  overdue: z.number().int(),
  urgent: z.number().int(),
  unassigned: z.number().int(),
  averageFirstResponseMinutes: z.number(),
});
export type SupportSummary = z.infer<typeof supportSummarySchema>;

// Platform staff management (PRD §5.2.6).
export const platformStaffMemberSchema = z.object({
  id: z.string(),
  userId: z.string(),
  email: z.string(),
  displayName: z.string(),
  roleCode: z.string(),
  status: z.string(),
  createdAt: z.string(),
  breakGlass: z.object({
    roleCode: z.string(),
    reason: z.string(),
    approvedBy: z.string(),
    grantedAt: z.string(),
    expiresAt: z.string(),
    revokedAt: z.string().optional(),
    revokedBy: z.string().optional(),
  }).optional(),
  accessReviewedAt: z.string().optional(),
  accessReviewedBy: z.string().optional(),
});

export const accessReviewItemSchema = z.object({
  staff: platformStaffMemberSchema,
  reviewDue: z.boolean(),
  effectiveRoleCode: z.string(),
});
export type AccessReviewItem = z.infer<typeof accessReviewItemSchema>;

export const createPlatformStaffResultSchema = z.object({
  staff: platformStaffMemberSchema,
  tempPassword: z.string().optional(),
  invited: z.boolean(),
});

// Equipment & access requests (PRD §5.3.8): employees raise org-scoped
// requests, managers approve/reject them, and approved requests are marked
// fulfilled once provisioned.
export const orgRequestSchema = z.object({
  id: z.string(),
  organizationId: z.string(),
  kind: z.enum(["equipment", "access"]),
  item: z.string(),
  details: z.string().optional(),
  status: z.enum(["pending", "approved", "fulfilled", "rejected", "cancelled"]),
  requesterEmployeeId: z.string(),
  approverUserId: z.string().optional(),
  decisionNote: z.string().optional(),
  decidedAt: z.string().optional().nullable(),
  fulfilledAt: z.string().optional().nullable(),
  createdAt: z.string(),
  updatedAt: z.string(),
});

// Meetings (PRD §5.3.7): org-scoped scheduled touchpoints between an
// organizer and an employee. Meetings work without a calendar connection
// (location is free text); calendarEventRef appears when the tenant's
// connected provider accepted an event.
export const meetingTypeSchema = z.enum([
  "manager_intro",
  "hr_orientation",
  "team_intro",
  "buddy_checkin",
  "architecture_walkthrough",
  "role_coaching",
  "first_week_review",
]);

export const meetingSchema = z.object({
  id: z.string(),
  organizationId: z.string(),
  title: z.string(),
  type: meetingTypeSchema,
  organizerUserId: z.string().optional(),
  attendeeEmployeeId: z.string(),
  startsAt: z.string(),
  durationMin: z.number().int(),
  location: z.string().optional(),
  status: z.enum(["scheduled", "completed", "cancelled", "no_show"]),
  notesLink: z.string().optional(),
  calendarEventRef: z.string().optional(),
  completedAt: z.string().optional().nullable(),
  createdAt: z.string(),
  updatedAt: z.string(),
});

// Calendar connections: the access token is write-only (encrypted at rest)
// and never returned; the connection exposes the masked account handle only.
export const calendarConnectionSchema = z.object({
  id: z.string(),
  provider: z.enum(["google", "microsoft"]),
  accountHandle: z.string(),
  connected: z.boolean(),
  lastSyncAt: z.string().optional().nullable(),
  lastError: z.string().optional(),
  createdBy: z.string(),
  createdAt: z.string(),
  updatedAt: z.string(),
});

export type Meeting = z.infer<typeof meetingSchema>;
export type MeetingType = z.infer<typeof meetingTypeSchema>;
export type MeetingStatus = Meeting["status"];
export type CalendarConnection = z.infer<typeof calendarConnectionSchema>;

export const samlConfigSchema = z.object({
  organizationId: z.string(),
  enabled: z.boolean(),
  configured: z.boolean(),
  emailAttribute: z.string(),
  metadataUrl: z.string().url(),
  acsUrl: z.string().url(),
  entityId: z.string().url(),
  updatedAt: z.string(),
});
export type SAMLConfig = z.infer<typeof samlConfigSchema>;

export type CreateMeetingRequest = {
  title: string;
  type: MeetingType;
  attendeeEmployeeId: string;
  startsAt: string;
  durationMin?: number;
  location?: string;
  notesLink?: string;
};

export type CompleteMeetingRequest = {
  noShow?: boolean;
  notesLink?: string;
};

export type RescheduleMeetingRequest = {
  startsAt: string;
  durationMin: number;
  location?: string;
};

// Knowledge documents (PRD §5.3.5): draft -> approved -> indexed, with an
// explicit approval gate before content reaches the AI assistant's index.
export const knowledgeDocumentSchema = z.object({
  id: z.string(),
  organizationId: z.string(),
  title: z.string(),
  source: z.string(),
  utmSource: z.string().optional(),
  utmMedium: z.string().optional(),
  utmCampaign: z.string().optional(),
  scheduledFor: z.string().optional().nullable(),
  uri: z.string().optional(),
  body: z.string().optional(),
  tags: z.array(z.string()).optional(),
  accessScope: z.string(),
  status: z.string(),
  version: z.number().int(),
  ownerUserId: z.string(),
  reviewDate: z.string().optional().nullable(),
  retentionDays: z.number().int().nonnegative().optional(),
  lastSyncedAt: z.string().optional().nullable(),
  staleNotifiedAt: z.string().optional().nullable(),
  syncError: z.string().optional(),
  approvedByUserId: z.string().optional(),
  approvedAt: z.string().optional().nullable(),
  indexedAt: z.string().optional().nullable(),
  createdByUserId: z.string(),
  createdAt: z.string(),
  updatedAt: z.string(),
});

export type KnowledgeDocument = z.infer<typeof knowledgeDocumentSchema>;
export const knowledgeVersionSchema = z.object({
  version: z.number().int(),
  title: z.string(),
  body: z.string(),
  uri: z.string(),
  tags: z.array(z.string()),
  accessScope: z.string(),
  savedAt: z.string(),
});
export type KnowledgeVersion = z.infer<typeof knowledgeVersionSchema>;
export type KnowledgeDocumentStatus = "draft" | "approved" | "indexed" | "archived";
export type KnowledgeSource =
  | "manual"
  | "upload"
  | "url"
  | "notion"
  | "confluence"
  | "google_drive"
  | "github"
  | "sharepoint"
  | "wiki";
export type KnowledgeAccessScope = "organization" | "restricted";

export type CreateKnowledgeDocumentRequest = {
  title: string;
  source?: KnowledgeSource;
  uri?: string;
  body?: string;
  tags?: string[];
  accessScope?: KnowledgeAccessScope;
  reviewDate?: string;
  retentionDays?: number;
};

export type UpdateKnowledgeDocumentRequest = {
  title?: string;
  uri?: string;
  body?: string;
  tags?: string[];
  accessScope?: KnowledgeAccessScope;
  reviewDate?: string;
  retentionDays?: number;
};

// Assessments (PRD §5.3.6): server-graded question sets with attempt limits,
// manager review of unmatched short answers, and certificates on a pass.
export const assessmentQuestionTypeSchema = z.enum([
  "single_choice",
  "multiple_choice",
  "true_false",
  "short_answer",
]);

export const assessmentQuestionSchema = z.object({
  id: z.string(),
  type: assessmentQuestionTypeSchema,
  text: z.string(),
  options: z.array(z.string()).optional().nullable(),
  correctOptions: z.array(z.number().int()).optional().nullable(),
  acceptedAnswers: z.array(z.string()).optional().nullable(),
  points: z.number().int(),
});

export const assessmentSchema = z.object({
  id: z.string(),
  organizationId: z.string(),
  title: z.string(),
  description: z.string(),
  questions: z.array(assessmentQuestionSchema),
  passingScore: z.number(),
  maxAttempts: z.number().int(),
  randomize: z.boolean(),
  status: z.string(),
  createdBy: z.string(),
  createdAt: z.string(),
  updatedAt: z.string(),
});

// The answer-key-free question view employees take (never carries
// correctOptions/acceptedAnswers).
export const assessmentQuestionViewSchema = z.object({
  id: z.string(),
  type: assessmentQuestionTypeSchema,
  text: z.string(),
  options: z.array(z.string()).optional().nullable(),
  points: z.number().int(),
});

export const assessmentTakeViewSchema = z.object({
  assessmentId: z.string(),
  title: z.string(),
  description: z.string(),
  passingScore: z.number(),
  questions: z.array(assessmentQuestionViewSchema),
  attemptsUsed: z.number().int(),
  // -1 means unlimited attempts.
  attemptsRemaining: z.number().int(),
});

export const assessmentAnswerSchema = z.object({
  questionId: z.string(),
  options: z.array(z.number().int()).optional(),
  text: z.string().optional(),
});

export const assessmentAttemptSchema = z.object({
  id: z.string(),
  organizationId: z.string(),
  assessmentId: z.string(),
  employeeId: z.string(),
  answers: z.array(assessmentAnswerSchema),
  score: z.number(),
  passed: z.boolean(),
  status: z.string(),
  attemptNumber: z.number().int(),
  reviewNote: z.string().optional().nullable(),
  reviewedBy: z.string().optional().nullable(),
  startedAt: z.string(),
  submittedAt: z.string(),
});

export const certificateSchema = z.object({
  id: z.string(),
  organizationId: z.string(),
  employeeId: z.string(),
  employeeName: z.string(),
  assessmentId: z.string(),
  assessmentTitle: z.string(),
  score: z.number(),
  serial: z.string(),
  issuedAt: z.string(),
});

export type AssessmentQuestionType = z.infer<typeof assessmentQuestionTypeSchema>;
export type AssessmentQuestion = z.infer<typeof assessmentQuestionSchema>;
export type Assessment = z.infer<typeof assessmentSchema>;
export type AssessmentStatus = "draft" | "published" | "archived";
export type AssessmentQuestionView = z.infer<typeof assessmentQuestionViewSchema>;
export type AssessmentTakeView = z.infer<typeof assessmentTakeViewSchema>;
export type AssessmentAnswer = z.infer<typeof assessmentAnswerSchema>;
export type AssessmentAttempt = z.infer<typeof assessmentAttemptSchema>;
export type AssessmentAttemptStatus = "graded" | "pending_review";
export type Certificate = z.infer<typeof certificateSchema>;

// Builder-side question input; ids are optional (the server assigns q1..qN).
export type AssessmentQuestionInput = {
  id?: string;
  type: AssessmentQuestionType;
  text: string;
  options?: string[];
  correctOptions?: number[];
  acceptedAnswers?: string[];
  points?: number;
};

export type CreateAssessmentRequest = {
  title: string;
  description?: string;
  questions: AssessmentQuestionInput[];
  passingScore: number;
  maxAttempts?: number;
  randomize?: boolean;
};

export type UpdateAssessmentRequest = {
  title?: string;
  description?: string;
  questions?: AssessmentQuestionInput[];
  passingScore?: number;
  maxAttempts?: number;
  randomize?: boolean;
};

export type SubmitAssessmentAttemptRequest = {
  answers: AssessmentAnswer[];
};

export type ReviewAssessmentAttemptRequest = {
  score: number;
  note?: string;
};

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

export const roleSchema = z.object({
  id: z.string(),
  organizationId: z.string(),
  name: z.string(),
  permissions: z.array(z.string()),
  builtin: z.boolean(),
  createdAt: z.string(),
  updatedAt: z.string(),
});

export const memberSchema = z.object({
  userId: z.string(),
  membershipId: z.string(),
  roleCode: z.string(),
  status: z.string(),
  email: z.string().optional(),
  displayName: z.string().optional(),
  userStatus: z.string().optional(),
  createdAt: z.string(),
});

export const memberRoleUpdateSchema = z.object({
  userId: z.string(),
  organizationId: z.string(),
  roleCode: z.string(),
});

export const launchReadinessCheckSchema = z.object({
  name: z.string(),
  status: z.enum(["ready", "watch", "blocked"]),
  summary: z.string(),
  action: z.string(),
});

export const launchReadinessReportSchema = z.object({
  checks: z.array(launchReadinessCheckSchema),
});

export type PlatformOverview = z.infer<typeof platformOverviewSchema>;
export type Lead = z.infer<typeof leadSchema>;
export type OrganizationMembership = z.infer<typeof organizationMembershipSchema>;
export type FeatureFlag = z.infer<typeof featureFlagSchema>;
export type MarketplaceTemplate = z.infer<typeof marketplaceTemplateSchema>;
export type MarketplaceStep = z.infer<typeof marketplaceStepSchema>;
export type MarketplaceInstallation = z.infer<typeof marketplaceInstallationSchema>;
export type MarketplacePurchase = z.infer<typeof marketplacePurchaseSchema>;
export type MarketplaceCheckout = z.infer<typeof marketplaceCheckoutSchema>;
export type FeatureFlagHistory = z.infer<typeof featureFlagHistorySchema>;
export type OrgFeatureFlags = z.infer<typeof orgFeatureFlagsSchema>;
export type Plan = z.infer<typeof planSchema>;
export type Subscription = z.infer<typeof subscriptionSchema>;
export type Invoice = z.infer<typeof invoiceSchema>;
export type Coupon = z.infer<typeof couponSchema>;
export type SupportTicket = z.infer<typeof supportTicketSchema>;
export type PlatformStaffMember = z.infer<typeof platformStaffMemberSchema>;
export type CreatePlatformStaffResult = z.infer<typeof createPlatformStaffResultSchema>;
export type OrgRequest = z.infer<typeof orgRequestSchema>;
export type OrgRequestStatus = OrgRequest["status"];
export type Role = z.infer<typeof roleSchema>;
export type Member = z.infer<typeof memberSchema>;
export type MemberRoleUpdate = z.infer<typeof memberRoleUpdateSchema>;
export type LaunchReadinessCheck = z.infer<typeof launchReadinessCheckSchema>;
export type LaunchReadinessReport = z.infer<typeof launchReadinessReportSchema>;

// Mirrors the permission registry in internal/roles/roles.go (PRD 6.3
// `resource.action` codes). Keep in the same order as roles.AllPermissions.
export const LP_PERMISSIONS = [
  "employees.read",
  "employees.create",
  "employees.update",
  "journeys.create",
  "journeys.publish",
  "journeys.assign",
  "assignments.read",
  "assignments.manage",
  "approvals.decide",
  "departments.manage",
  "billing.read",
  "billing.manage",
  "audit.read",
  "analytics.read",
  "knowledge.manage",
  "assessments.manage",
  "integrations.manage",
  "members.read",
  "members.invite",
  "members.update",
  "notifications.read",
  "steps.complete",
  "meetings.manage",
] as const;

export type LpPermission = (typeof LP_PERMISSIONS)[number];

// Assistant (PRD §16.2): grounded answers cite approved knowledge documents;
// the API refuses instead of inventing content when nothing reliable matches.
export const assistantCitationSchema = z.object({
  documentId: z.string(),
  documentTitle: z.string(),
  documentUri: z.string().optional(),
  snippet: z.string(),
});

export const assistantAnswerSchema = z.object({
  interactionId: z.string(),
  text: z.string(),
  citations: z.array(assistantCitationSchema),
  grounded: z.boolean(),
  refused: z.boolean(),
});

export type AssistantCitation = z.infer<typeof assistantCitationSchema>;
export type AssistantAnswer = z.infer<typeof assistantAnswerSchema>;

export type AssistantFeedbackRequest = {
  helpful: boolean;
};

// Integrations: credentials are write-only (encrypted at rest) and never
// returned; connections expose status plus secret-free error text only.
export const integrationConnectionSchema = z.object({
  id: z.string(),
  provider: z.enum(["github", "jira"]),
  status: z.enum(["connected", "error"]),
  baseUrl: z.string().optional(),
  accountHandle: z.string(),
  lastSyncAt: z.string().optional().nullable(),
  lastError: z.string().optional(),
  createdBy: z.string(),
  createdAt: z.string(),
  updatedAt: z.string(),
});

export type IntegrationConnection = z.infer<typeof integrationConnectionSchema>;
export type IntegrationProvider = IntegrationConnection["provider"];

export type IntegrationConnectRequest = {
  token: string;
  baseUrl?: string;
  email?: string;
};

export const onboardingSummarySchema = z.object({
  employeeCount: z.number().int().nonnegative(),
  activeAssignmentCount: z.number().int().nonnegative(),
  completedAssignmentCount: z.number().int().nonnegative(),
  scheduledAssignmentCount: z.number().int().nonnegative(),
  pendingApprovalCount: z.number().int().nonnegative(),
  completionRate: z.number().nonnegative(),
  averageDaysToComplete: z.number().nonnegative(),
  incompleteStepCount: z.number().int().nonnegative(),
  overdueStepCount: z.number().int().nonnegative(),
  overdueRate: z.number().nonnegative(),
  generatedAt: z.string(),
});

export const cmsPageSchema = z.object({
  id: z.string(),
  slug: z.string(),
  title: z.string(),
  summary: z.string(),
  body: z.string(),
  contentType: z.string(),
  navLabel: z.string().optional(),
  navOrder: z.number().int().optional(),
  status: z.string(),
  scheduledAt: z.string().optional().nullable(),
  publishedAt: z.string().optional().nullable(),
  createdAt: z.string(),
  updatedAt: z.string(),
});

export type OnboardingSummary = z.infer<typeof onboardingSummarySchema>;
export type CMSPage = z.infer<typeof cmsPageSchema>;
export const jobStatusSchema = z.object({
  name: z.string(),
  running: z.boolean(),
  lastStartedAt: z.string().optional(),
  lastCompletedAt: z.string().optional(),
  lastSucceededAt: z.string().optional(),
  lastError: z.string().optional(),
  runCount: z.number().int(),
  failureCount: z.number().int(),
});
export type JobStatus = z.infer<typeof jobStatusSchema>;
export const deliverySchema = z.object({
  id: z.string(),
  organizationId: z.string(),
  notificationId: z.string(),
  userId: z.string(),
  channel: z.string(),
  status: z.string(),
  attempts: z.number().int(),
  lastError: z.string().optional(),
  nextAttemptAt: z.string().optional(),
  createdAt: z.string(),
  updatedAt: z.string(),
});
export type Delivery = z.infer<typeof deliverySchema>;
export const storageOverviewSchema = z.object({
  collections: z.number().int(),
  objects: z.number().int(),
  dataSizeBytes: z.number().int(),
  storageSizeBytes: z.number().int(),
  indexSizeBytes: z.number().int(),
});
export type StorageOverview = z.infer<typeof storageOverviewSchema>;

export type CreateLeadRequest = {
  name: string;
  email: string;
  company?: string;
  message?: string;
  source?: string;
  utmSource?: string;
  utmMedium?: string;
  utmCampaign?: string;
  scheduledFor?: string;
};

export type CreateCMSPageRequest = {
  slug: string;
  title: string;
  summary?: string;
  body: string;
  contentType?: "page" | "blog" | "faq" | "legal" | "settings";
  navLabel?: string;
  navOrder?: number;
};

export type UpdateCMSPageRequest = {
  title?: string;
  summary?: string;
  body?: string;
  contentType?: "page" | "blog" | "faq" | "legal" | "settings";
  navLabel?: string;
  navOrder?: number;
};

export type InviteMemberRequest = {
  email: string;
  displayName: string;
  password: string;
  roleCode?: string;
};

export type UpdateMemberRoleRequest = {
  roleCode: string;
};

export type CreateRoleRequest = {
  name: string;
  permissions: string[];
};

export type UpdateRoleRequest = {
  permissions: string[];
};

export type CreateFeatureFlagRequest = {
  key: string;
  description: string;
  enabled?: boolean;
  planCodes?: string[];
  rolloutPercentage?: number;
  cohortUserIds?: string[];
  expiresAt?: string;
};

export type UpdateFeatureFlagRequest = {
  description?: string;
  enabled?: boolean;
  planCodes?: string[];
  rolloutPercentage?: number;
  cohortUserIds?: string[];
  expiresAt?: string;
};

export type SetOrganizationFeatureFlagRequest = {
  enabled: boolean;
};

export type CreatePlanRequest = {
  code: string;
  name: string;
  description?: string;
  priceMonthlyCents: number;
  currency?: string;
  features?: string[];
  active?: boolean;
};

export type UpdatePlanRequest = {
  name?: string;
  description?: string;
  priceMonthlyCents?: number;
  currency?: string;
  features?: string[];
  active?: boolean;
};

export type SetOrganizationSubscriptionRequest = {
  planCode: string;
  status?: string;
};

export type CreateSupportTicketRequest = {
  subject: string;
  body: string;
  priority?: string;
  category?: BlockerCategory;
};

export type UpdateSupportTicketStatusRequest = {
  status: string;
  assigneeUserId?: string;
};

export type CreatePlatformStaffRequest = {
  email: string;
  displayName: string;
  roleCode: string;
};

export type UpdatePlatformStaffRoleRequest = {
  roleCode: string;
};

export type CreateOrgRequestRequest = {
  kind: "equipment" | "access";
  item: string;
  details?: string;
};

export type DecideOrgRequestRequest = {
  approve: boolean;
  note?: string;
};

export type LaunchPadClientOptions = {
  baseUrl: string;
};

export { createSessionStorage, type SessionStorage } from "./session";

/** Endpoints that must not trigger the refresh-and-retry flow on 401. */
const noRefreshRetryPaths = new Set([
  "/api/v1/auth/login",
  "/api/v1/auth/login/mfa",
  "/api/v1/auth/register",
  "/api/v1/auth/refresh",
  "/api/v1/auth/invitations/accept",
]);

/** Reads a non-HttpOnly cookie by name (the double-submit CSRF cookie). */
function readCookie(name: string): string | null {
  if (typeof document === "undefined") {
    return null;
  }

  const prefix = `${name}=`;
  for (const entry of document.cookie.split(";")) {
    const trimmed = entry.trim();
    if (trimmed.startsWith(prefix)) {
      return decodeURIComponent(trimmed.slice(prefix.length));
    }
  }

  return null;
}

type requestOptions = {
  /** Explicit bearer token for one-off calls (e.g. auth-callback validation). */
  token?: string;
  /** Set false to skip the single refresh-retry on 401. */
  refreshRetry?: boolean;
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

  // Single-flight refresh: concurrent 401s share one rotation request.
  let refreshFlight: Promise<void> | null = null;

  function refreshSession(): Promise<void> {
    refreshFlight ??= (async () => {
      const response = await fetch(`${baseUrl}/api/v1/auth/refresh`, {
        method: "POST",
        headers: {
          Accept: "application/json",
          "Content-Type": "application/json",
        },
        credentials: "include",
        body: JSON.stringify({}),
      });

      await parseEnvelope(response, tokenPairSchema);
    })().finally(() => {
      refreshFlight = null;
    });

    return refreshFlight;
  }

  async function sendWithRefresh(
    path: string,
    init: RequestInit,
    opts: requestOptions = {},
  ): Promise<Response> {
    const send = () => {
      const headers = new Headers(init.headers);
      headers.set("Accept", "application/json");

      if (init.body && !headers.has("Content-Type")) {
        headers.set("Content-Type", "application/json");
      }

      // Session tokens normally travel in HttpOnly cookies; an explicit
      // token is only used to validate a token handed over out-of-band
      // (e.g. the SSO signup callback) before a cookie session exists.
      if (opts.token) {
        headers.set("Authorization", `Bearer ${opts.token}`);
      }

      // Cookie-authenticated mutations need the double-submit CSRF token.
      if (init.method && init.method !== "GET") {
        const csrf = readCookie("lp_csrf");
        if (csrf) {
          headers.set("X-CSRF-Token", csrf);
        }
      }

      return fetch(`${baseUrl}${path}`, {
        ...init,
        headers,
        credentials: "include",
      });
    };

    let response = await send();

    if (
      response.status === 401 &&
      opts.refreshRetry !== false &&
      !opts.token &&
      !noRefreshRetryPaths.has(path)
    ) {
      try {
        await refreshSession();
        response = await send();
      } catch {
        // Refresh failed; surface the original 401 below.
      }
    }

    return response;
  }

  async function request<T>(
    path: string,
    init: RequestInit,
    dataSchema: z.ZodType<T>,
    opts: requestOptions = {},
  ): Promise<T> {
    return parseEnvelope(await sendWithRefresh(path, init, opts), dataSchema);
  }

  // For endpoints that answer 204 No Content on success (e.g. DELETE); error
  // responses still use the standard envelope.
  async function requestNoContent(
    path: string,
    init: RequestInit,
    opts: requestOptions = {},
  ): Promise<void> {
    const response = await sendWithRefresh(path, init, opts);

    if (response.ok) {
      return;
    }

    let code = "REQUEST_FAILED";
    let message = "Request failed";

    try {
      const body: unknown = await response.json();
      const parsed = envelopeSchema(z.unknown()).safeParse(body);
      if (parsed.success && parsed.data.error) {
        code = parsed.data.error.code;
        message = parsed.data.error.message;
      }
    } catch {
      // Non-JSON error body; keep the generic code/message.
    }

    throw new ApiError(response.status, code, message);
  }

  return {
    register(payload: RegisterRequest): Promise<AuthResult> {
      return request("/api/v1/auth/register", {
        method: "POST",
        body: JSON.stringify(payload),
      }, authResultSchema);
    },

    login(payload: LoginRequest): Promise<LoginResponse> {
      return request("/api/v1/auth/login", {
        method: "POST",
        body: JSON.stringify(payload),
      }, loginResponseSchema);
    },

    loginMFA(payload: MFALoginRequest): Promise<AuthResult> {
      return request("/api/v1/auth/login/mfa", {
        method: "POST",
        body: JSON.stringify(payload),
      }, authResultSchema);
    },

    mfaEnroll(): Promise<MFAEnrollResult> {
      return request("/api/v1/auth/mfa/enroll", {
        method: "POST",
        body: JSON.stringify({}),
      }, mfaEnrollResultSchema);
    },

    mfaConfirm(code: string): Promise<{ status: string }> {
      return request("/api/v1/auth/mfa/confirm", {
        method: "POST",
        body: JSON.stringify({ code }),
      }, z.object({ status: z.string() }));
    },

    mfaDisable(code: string): Promise<{ status: string }> {
      return request("/api/v1/auth/mfa/disable", {
        method: "POST",
        body: JSON.stringify({ code }),
      }, z.object({ status: z.string() }));
    },

    refresh(refreshToken?: string): Promise<TokenPair> {
      return request("/api/v1/auth/refresh", {
        method: "POST",
        body: JSON.stringify(refreshToken ? { refreshToken } : {}),
      }, tokenPairSchema);
    },

    me(): Promise<MeResponse> {
      return request("/api/v1/auth/me", { method: "GET" }, meSchema);
    },

    meWithToken(accessToken: string): Promise<MeResponse> {
      return request(
        "/api/v1/auth/me",
        { method: "GET" },
        meSchema,
        { token: accessToken, refreshRetry: false },
      );
    },

    listMyOrganizations(): Promise<OrganizationChoice[]> {
      return request("/api/v1/auth/organizations", { method: "GET" }, z.array(organizationChoiceSchema));
    },

    switchOrganization(organizationId: string): Promise<AuthResult> {
      return request("/api/v1/auth/switch-organization", {
        method: "POST",
        body: JSON.stringify({ organizationId }),
      }, authResultSchema);
    },

    getCurrentOrganization(): Promise<Organization> {
      return request("/api/v1/organizations/current", { method: "GET" }, organizationSchema);
    },

    updateCurrentOrganization(payload: UpdateOrganizationRequest): Promise<Organization> {
      return request("/api/v1/organizations/current", {
        method: "PATCH",
        body: JSON.stringify(payload),
      }, organizationSchema);
    },

    updateOrganizationSetup(step: number, completed = false): Promise<Organization> {
      return request("/api/v1/organizations/current/setup", {
        method: "PATCH",
        body: JSON.stringify({ step, completed }),
      }, organizationSchema);
    },

    getNotificationChannels(): Promise<ChannelStatus> {
      return request("/api/v1/notifications/channels", { method: "GET" }, channelStatusSchema);
    },

    setNotificationChannels(payload: SetChannelsRequest): Promise<ChannelStatus> {
      return request("/api/v1/notifications/channels", {
        method: "PUT",
        body: JSON.stringify(payload),
      }, channelStatusSchema);
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

    listMyContacts(): Promise<EmployeeContact[]> {
      return request("/api/v1/me/contacts", { method: "GET" }, z.array(employeeContactSchema));
    },

    createEmployee(payload: CreateEmployeeRequest): Promise<Employee> {
      return request("/api/v1/employees", {
        method: "POST",
        body: JSON.stringify(payload),
      }, employeeSchema);
    },

    importEmployeesCSV(csv: string): Promise<EmployeeImportResult> {
      return request("/api/v1/employees/import", {
        method: "POST",
        headers: { "Content-Type": "text/csv" },
        body: csv,
      }, employeeImportResultSchema);
    },

    listOrganizationInvitations(): Promise<Invitation[]> {
      return request(
        "/api/v1/organizations/current/invitations",
        { method: "GET" },
        z.array(invitationSchema),
      );
    },

    issueOrganizationInvitation(payload: {
      email: string;
      displayName: string;
      role?: string;
    }): Promise<void> {
      return request(
        "/api/v1/organizations/current/invitations",
        { method: "POST", body: JSON.stringify(payload) },
        z.object({ token: z.string() }),
      ).then(() => undefined);
    },

    resendOrganizationInvitation(invitationId: string): Promise<void> {
      return request(
        `/api/v1/organizations/current/invitations/${encodeURIComponent(invitationId)}/resend`,
        { method: "POST", body: JSON.stringify({}) },
        z.object({ token: z.string() }),
      ).then(() => undefined);
    },

    revokeOrganizationInvitation(invitationId: string): Promise<void> {
      return requestNoContent(
        `/api/v1/organizations/current/invitations/${encodeURIComponent(invitationId)}`,
        { method: "DELETE" },
      );
    },

    getEmployee(employeeId: string): Promise<Employee> {
      return request(`/api/v1/employees/${encodeURIComponent(employeeId)}`, { method: "GET" }, employeeSchema);
    },

    updateEmployee(employeeId: string, payload: UpdateEmployeeRequest): Promise<Employee> {
      return request(`/api/v1/employees/${encodeURIComponent(employeeId)}`, {
        method: "PATCH",
        body: JSON.stringify(payload),
      }, employeeSchema);
    },

    provisionEmployee(employeeId: string, payload: ProvisionEmployeeRequest): Promise<Employee> {
      return request(`/api/v1/employees/${encodeURIComponent(employeeId)}/provision`, {
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
      return request(`/api/v1/journeys/${encodeURIComponent(journeyId)}`, { method: "GET" }, journeyTemplateSchema);
    },

    listJourneySteps(journeyId: string): Promise<JourneyStep[]> {
      return request(
        `/api/v1/journeys/${encodeURIComponent(journeyId)}/steps`,
        { method: "GET" },
        z.array(journeyStepSchema),
      );
    },

    addJourneyStep(journeyId: string, payload: AddJourneyStepRequest): Promise<JourneyStep> {
      return request(`/api/v1/journeys/${encodeURIComponent(journeyId)}/steps`, {
        method: "POST",
        body: JSON.stringify(payload),
      }, journeyStepSchema);
    },

    publishJourney(journeyId: string): Promise<JourneyTemplate> {
      return request(
        `/api/v1/journeys/${encodeURIComponent(journeyId)}/publish`,
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

    assignJourneyToDepartment(payload: AssignDepartmentRequest): Promise<AssignDepartmentResult> {
      return request("/api/v1/assignments/department", {
        method: "POST",
        body: JSON.stringify(payload),
      }, assignDepartmentResultSchema);
    },

    getAssignment(assignmentId: string): Promise<JourneyAssignment> {
      return request(
        `/api/v1/assignments/${encodeURIComponent(assignmentId)}`,
        { method: "GET" },
        journeyAssignmentSchema,
      );
    },

    listAssignmentSteps(assignmentId: string, locale?: string): Promise<StepAssignment[]> {
      const query = locale ? `?locale=${encodeURIComponent(locale)}` : "";
      return request(
        `/api/v1/assignments/${encodeURIComponent(assignmentId)}/steps${query}`,
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

    listAssignmentRules(): Promise<AssignmentRule[]> {
      return request("/api/v1/assignment-rules", { method: "GET" }, z.array(assignmentRuleSchema));
    },

    createAssignmentRule(payload: CreateAssignmentRuleRequest): Promise<AssignmentRule> {
      return request("/api/v1/assignment-rules", {
        method: "POST",
        body: JSON.stringify(payload),
      }, assignmentRuleSchema);
    },

    updateAssignmentRule(ruleId: string, payload: UpdateAssignmentRuleRequest): Promise<AssignmentRule> {
      return request(`/api/v1/assignment-rules/${encodeURIComponent(ruleId)}`, {
        method: "PATCH",
        body: JSON.stringify(payload),
      }, assignmentRuleSchema);
    },

    deleteAssignmentRule(ruleId: string): Promise<void> {
      return request(
        `/api/v1/assignment-rules/${encodeURIComponent(ruleId)}`,
        { method: "DELETE" },
        z.unknown(),
      ).then(() => undefined);
    },

    runAssignmentRule(ruleId: string): Promise<RunAssignmentRuleResult> {
      return request(
        `/api/v1/assignment-rules/${encodeURIComponent(ruleId)}/run`,
        { method: "POST" },
        assignDepartmentResultSchema,
      );
    },

    completeStep(stepAssignmentId: string, payload: CompleteStepRequest = {}): Promise<StepAssignment> {
      return request(`/api/v1/step-assignments/${encodeURIComponent(stepAssignmentId)}/complete`, {
        method: "POST",
        body: JSON.stringify(payload),
      }, stepAssignmentSchema);
    },

    overrideStep(stepAssignmentId: string, payload: OverrideStepRequest): Promise<StepAssignment> {
      return request(`/api/v1/step-assignments/${encodeURIComponent(stepAssignmentId)}/override`, {
        method: "POST",
        body: JSON.stringify(payload),
      }, stepAssignmentSchema);
    },

    listApprovals(): Promise<Approval[]> {
      return request("/api/v1/approvals", { method: "GET" }, z.array(approvalSchema));
    },

    decideApproval(approvalId: string, payload: DecideApprovalRequest): Promise<Approval> {
      return request(`/api/v1/approvals/${encodeURIComponent(approvalId)}/decide`, {
        method: "POST",
        body: JSON.stringify(payload),
      }, approvalSchema);
    },

    reportBlocker(payload: ReportBlockerRequest): Promise<Blocker> {
      return request("/api/v1/me/blockers", {
        method: "POST",
        body: JSON.stringify(payload),
      }, blockerSchema);
    },

    getManagerTeam(): Promise<ManagerTeamReport[]> {
      return request(
        "/api/v1/manager/team",
        { method: "GET" },
        z.array(managerTeamReportSchema),
      );
    },

    listManagerBlockers(): Promise<Blocker[]> {
      return request(
        "/api/v1/manager/blockers",
        { method: "GET" },
        z.array(blockerSchema),
      );
    },

    listNotifications(): Promise<Notification[]> {
      return request("/api/v1/notifications", { method: "GET" }, z.array(notificationSchema));
    },

    markNotificationRead(notificationId: string): Promise<Notification> {
      return request(
        `/api/v1/notifications/${encodeURIComponent(notificationId)}/read`,
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

    listMembers(): Promise<Member[]> {
      return request(
        "/api/v1/organizations/current/members",
        { method: "GET" },
        z.array(memberSchema),
      );
    },

    updateMemberRole(
      userId: string,
      payload: UpdateMemberRoleRequest,
    ): Promise<MemberRoleUpdate> {
      return request(
        `/api/v1/organizations/current/members/${encodeURIComponent(userId)}`,
        {
          method: "PATCH",
          body: JSON.stringify(payload),
        },
        memberRoleUpdateSchema,
      );
    },

    listRoles(): Promise<Role[]> {
      return request(
        "/api/v1/organizations/current/roles",
        { method: "GET" },
        z.array(roleSchema),
      );
    },

    createRole(payload: CreateRoleRequest): Promise<Role> {
      return request("/api/v1/organizations/current/roles", {
        method: "POST",
        body: JSON.stringify(payload),
      }, roleSchema);
    },

    updateRole(roleId: string, payload: UpdateRoleRequest): Promise<Role> {
      return request(
        `/api/v1/organizations/current/roles/${encodeURIComponent(roleId)}`,
        {
          method: "PATCH",
          body: JSON.stringify(payload),
        },
        roleSchema,
      );
    },

    deleteRole(roleId: string): Promise<void> {
      return requestNoContent(
        `/api/v1/organizations/current/roles/${encodeURIComponent(roleId)}`,
        { method: "DELETE" },
      );
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

    getLaunchReadiness(): Promise<LaunchReadinessReport> {
      return request(
        "/api/v1/platform/launch-readiness",
        { method: "GET" },
        launchReadinessReportSchema,
      );
    },

    listPlatformAuditEvents(limit = 50): Promise<AuditEvent[]> {
      return request(
        `/api/v1/platform/audit-events?limit=${String(limit)}`,
        { method: "GET" },
        z.array(auditEventSchema),
      );
    },

    listPlatformOrganizations(filters?: {
      search?: string;
      status?: string;
      planCode?: string;
      offset?: number;
      limit?: number;
    }): Promise<OrganizationPage> {
      const params = new URLSearchParams();
      if (filters?.search) params.set("search", filters.search);
      if (filters?.status) params.set("status", filters.status);
      if (filters?.planCode) params.set("planCode", filters.planCode);
      if (filters?.offset !== undefined) params.set("offset", String(filters.offset));
      if (filters?.limit !== undefined) params.set("limit", String(filters.limit));
      const query = params.toString();
      return request(
        `/api/v1/platform/organizations${query ? `?${query}` : ""}`,
        { method: "GET" },
        organizationPageSchema,
      );
    },

    getPlatformOrganization(organizationId: string): Promise<Organization> {
      return request(
        `/api/v1/platform/organizations/${encodeURIComponent(organizationId)}`,
        { method: "GET" },
        organizationSchema,
      );
    },

    getPlatformOrganizationDetail(organizationId: string): Promise<PlatformOrganizationDetail> {
      return request(
        `/api/v1/platform/organizations/${encodeURIComponent(organizationId)}/details`,
        { method: "GET" },
        platformOrganizationDetailSchema,
      );
    },

    suspendOrganization(organizationId: string): Promise<Organization> {
      return request(
        `/api/v1/platform/organizations/${encodeURIComponent(organizationId)}/suspend`,
        { method: "POST" },
        organizationSchema,
      );
    },

    activateOrganization(organizationId: string): Promise<Organization> {
      return request(
        `/api/v1/platform/organizations/${encodeURIComponent(organizationId)}/activate`,
        { method: "POST" },
        organizationSchema,
      );
    },

    closeOrganization(organizationId: string): Promise<Organization> {
      return request(
        `/api/v1/platform/organizations/${encodeURIComponent(organizationId)}/close`,
        { method: "POST" },
        organizationSchema,
      );
    },

    startSupportSession(
      organizationId: string,
      reason: string,
      durationMinutes?: number,
    ): Promise<SupportSessionCreated> {
      return request(
        "/api/v1/platform/support-sessions",
        {
          method: "POST",
          body: JSON.stringify({ organizationId, reason, durationMinutes }),
        },
        supportSessionCreatedSchema,
      );
    },

    endSupportSession(sessionId: string, endReason?: string): Promise<SupportSession> {
      return request(
        `/api/v1/platform/support-sessions/${encodeURIComponent(sessionId)}/end`,
        { method: "POST", body: JSON.stringify(endReason ? { endReason } : {}) },
        supportSessionSchema,
      );
    },

    listSupportSessions(organizationId: string): Promise<SupportSession[]> {
      return request(
        `/api/v1/platform/support-sessions?organizationId=${encodeURIComponent(organizationId)}`,
        { method: "GET" },
        z.array(supportSessionSchema),
      );
    },

    listPlatformLeads(): Promise<Lead[]> {
      return request("/api/v1/platform/leads", { method: "GET" }, z.array(leadSchema));
    },

    listPlatformFeatureFlags(): Promise<FeatureFlag[]> {
      return request(
        "/api/v1/platform/feature-flags",
        { method: "GET" },
        z.array(featureFlagSchema),
      );
    },

    listMarketplaceTemplates(): Promise<MarketplaceTemplate[]> {
      return request("/api/v1/marketplace/templates", { method: "GET" }, z.array(marketplaceTemplateSchema));
    },

    listMyMarketplaceTemplates(): Promise<MarketplaceTemplate[]> {
      return request("/api/v1/marketplace/templates/mine", { method: "GET" }, z.array(marketplaceTemplateSchema));
    },

    listPlatformMarketplaceTemplates(): Promise<MarketplaceTemplate[]> {
      return request("/api/v1/platform/marketplace/templates", { method: "GET" }, z.array(marketplaceTemplateSchema));
    },

    createPlatformMarketplaceTemplate(payload: {
      name: string; description: string; category: string; steps: MarketplaceStep[];
    }): Promise<MarketplaceTemplate> {
      return request("/api/v1/platform/marketplace/templates", {
        method: "POST", body: JSON.stringify(payload),
      }, marketplaceTemplateSchema);
    },

    submitMarketplaceTemplate(payload: {
      name: string; description: string; category: string; steps: MarketplaceStep[];
      priceCents?: number; currency?: string;
    }): Promise<MarketplaceTemplate> {
      return request("/api/v1/marketplace/templates/submit", {
        method: "POST", body: JSON.stringify(payload),
      }, marketplaceTemplateSchema);
    },

    purchaseMarketplaceTemplate(id: string): Promise<MarketplaceCheckout> {
      return request(`/api/v1/marketplace/templates/${encodeURIComponent(id)}/purchase`, {
        method: "POST",
      }, marketplaceCheckoutSchema);
    },

    completeMarketplacePurchase(reference: string): Promise<MarketplaceInstallation> {
      return request("/api/v1/marketplace/purchases/complete", {
        method: "POST", body: JSON.stringify({ reference }),
      }, marketplaceInstallationSchema);
    },

    publishMarketplaceTemplate(id: string): Promise<MarketplaceTemplate> {
      return request(`/api/v1/platform/marketplace/templates/${encodeURIComponent(id)}/publish`, { method: "POST" }, marketplaceTemplateSchema);
    },

    removeMarketplaceTemplate(id: string): Promise<MarketplaceTemplate> {
      return request(`/api/v1/platform/marketplace/templates/${encodeURIComponent(id)}/remove`, { method: "POST" }, marketplaceTemplateSchema);
    },

    featureMarketplaceTemplate(id: string, featured: boolean): Promise<MarketplaceTemplate> {
      return request(`/api/v1/platform/marketplace/templates/${encodeURIComponent(id)}/featured`, {
        method: "PUT", body: JSON.stringify({ featured }),
      }, marketplaceTemplateSchema);
    },

    installMarketplaceTemplate(id: string): Promise<MarketplaceInstallation> {
      return request(`/api/v1/marketplace/templates/${encodeURIComponent(id)}/install`, { method: "POST" }, marketplaceInstallationSchema);
    },

    rateMarketplaceTemplate(id: string, score: number): Promise<MarketplaceTemplate> {
      return request(`/api/v1/marketplace/templates/${encodeURIComponent(id)}/rating`, {
        method: "PUT", body: JSON.stringify({ score }),
      }, marketplaceTemplateSchema);
    },

    createPlatformFeatureFlag(payload: CreateFeatureFlagRequest): Promise<FeatureFlag> {
      return request("/api/v1/platform/feature-flags", {
        method: "POST",
        body: JSON.stringify(payload),
      }, featureFlagSchema);
    },

    updatePlatformFeatureFlag(
      key: string,
      payload: UpdateFeatureFlagRequest,
    ): Promise<FeatureFlag> {
      return request(`/api/v1/platform/feature-flags/${encodeURIComponent(key)}`, {
        method: "PATCH",
        body: JSON.stringify(payload),
      }, featureFlagSchema);
    },

    listPlatformFeatureFlagHistory(key: string, limit = 50): Promise<FeatureFlagHistory[]> {
      return request(
        `/api/v1/platform/feature-flags/${encodeURIComponent(key)}/history?limit=${String(limit)}`,
        { method: "GET" },
        z.array(featureFlagHistorySchema),
      );
    },

    setOrganizationFeatureFlag(
      organizationId: string,
      key: string,
      payload: SetOrganizationFeatureFlagRequest,
    ): Promise<{ id: string; organizationId: string; key: string; enabled: boolean; updatedAt: string; updatedBy: string }> {
      return request(
        `/api/v1/platform/organizations/${encodeURIComponent(organizationId)}/feature-flags/${encodeURIComponent(key)}`,
        {
          method: "PUT",
          body: JSON.stringify(payload),
        },
        z.object({
          id: z.string(),
          organizationId: z.string(),
          key: z.string(),
          enabled: z.boolean(),
          updatedAt: z.string(),
          updatedBy: z.string(),
        }),
      );
    },

    listFeatureFlags(): Promise<OrgFeatureFlags> {
      return request("/api/v1/feature-flags", { method: "GET" }, orgFeatureFlagsSchema);
    },

    listPlatformPlans(): Promise<Plan[]> {
      return request("/api/v1/platform/plans", { method: "GET" }, z.array(planSchema));
    },

    createPlatformPlan(payload: CreatePlanRequest): Promise<Plan> {
      return request("/api/v1/platform/plans", {
        method: "POST",
        body: JSON.stringify(payload),
      }, planSchema);
    },

    updatePlatformPlan(code: string, payload: UpdatePlanRequest): Promise<Plan> {
      return request(`/api/v1/platform/plans/${encodeURIComponent(code)}`, {
        method: "PATCH",
        body: JSON.stringify(payload),
      }, planSchema);
    },

    listPlatformSubscriptions(): Promise<Subscription[]> {
      return request(
        "/api/v1/platform/subscriptions",
        { method: "GET" },
        z.array(subscriptionSchema),
      );
    },

    listPlatformInvoices(): Promise<Invoice[]> {
      return request("/api/v1/platform/invoices", { method: "GET" }, z.array(invoiceSchema));
    },
    listPlatformCoupons(): Promise<Coupon[]> {
      return request("/api/v1/platform/coupons", { method: "GET" }, z.array(couponSchema));
    },
    createPlatformCoupon(payload: {
      code: string;
      percentOffBasisPoints?: number;
      amountOffCents?: number;
      currency?: string;
      maxRedemptions?: number;
      expiresAt?: string;
    }): Promise<Coupon> {
      return request("/api/v1/platform/coupons", {
        method: "POST", body: JSON.stringify(payload),
      }, couponSchema);
    },

    adjustPlatformInvoice(invoiceId: string, payload: {
      taxRateBasisPoints: number;
      couponCode?: string;
    }): Promise<Invoice> {
      return request(`/api/v1/platform/invoices/${encodeURIComponent(invoiceId)}`, {
        method: "PATCH", body: JSON.stringify(payload),
      }, invoiceSchema);
    },

    refundPlatformInvoice(invoiceId: string, payload: {
      amountCents: number;
      reason: string;
    }): Promise<Invoice> {
      return request(`/api/v1/platform/invoices/${encodeURIComponent(invoiceId)}/refund`, {
        method: "POST", body: JSON.stringify(payload),
      }, invoiceSchema);
    },

    setOrganizationSubscription(
      organizationId: string,
      payload: SetOrganizationSubscriptionRequest,
    ): Promise<Subscription> {
      return request(
        `/api/v1/platform/organizations/${encodeURIComponent(organizationId)}/subscription`,
        {
          method: "POST",
          body: JSON.stringify(payload),
        },
        subscriptionSchema,
      );
    },

    listBillingPlans(): Promise<Plan[]> {
      return request("/api/v1/billing/plans", { method: "GET" }, z.array(planSchema));
    },

    getBillingSubscription(): Promise<Subscription> {
      return request("/api/v1/billing/subscription", { method: "GET" }, subscriptionSchema);
    },

    updateBillingSubscription(payload: SetOrganizationSubscriptionRequest): Promise<Subscription> {
      return request("/api/v1/billing/subscription", {
        method: "POST",
        body: JSON.stringify(payload),
      }, subscriptionSchema);
    },

    listSupportTickets(): Promise<SupportTicket[]> {
      return request("/api/v1/support/tickets", { method: "GET" }, z.array(supportTicketSchema));
    },

    createSupportTicket(payload: CreateSupportTicketRequest): Promise<SupportTicket> {
      return request("/api/v1/support/tickets", {
        method: "POST",
        body: JSON.stringify(payload),
      }, supportTicketSchema);
    },

    getSupportTicket(ticketId: string): Promise<SupportTicket> {
      return request(
        `/api/v1/support/tickets/${encodeURIComponent(ticketId)}`,
        { method: "GET" },
        supportTicketSchema,
      );
    },

    addSupportMessage(ticketId: string, body: string): Promise<SupportTicket> {
      return request(`/api/v1/support/tickets/${encodeURIComponent(ticketId)}/messages`, {
        method: "POST", body: JSON.stringify({ body }),
      }, supportTicketSchema);
    },

    listMyRequests(): Promise<OrgRequest[]> {
      return request("/api/v1/me/requests", { method: "GET" }, z.array(orgRequestSchema));
    },

    createMyRequest(payload: CreateOrgRequestRequest): Promise<OrgRequest> {
      return request("/api/v1/me/requests", {
        method: "POST",
        body: JSON.stringify(payload),
      }, orgRequestSchema);
    },

    cancelMyRequest(requestId: string): Promise<OrgRequest> {
      return request(`/api/v1/me/requests/${encodeURIComponent(requestId)}/cancel`, {
        method: "POST",
        body: JSON.stringify({}),
      }, orgRequestSchema);
    },

    listOrgRequests(status?: OrgRequestStatus): Promise<OrgRequest[]> {
      const query = status ? `?status=${encodeURIComponent(status)}` : "";
      return request(`/api/v1/requests${query}`, { method: "GET" }, z.array(orgRequestSchema));
    },

    decideOrgRequest(requestId: string, payload: DecideOrgRequestRequest): Promise<OrgRequest> {
      return request(`/api/v1/requests/${encodeURIComponent(requestId)}/decide`, {
        method: "POST",
        body: JSON.stringify(payload),
      }, orgRequestSchema);
    },

    fulfillOrgRequest(requestId: string): Promise<OrgRequest> {
      return request(`/api/v1/requests/${encodeURIComponent(requestId)}/fulfill`, {
        method: "POST",
        body: JSON.stringify({}),
      }, orgRequestSchema);
    },

    listMyMeetings(): Promise<Meeting[]> {
      return request("/api/v1/me/meetings", { method: "GET" }, z.array(meetingSchema));
    },

    cancelMyMeeting(meetingId: string): Promise<Meeting> {
      return request(`/api/v1/me/meetings/${encodeURIComponent(meetingId)}/cancel`, {
        method: "POST",
        body: JSON.stringify({}),
      }, meetingSchema);
    },

    listOrgMeetings(status?: MeetingStatus): Promise<Meeting[]> {
      const query = status ? `?status=${encodeURIComponent(status)}` : "";
      return request(`/api/v1/meetings${query}`, { method: "GET" }, z.array(meetingSchema));
    },

    createOrgMeeting(payload: CreateMeetingRequest): Promise<Meeting> {
      return request("/api/v1/meetings", {
        method: "POST",
        body: JSON.stringify(payload),
      }, meetingSchema);
    },

    completeOrgMeeting(meetingId: string, payload: CompleteMeetingRequest): Promise<Meeting> {
      return request(`/api/v1/meetings/${encodeURIComponent(meetingId)}/complete`, {
        method: "POST",
        body: JSON.stringify(payload),
      }, meetingSchema);
    },

    cancelOrgMeeting(meetingId: string): Promise<Meeting> {
      return request(`/api/v1/meetings/${encodeURIComponent(meetingId)}/cancel`, {
        method: "POST",
        body: JSON.stringify({}),
      }, meetingSchema);
    },

    rescheduleOrgMeeting(meetingId: string, payload: RescheduleMeetingRequest): Promise<Meeting> {
      return request(`/api/v1/meetings/${encodeURIComponent(meetingId)}`, {
        method: "PATCH",
        body: JSON.stringify(payload),
      }, meetingSchema);
    },

    getCalendarConnection(provider: "google" | "microsoft" = "google"): Promise<CalendarConnection> {
      return request(`/api/v1/calendar/connection?provider=${provider}`, { method: "GET" }, calendarConnectionSchema);
    },

    connectCalendar(token: string, provider: "google" | "microsoft" = "google"): Promise<CalendarConnection> {
      return request("/api/v1/calendar/connection", {
        method: "PUT",
        body: JSON.stringify({ token, provider }),
      }, calendarConnectionSchema);
    },

    disconnectCalendar(provider: "google" | "microsoft" = "google"): Promise<void> {
      return requestNoContent(`/api/v1/calendar/connection?provider=${provider}`, { method: "DELETE" });
    },

    startCalendarOAuth(provider: "google" | "microsoft"): Promise<string> {
      return request(
        `/api/v1/calendar/oauth/${provider}/start`,
        { method: "GET" },
        z.object({ authorizationUrl: z.string().url() }),
      ).then((result) => result.authorizationUrl);
    },

    requestPasswordReset(email: string): Promise<void> {
      return request(
        "/api/v1/auth/password-reset/request",
        { method: "POST", body: JSON.stringify({ email }) },
        z.object({ status: z.literal("reset_requested") }),
      ).then(() => undefined);
    },

    confirmPasswordReset(token: string, newPassword: string): Promise<void> {
      return request(
        "/api/v1/auth/password-reset/confirm",
        { method: "POST", body: JSON.stringify({ token, newPassword }) },
        z.object({ status: z.literal("password_reset") }),
      ).then(() => undefined);
    },

    getSAMLConfig(): Promise<SAMLConfig> {
      return request("/api/v1/organizations/current/saml", { method: "GET" }, samlConfigSchema);
    },

    setSAMLConfig(payload: {
      enabled: boolean;
      idpMetadataXml: string;
      emailAttribute?: string;
    }): Promise<SAMLConfig> {
      return request("/api/v1/organizations/current/saml", {
        method: "PUT",
        body: JSON.stringify(payload),
      }, samlConfigSchema);
    },

    listKnowledgeDocuments(): Promise<KnowledgeDocument[]> {
      return request(
        "/api/v1/knowledge/documents",
        { method: "GET" },
        z.array(knowledgeDocumentSchema),
      );
    },

    createKnowledgeDocument(payload: CreateKnowledgeDocumentRequest): Promise<KnowledgeDocument> {
      return request("/api/v1/knowledge/documents", {
        method: "POST",
        body: JSON.stringify(payload),
      }, knowledgeDocumentSchema);
    },

    getKnowledgeDocument(documentId: string): Promise<KnowledgeDocument> {
      return request(
        `/api/v1/knowledge/documents/${encodeURIComponent(documentId)}`,
        { method: "GET" },
        knowledgeDocumentSchema,
      );
    },

    updateKnowledgeDocument(
      documentId: string,
      payload: UpdateKnowledgeDocumentRequest,
    ): Promise<KnowledgeDocument> {
      return request(`/api/v1/knowledge/documents/${encodeURIComponent(documentId)}`, {
        method: "PATCH",
        body: JSON.stringify(payload),
      }, knowledgeDocumentSchema);
    },

    listKnowledgeDocumentVersions(documentId: string): Promise<KnowledgeVersion[]> {
      return request(
        `/api/v1/knowledge/documents/${encodeURIComponent(documentId)}/history`,
        { method: "GET" },
        z.array(knowledgeVersionSchema),
      );
    },

    createKnowledgeDocumentVersion(documentId: string): Promise<KnowledgeDocument> {
      return request(
        `/api/v1/knowledge/documents/${encodeURIComponent(documentId)}/versions`,
        { method: "POST" },
        knowledgeDocumentSchema,
      );
    },

    syncKnowledgeDocument(documentId: string): Promise<KnowledgeDocument> {
      return request(
        `/api/v1/knowledge/documents/${encodeURIComponent(documentId)}/sync`,
        { method: "POST" },
        knowledgeDocumentSchema,
      );
    },

    approveKnowledgeDocument(documentId: string): Promise<KnowledgeDocument> {
      return request(
        `/api/v1/knowledge/documents/${encodeURIComponent(documentId)}/approve`,
        { method: "POST" },
        knowledgeDocumentSchema,
      );
    },

    indexKnowledgeDocument(documentId: string): Promise<KnowledgeDocument> {
      return request(
        `/api/v1/knowledge/documents/${encodeURIComponent(documentId)}/index`,
        { method: "POST" },
        knowledgeDocumentSchema,
      );
    },

    archiveKnowledgeDocument(documentId: string): Promise<KnowledgeDocument> {
      return request(
        `/api/v1/knowledge/documents/${encodeURIComponent(documentId)}/archive`,
        { method: "POST" },
        knowledgeDocumentSchema,
      );
    },

    listAssessments(): Promise<Assessment[]> {
      return request("/api/v1/assessments", { method: "GET" }, z.array(assessmentSchema));
    },

    createAssessment(payload: CreateAssessmentRequest): Promise<Assessment> {
      return request("/api/v1/assessments", {
        method: "POST",
        body: JSON.stringify(payload),
      }, assessmentSchema);
    },

    getAssessment(assessmentId: string): Promise<Assessment> {
      return request(
        `/api/v1/assessments/${encodeURIComponent(assessmentId)}`,
        { method: "GET" },
        assessmentSchema,
      );
    },

    updateAssessment(assessmentId: string, payload: UpdateAssessmentRequest): Promise<Assessment> {
      return request(`/api/v1/assessments/${encodeURIComponent(assessmentId)}`, {
        method: "PATCH",
        body: JSON.stringify(payload),
      }, assessmentSchema);
    },

    publishAssessment(assessmentId: string): Promise<Assessment> {
      return request(
        `/api/v1/assessments/${encodeURIComponent(assessmentId)}/publish`,
        { method: "POST" },
        assessmentSchema,
      );
    },

    archiveAssessment(assessmentId: string): Promise<Assessment> {
      return request(
        `/api/v1/assessments/${encodeURIComponent(assessmentId)}/archive`,
        { method: "POST" },
        assessmentSchema,
      );
    },

    takeAssessment(assessmentId: string): Promise<AssessmentTakeView> {
      return request(
        `/api/v1/assessments/${encodeURIComponent(assessmentId)}/take`,
        { method: "GET" },
        assessmentTakeViewSchema,
      );
    },

    submitAssessmentAttempt(
      assessmentId: string,
      payload: SubmitAssessmentAttemptRequest,
    ): Promise<AssessmentAttempt> {
      return request(`/api/v1/assessments/${encodeURIComponent(assessmentId)}/attempts`, {
        method: "POST",
        body: JSON.stringify(payload),
      }, assessmentAttemptSchema);
    },

    listAssessmentAttempts(assessmentId: string): Promise<AssessmentAttempt[]> {
      return request(
        `/api/v1/assessments/${encodeURIComponent(assessmentId)}/attempts`,
        { method: "GET" },
        z.array(assessmentAttemptSchema),
      );
    },

    reviewAssessmentAttempt(
      assessmentId: string,
      attemptId: string,
      payload: ReviewAssessmentAttemptRequest,
    ): Promise<AssessmentAttempt> {
      return request(
        `/api/v1/assessments/${encodeURIComponent(assessmentId)}/attempts/${encodeURIComponent(attemptId)}/review`,
        {
          method: "POST",
          body: JSON.stringify(payload),
        },
        assessmentAttemptSchema,
      );
    },

    listMyCertificates(): Promise<Certificate[]> {
      return request("/api/v1/me/certificates", { method: "GET" }, z.array(certificateSchema));
    },

    listPlatformSupportTickets(): Promise<SupportTicket[]> {
      return request(
        "/api/v1/platform/support/tickets",
        { method: "GET" },
        z.array(supportTicketSchema),
      );
    },

    getPlatformSupportSummary(): Promise<SupportSummary> {
      return request("/api/v1/platform/support/summary", { method: "GET" }, supportSummarySchema);
    },

    addPlatformSupportMessage(ticketId: string, body: string, internal = false): Promise<SupportTicket> {
      return request(`/api/v1/platform/support/tickets/${encodeURIComponent(ticketId)}/messages`, {
        method: "POST", body: JSON.stringify({ body, internal }),
      }, supportTicketSchema);
    },

    escalatePlatformSupportTicket(ticketId: string, assigneeUserId?: string): Promise<SupportTicket> {
      return request(`/api/v1/platform/support/tickets/${encodeURIComponent(ticketId)}/escalate`, {
        method: "POST", body: JSON.stringify({ assigneeUserId: assigneeUserId ?? "" }),
      }, supportTicketSchema);
    },

    getPlatformSupportTicket(ticketId: string): Promise<SupportTicket> {
      return request(
        `/api/v1/platform/support/tickets/${encodeURIComponent(ticketId)}`,
        { method: "GET" },
        supportTicketSchema,
      );
    },

    updatePlatformSupportTicketStatus(
      ticketId: string,
      payload: UpdateSupportTicketStatusRequest,
    ): Promise<SupportTicket> {
      return request(
        `/api/v1/platform/support/tickets/${encodeURIComponent(ticketId)}/status`,
        {
          method: "POST",
          body: JSON.stringify(payload),
        },
        supportTicketSchema,
      );
    },

    listPlatformStaff(): Promise<PlatformStaffMember[]> {
      return request(
        "/api/v1/platform/staff",
        { method: "GET" },
        z.array(platformStaffMemberSchema),
      );
    },

    createPlatformStaff(payload: CreatePlatformStaffRequest): Promise<CreatePlatformStaffResult> {
      return request(
        "/api/v1/platform/staff",
        { method: "POST", body: JSON.stringify(payload) },
        createPlatformStaffResultSchema,
      );
    },

    updatePlatformStaffRole(
      staffId: string,
      payload: UpdatePlatformStaffRoleRequest,
    ): Promise<PlatformStaffMember> {
      return request(
        `/api/v1/platform/staff/${encodeURIComponent(staffId)}`,
        { method: "PATCH", body: JSON.stringify(payload) },
        platformStaffMemberSchema,
      );
    },

    deactivatePlatformStaff(staffId: string): Promise<PlatformStaffMember> {
      return request(
        `/api/v1/platform/staff/${encodeURIComponent(staffId)}/deactivate`,
        { method: "POST" },
        platformStaffMemberSchema,
      );
    },

    reactivatePlatformStaff(staffId: string): Promise<PlatformStaffMember> {
      return request(
        `/api/v1/platform/staff/${encodeURIComponent(staffId)}/reactivate`,
        { method: "POST" },
        platformStaffMemberSchema,
      );
    },

    getPlatformAccessReview(): Promise<AccessReviewItem[]> {
      return request("/api/v1/platform/security/access-review", { method: "GET" }, z.array(accessReviewItemSchema));
    },

    attestPlatformAccess(staffId: string): Promise<PlatformStaffMember> {
      return request(`/api/v1/platform/security/access-review/${encodeURIComponent(staffId)}/attest`, {
        method: "POST",
      }, platformStaffMemberSchema);
    },

    grantPlatformBreakGlass(staffId: string, reason: string, durationMinutes: number): Promise<PlatformStaffMember> {
      return request(`/api/v1/platform/security/break-glass/${encodeURIComponent(staffId)}`, {
        method: "POST", body: JSON.stringify({ reason, durationMinutes }),
      }, platformStaffMemberSchema);
    },

    revokePlatformBreakGlass(staffId: string): Promise<PlatformStaffMember> {
      return request(`/api/v1/platform/security/break-glass/${encodeURIComponent(staffId)}/revoke`, {
        method: "POST",
      }, platformStaffMemberSchema);
    },

    getOnboardingAnalytics(): Promise<OnboardingSummary> {
      return request(
        "/api/v1/analytics/onboarding",
        { method: "GET" },
        onboardingSummarySchema,
      );
    },

    getPublishedCMSPage(slug: string): Promise<CMSPage> {
      return request(
        `/api/v1/cms/pages/${encodeURIComponent(slug)}`,
        { method: "GET" },
        cmsPageSchema,
      );
    },

    getCMSNavigation(): Promise<CMSPage[]> {
      return request("/api/v1/cms/navigation", { method: "GET" }, z.array(cmsPageSchema));
    },

    listPlatformCMSPages(): Promise<CMSPage[]> {
      return request("/api/v1/platform/cms/pages", { method: "GET" }, z.array(cmsPageSchema));
    },

    listPlatformJobs(): Promise<JobStatus[]> {
      return request("/api/v1/platform/jobs", { method: "GET" }, z.array(jobStatusSchema));
    },

    runPlatformJob(name: string): Promise<JobStatus[]> {
      return request(`/api/v1/platform/jobs/${encodeURIComponent(name)}/run`, {
        method: "POST",
      }, z.array(jobStatusSchema));
    },

    listPlatformDeliveries(): Promise<Delivery[]> {
      return request("/api/v1/platform/deliveries", { method: "GET" }, z.array(deliverySchema));
    },

    getPlatformStorageOverview(): Promise<StorageOverview> {
      return request("/api/v1/platform/storage", { method: "GET" }, storageOverviewSchema);
    },

    retryPlatformDelivery(id: string): Promise<Delivery> {
      return request(`/api/v1/platform/deliveries/${encodeURIComponent(id)}/retry`, {
        method: "POST",
      }, deliverySchema);
    },

    createPlatformCMSPage(payload: CreateCMSPageRequest): Promise<CMSPage> {
      return request(
        "/api/v1/platform/cms/pages",
        {
          method: "POST",
          body: JSON.stringify(payload),
        },
        cmsPageSchema,
      );
    },

    getPlatformCMSPage(pageId: string): Promise<CMSPage> {
      return request(
        `/api/v1/platform/cms/pages/${encodeURIComponent(pageId)}`,
        { method: "GET" },
        cmsPageSchema,
      );
    },

    updatePlatformCMSPage(pageId: string, payload: UpdateCMSPageRequest): Promise<CMSPage> {
      return request(
        `/api/v1/platform/cms/pages/${encodeURIComponent(pageId)}`,
        {
          method: "PATCH",
          body: JSON.stringify(payload),
        },
        cmsPageSchema,
      );
    },

    publishPlatformCMSPage(pageId: string): Promise<CMSPage> {
      return request(
        `/api/v1/platform/cms/pages/${encodeURIComponent(pageId)}/publish`,
        { method: "POST" },
        cmsPageSchema,
      );
    },

    schedulePlatformCMSPage(pageId: string, publishAt: string): Promise<CMSPage> {
      return request(
        `/api/v1/platform/cms/pages/${encodeURIComponent(pageId)}/schedule`,
        { method: "POST", body: JSON.stringify({ publishAt }) },
        cmsPageSchema,
      );
    },

    askAssistant(question: string): Promise<AssistantAnswer> {
      return request("/api/v1/assistant/ask", {
        method: "POST",
        body: JSON.stringify({ question }),
      }, assistantAnswerSchema);
    },

    submitAssistantFeedback(
      interactionId: string,
      payload: AssistantFeedbackRequest,
    ): Promise<void> {
      return requestNoContent(
        `/api/v1/assistant/interactions/${encodeURIComponent(interactionId)}/feedback`,
        {
          method: "POST",
          body: JSON.stringify(payload),
        },
      );
    },

    listIntegrations(): Promise<IntegrationConnection[]> {
      return request(
        "/api/v1/integrations",
        { method: "GET" },
        z.array(integrationConnectionSchema),
      );
    },

    connectIntegration(
      provider: IntegrationProvider,
      payload: IntegrationConnectRequest,
    ): Promise<IntegrationConnection> {
      return request(
        `/api/v1/integrations/${encodeURIComponent(provider)}/connect`,
        {
          method: "POST",
          body: JSON.stringify(payload),
        },
        integrationConnectionSchema,
      );
    },

    disconnectIntegration(provider: IntegrationProvider): Promise<void> {
      return requestNoContent(
        `/api/v1/integrations/${encodeURIComponent(provider)}/connect`,
        { method: "DELETE" },
      );
    },

    checkIntegrationHealth(provider: IntegrationProvider): Promise<IntegrationConnection> {
      return request(
        `/api/v1/integrations/${encodeURIComponent(provider)}/health`,
        { method: "POST" },
        integrationConnectionSchema,
      );
    },

    cloneJourney(journeyId: string): Promise<JourneyTemplate> {
      return request(
        `/api/v1/journeys/${journeyId}/clone`,
        { method: "POST" },
        journeyTemplateSchema,
      );
    },
    createJourneyVersion(journeyId: string): Promise<JourneyTemplate> {
      return request(
        `/api/v1/journeys/${journeyId}/versions`,
        { method: "POST" },
        journeyTemplateSchema,
      );
    },
    deleteJourneyStep(journeyId: string, stepId: string): Promise<void> {
      return request(
        `/api/v1/journeys/${journeyId}/steps/${stepId}`,
        { method: "DELETE" },
        z.unknown(),
      ).then(() => undefined);
    },
    getAssistantReport(): Promise<AssistantReport> {
      return request(
        "/api/v1/analytics/assistant",
        { method: "GET" },
        assistantReportSchema,
      );
    },
    getOnboardingBreakdown(by: OnboardingBreakdownGroupBy): Promise<OnboardingBreakdown> {
      return request(
        `/api/v1/analytics/onboarding/breakdown?by=${encodeURIComponent(by)}`,
        { method: "GET" },
        onboardingBreakdownSchema,
      );
    },

    getOnboardingFunnel(): Promise<FunnelReport> {
      return request(
        "/api/v1/analytics/onboarding/funnel",
        { method: "GET" },
        funnelReportSchema,
      );
    },
    listJourneyVersions(journeyId: string): Promise<JourneyVersionSummary[]> {
      return request(
        `/api/v1/journeys/${journeyId}/versions`,
        { method: "GET" },
        z.array(journeyVersionSummarySchema),
      );
    },
    rollbackJourney(journeyId: string, version: number): Promise<JourneyTemplate> {
      return request(
        `/api/v1/journeys/${journeyId}/rollback`,
        { method: "POST", body: JSON.stringify({ version }) },
        journeyTemplateSchema,
      );
    },
    startStep(stepAssignmentId: string): Promise<StepAssignment> {
      return request(`/api/v1/step-assignments/${stepAssignmentId}/start`, {
        method: "POST",
      }, stepAssignmentSchema);
    },
    submitStep(stepAssignmentId: string, payload: SubmitStepRequest): Promise<StepAssignment> {
      return request(`/api/v1/step-assignments/${stepAssignmentId}/submit`, {
        method: "POST",
        body: JSON.stringify(payload),
      }, stepAssignmentSchema);
    }
  };
}

export type LaunchPadClient = ReturnType<typeof createLaunchPadClient>;
