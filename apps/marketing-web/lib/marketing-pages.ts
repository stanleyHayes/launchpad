export type MarketingPage = {
  slug: string;
  title: string;
  eyebrow: string;
  description: string;
  outcomes: string[];
};

export const featurePages: MarketingPage[] = ([
  ["onboarding-journeys", "Onboarding journeys", "Coordinate every milestone from offer to first win.", ["Reusable role-based journeys", "Clear employee progress", "Versioned templates"]],
  ["workflow-builder", "Workflow builder", "Turn operating knowledge into structured, accountable workflows.", ["Twenty step types", "Approvals and prerequisites", "Due dates and reminders"]],
  ["learning-assessments", "Learning and assessments", "Prove understanding with scored checkpoints and certificates.", ["Reusable assessments", "Randomized questions", "Attempt history and review"]],
  ["knowledge-assistant", "Employee knowledge assistant", "Give cited answers from approved company knowledge.", ["Grounded responses", "Source citations", "Human escalation"]],
  ["it-access-setup", "IT and access setup", "Route equipment and system access before day one.", ["Request workflows", "Approval queues", "Fulfilment tracking"]],
  ["manager-dashboard", "Manager dashboard", "Put team progress, blockers, and approvals in one view.", ["Direct-report rollups", "Overdue visibility", "Blocker triage"]],
  ["analytics-reporting", "Analytics and reporting", "Measure completion, ramp time, and onboarding risk.", ["Department breakdowns", "Overdue rates", "Assistant insights"]],
  ["security-compliance", "Security and compliance", "Keep access controlled and every privileged action traceable.", ["RBAC and MFA", "Immutable audit trail", "SSO and SCIM"]],
  ["integrations", "Integrations", "Connect the systems that already run your employee lifecycle.", ["HRIS synchronization", "Slack and Teams", "GitHub and Jira"]],
  ["templates-marketplace", "Templates marketplace", "Start with proven onboarding blueprints and adapt them safely.", ["Official templates", "Versioned installation", "Ratings and categories"]],
] satisfies Array<[string, string, string, string[]]>).map(([slug, title, description, outcomes]) => ({ slug, title, eyebrow: "Feature", description, outcomes }));

export const solutionPages: MarketingPage[] = ([
  ["hr-teams", "HR teams", "Run consistent onboarding without chasing every stakeholder.", ["Automated assignments", "Employee directory", "Completion reporting"]],
  ["it-teams", "IT teams", "See, approve, and fulfil access and equipment requests on time.", ["Access queues", "Equipment tracking", "Audit evidence"]],
  ["security-teams", "Security teams", "Make identity, policy, and access controls part of every journey.", ["SSO and SCIM", "Policy attestations", "Privileged audit"]],
  ["engineering-teams", "Engineering teams", "Ramp developers through systems, architecture, and first delivery.", ["Role-specific journeys", "Coding checkpoints", "Manager reviews"]],
  ["sales-teams", "Sales teams", "Move new sellers from product learning to confident customer conversations.", ["Certification paths", "Knowledge checks", "Ramp analytics"]],
  ["customer-support-teams", "Customer-support teams", "Teach product, policy, and escalation practices with measurable readiness.", ["Scenario assessments", "Knowledge assistant", "Coaching touchpoints"]],
  ["remote-companies", "Remote companies", "Create belonging and clarity when onboarding happens across time zones.", ["Async journeys", "Scheduled touchpoints", "Buddy workflows"]],
  ["startups", "Startups", "Replace founder-led onboarding with a repeatable system that scales.", ["Fast templates", "Simple automation", "Visible ownership"]],
  ["enterprises", "Enterprises", "Coordinate complex onboarding across business units and identity systems.", ["Tenant controls", "Custom roles", "Enterprise integrations"]],
  ["regulated-industries", "Regulated industries", "Build evidence, approval, and policy controls into employee readiness.", ["Audit exports", "Certification records", "Controlled knowledge"]],
] satisfies Array<[string, string, string, string[]]>).map(([slug, title, description, outcomes]) => ({ slug, title, eyebrow: "Solution", description, outcomes }));

export function findMarketingPage(items: MarketingPage[], slug: string) {
  return items.find((item) => item.slug === slug);
}
