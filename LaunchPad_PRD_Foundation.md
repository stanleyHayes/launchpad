# LaunchPad — Enterprise Employee Onboarding Platform

> **Version:** 0.3  
> **Status:** Build-ready foundation  
> **Audience:** Founders, product managers, designers, engineers, QA engineers, DevOps engineers, security teams, and AI coding agents  
> **Primary objective:** Define a product and implementation structure that can be followed from discovery through production deployment.  
> **Stack decision (v0.3):** Go modular monolith, MongoDB primary datastore, Redis for cache/sessions/jobs, custom Go authentication (email/password + sessions/JWT). SSO/SCIM remain Phase 3.

---

# 1. Executive Summary

LaunchPad is a multi-tenant SaaS platform that helps organizations onboard new employees through structured, measurable, role-specific journeys.

The product replaces fragmented onboarding based on meetings, documents, chats, and tribal knowledge with a guided system containing:

- pre-boarding workflows;
- account and equipment setup;
- company orientation;
- department and role training;
- practical assignments and simulations;
- meetings and buddy sessions;
- approvals and assessments;
- AI-assisted company knowledge search;
- analytics measuring onboarding quality and time-to-productivity.

LaunchPad consists of four major product surfaces:

1. **Public marketing website** for acquiring customers and explaining the product.
2. **Platform administration website** for LaunchPad's internal operations team.
3. **Organization administration portal** for customer HR, IT, managers, security, and executives.
4. **Employee onboarding application** for employees completing assigned journeys.

---

# 2. Product Vision

## 2.1 Vision Statement

Enable a new employee to understand the company, access the right systems, meet the right people, and contribute meaningfully within their first week.

## 2.2 Mission

- Reduce employee time-to-productivity.
- Make onboarding consistent across departments and locations.
- Preserve institutional knowledge.
- Automate repetitive HR and IT work.
- Detect blockers before employees become disengaged.
- Give organizations measurable evidence that onboarding is effective.

## 2.3 Product Positioning

LaunchPad is not only an LMS. It combines:

- onboarding workflow orchestration;
- learning management;
- identity and access coordination;
- company knowledge management;
- task and assessment management;
- manager visibility;
- employee experience analytics;
- AI company assistance.

---

# 3. Product Scope

## 3.1 In Scope

- SaaS multi-tenancy.
- Organization and employee management.
- Role-based onboarding journeys.
- Drag-and-drop workflow builder.
- Learning content and assessments.
- Meetings, approvals, and practical tasks.
- Email, in-app, Slack, and Teams notifications.
- AI knowledge assistant using approved organization sources.
- Integrations with HRIS, identity providers, calendars, communication platforms, source control, and project-management tools.
- Public website, pricing, lead capture, demo requests, and customer signup.
- Platform administration, billing operations, support, tenant monitoring, and feature controls.
- Organization analytics and platform analytics.

## 3.2 Out of Scope for MVP

- Full payroll processing.
- Full recruitment applicant-tracking system.
- Full performance-management suite.
- Native device-management replacement.
- Replacement for Okta, Microsoft Entra, Google Workspace, or HRIS products.
- Fully autonomous access provisioning without organization approval.

---

# 4. User Types and Personas

## 4.1 Public Visitor

A potential buyer researching employee onboarding software.

Primary goals:

- understand what LaunchPad does;
- view solutions and integrations;
- compare pricing;
- request a demo;
- create a trial account.

## 4.2 LaunchPad Platform Super Administrator

An internal LaunchPad employee who manages the SaaS platform.

Primary goals:

- manage organizations and subscriptions;
- investigate tenant issues;
- configure global features;
- view platform usage and health;
- manage content on the marketing website;
- manage support and security incidents.

## 4.3 Organization Owner

The primary customer account owner.

Primary goals:

- configure the organization;
- assign administrators;
- manage subscription and billing;
- oversee onboarding performance.

## 4.4 HR Administrator

Creates onboarding templates and assigns employees.

## 4.5 IT Administrator

Manages equipment, system-access tasks, SSO, and application integrations.

## 4.6 Security and Compliance Administrator

Manages mandatory training, certifications, policy acknowledgements, and audit exports.

## 4.7 Manager or Team Lead

Reviews employee progress, approves tasks, schedules sessions, and provides feedback.

## 4.8 Buddy or Mentor

Supports one or more employees through guided check-ins.

## 4.9 New Employee

Completes the onboarding journey, asks questions, submits tasks, attends meetings, and receives feedback.

## 4.10 Executive Viewer

Views high-level onboarding outcomes, risks, and departmental comparisons.

---

# 5. Major Product Surfaces

# 5.1 Public Marketing Website

The marketing website is a public, SEO-friendly application used to acquire customers, educate prospects, and convert visitors into demo requests or trial accounts.

## 5.1.1 Core Pages

### Home Page

Required sections:

- hero statement and primary call-to-action;
- product demonstration video or interactive preview;
- key customer problems;
- product capabilities;
- seven-day onboarding journey example;
- customer logos and testimonials;
- integrations preview;
- security and compliance summary;
- pricing preview;
- final demo and trial call-to-action.

### Product Overview

Explain the complete platform with links to feature pages.

### Feature Pages

Create dedicated pages for:

- onboarding journeys;
- workflow builder;
- learning and assessments;
- employee knowledge assistant;
- IT and access setup;
- manager dashboard;
- analytics and reporting;
- security and compliance;
- integrations;
- templates marketplace.

### Solutions Pages

Pages targeted at specific departments and customer types:

- HR teams;
- IT teams;
- security teams;
- engineering teams;
- sales teams;
- customer-support teams;
- remote companies;
- startups;
- enterprises;
- regulated industries.

### Pricing Page

Display plans and included capabilities.

Suggested plans:

- Starter;
- Growth;
- Enterprise.

Pricing requirements:

- monthly and annual toggle;
- price per active employee or onboarding seat;
- feature comparison;
- FAQ;
- contact-sales option for enterprise;
- transparent trial limitations.

### Integrations Directory

Display supported and planned integrations.

Each integration page includes:

- logo and description;
- capabilities;
- setup requirements;
- permissions requested;
- installation guide;
- data synced;
- support status.

### Templates Marketplace Preview

Public preview of onboarding templates.

Examples:

- software engineer;
- sales representative;
- customer-support agent;
- finance analyst;
- remote employee;
- manager onboarding;
- security compliance onboarding.

### Resources

- blog;
- onboarding guides;
- downloadable checklists;
- customer stories;
- webinars;
- API documentation link;
- help-center link.

### Company Pages

- about;
- careers;
- contact;
- partners;
- security;
- privacy policy;
- terms of service;
- cookie policy;
- data-processing agreement.

## 5.1.2 Marketing Website Features

- SEO metadata per page.
- Open Graph and social previews.
- Schema.org structured data.
- sitemap and robots configuration.
- multi-language-ready content model.
- CMS-managed page sections.
- lead capture forms.
- demo scheduling integration.
- newsletter subscription.
- gated resource downloads.
- referral and campaign tracking.
- UTM capture and attribution.
- cookie-consent management.
- A/B testing support.
- live chat or support widget.
- product-status link.
- accessibility compliance.
- analytics events for key conversions.

## 5.1.3 Lead Management Flow

1. Visitor submits a demo form.
2. System validates data and consent.
3. Lead is stored in LaunchPad CRM tables.
4. Lead is optionally synced to HubSpot or Salesforce.
5. Sales team receives notification.
6. Confirmation email is sent.
7. Lead receives scheduling link.
8. Conversion source and campaign are recorded.

## 5.1.4 Marketing Content Administration

Platform staff can manage:

- navigation menus;
- landing pages;
- blog posts;
- authors and categories;
- customer stories;
- FAQs;
- pricing content;
- integration listings;
- homepage banners;
- legal-document versions;
- SEO titles and descriptions;
- redirects;
- publication scheduling.

---

# 5.2 LaunchPad Platform Administration Website

This is the internal control plane for operating the SaaS company itself. It must be isolated from customer organization administration.

## 5.2.1 Platform Dashboard

Display:

- total organizations;
- active organizations;
- trial organizations;
- suspended organizations;
- monthly recurring revenue;
- annual recurring revenue;
- churn;
- active users;
- onboarding journeys started and completed;
- API error rate;
- integration health;
- support-ticket volume;
- security alerts;
- recent platform incidents.

## 5.2.2 Organization Management

Platform administrators can:

- search and filter organizations;
- view organization profile;
- view plan, billing status, and usage;
- activate, suspend, or close an organization;
- change subscription plan with authorization;
- inspect enabled features;
- view configured integrations;
- reset an organization administrator's access;
- export organization data subject to authorization;
- trigger data deletion workflows;
- view tenant audit logs;
- impersonate a tenant administrator only through a controlled, audited support mode.

### Support Impersonation Requirements

- explicit reason is mandatory;
- session duration is limited;
- customer notification is configurable;
- all actions are audited;
- sensitive values remain masked;
- impersonation cannot change billing ownership or disable audit logging.

## 5.2.3 Subscription and Billing Operations

Features:

- plan catalogue;
- prices and currencies;
- coupons and promotions;
- subscription status;
- invoices;
- payment attempts;
- refunds and credits;
- tax configuration;
- seat usage;
- billing alerts;
- failed-payment recovery;
- enterprise contract metadata;
- manual invoice notes;
- revenue reporting.

## 5.2.4 Feature Flag Management

Support:

- global flags;
- plan-based flags;
- tenant-specific flags;
- percentage rollouts;
- user-specific test cohorts;
- kill switches;
- rollout history;
- flag-expiry dates.

## 5.2.5 Global Template Marketplace Administration

Platform administrators can:

- create and publish official templates;
- review customer-submitted templates;
- categorize templates;
- version templates;
- remove unsafe or outdated templates;
- track installations and ratings;
- feature selected templates.

## 5.2.6 Platform User and Staff Management

- create platform staff accounts;
- assign internal roles;
- enforce MFA;
- review privileged access;
- deactivate departing staff;
- create break-glass accounts;
- generate access-review reports.

Suggested internal roles:

- platform_owner;
- platform_admin;
- support_agent;
- billing_admin;
- content_editor;
- security_admin;
- analyst;
- read_only_auditor.

## 5.2.7 Customer Support Management

- customer support tickets;
- organization timeline;
- internal notes;
- priority and severity;
- assignment and escalation;
- SLA timers;
- attachment support;
- links to platform logs;
- canned responses;
- customer satisfaction rating;
- root-cause category.

## 5.2.8 Platform Content Management

Manage marketing and help-center content without a code deployment.

## 5.2.9 Platform Security Center

Display:

- suspicious login events;
- privileged actions;
- tenant data-export events;
- failed SSO configurations;
- secret rotation status;
- vulnerability status;
- integration token health;
- security incident records;
- breach-notification workflow;
- audit-log export.

## 5.2.10 Platform Operations

- job queue monitoring;
- dead-letter queue inspection;
- webhook delivery logs;
- email delivery logs;
- integration synchronization logs;
- data-migration progress;
- tenant storage usage;
- scheduled maintenance notices;
- service status publishing.

---

# 5.3 Organization Administration Portal

This is the main administrative product used by each customer organization.

## 5.3.1 Organization Setup Wizard

Steps:

1. organization profile;
2. logo, colors, and domain;
3. departments and teams;
4. job roles;
5. administrators;
6. SSO and identity settings;
7. integrations;
8. onboarding template selection;
9. notification settings;
10. launch readiness check.

## 5.3.2 Employee Directory

Features:

- employee list;
- bulk CSV import;
- HRIS synchronization;
- invitation management;
- employment status;
- manager and buddy assignment;
- department, team, location, and role;
- onboarding status;
- profile timeline;
- custom attributes;
- archive and offboarding transition.

## 5.3.3 Journey Template Builder

Administrators can create reusable onboarding journeys.

### Supported Step Types

- information page;
- document read;
- policy acknowledgement;
- video;
- external course;
- quiz;
- survey;
- file submission;
- text submission;
- coding exercise;
- meeting;
- shadowing session;
- checklist;
- approval;
- equipment request;
- access request;
- automated integration action;
- manager feedback;
- employee reflection;
- certification.

### Workflow Capabilities

- sequential and parallel stages;
- prerequisites;
- conditional branches;
- reusable subflows;
- due dates relative to start date;
- business-day calculations;
- reminders;
- escalations;
- approval rules;
- retries;
- manual overrides;
- versioning;
- drafts and publishing;
- rollback to prior version;
- template cloning;
- localization.

## 5.3.4 Assignment Rules

Automatically assign a journey using:

- job role;
- department;
- team;
- location;
- employment type;
- seniority;
- manager;
- start date;
- legal entity;
- custom employee attributes.

## 5.3.5 Content and Knowledge Management

Organizations can upload or connect:

- policies;
- handbooks;
- process documents;
- videos;
- architecture documentation;
- FAQs;
- wikis;
- SharePoint;
- Notion;
- Confluence;
- Google Drive;
- GitHub repositories;
- internal websites.

Requirements:

- document ownership;
- review dates;
- version history;
- access scopes;
- approval before AI indexing;
- retention policies;
- stale-content alerts;
- source citations in AI responses.

## 5.3.6 Assessments

Question types:

- single choice;
- multiple choice;
- true or false;
- short answer;
- long answer;
- ordering;
- matching;
- scenario decision;
- file submission;
- code submission.

Assessment features:

- question banks;
- randomized questions;
- attempt limits;
- pass scores;
- manual grading;
- automatic grading;
- explanations;
- retake rules;
- certificates;
- integrity controls.

## 5.3.7 Meetings and Scheduling

- manager introduction;
- HR orientation;
- team introduction;
- buddy check-in;
- architecture walkthrough;
- role-specific coaching;
- first-week review.

Calendar integration requirements:

- availability lookup;
- timezone handling;
- meeting templates;
- automatic invitations;
- rescheduling;
- attendance tracking;
- recording or notes links.

## 5.3.8 Equipment and Access Requests

Track requests for:

- laptop;
- mobile device;
- access badge;
- desk equipment;
- VPN;
- email;
- source-control account;
- project-management access;
- cloud environment access;
- internal application roles.

Each request contains:

- owner;
- approver;
- requested access level;
- status;
- due date;
- evidence;
- provisioning result;
- revocation requirement.

## 5.3.9 Manager Dashboard

Managers can view:

- direct reports onboarding;
- current stage;
- overdue tasks;
- blocked tasks;
- upcoming approvals;
- meetings to schedule;
- assessment results;
- AI-detected knowledge gaps;
- employee sentiment;
- first-task readiness;
- recommended manager actions.

## 5.3.10 Analytics and Reports

Organization reports include:

- onboarding completion rate;
- average completion time;
- time to first meaningful task;
- time to first approval;
- time to first repository contribution;
- time to first customer interaction;
- overdue-task rate;
- assessment performance;
- employee satisfaction;
- manager response time;
- access-provisioning time;
- completion by department, role, manager, and location;
- content usefulness;
- frequently asked AI questions;
- unresolved questions;
- onboarding drop-off points.

## 5.3.11 Organization Settings

- branding;
- domain verification;
- localization;
- timezone;
- working days and holidays;
- email sender configuration;
- retention settings;
- security policies;
- SSO;
- SCIM;
- API keys;
- webhooks;
- integrations;
- custom fields;
- audit exports;
- billing access.

---

# 5.4 Employee Onboarding Application

## 5.4.1 Employee Home

Display:

- welcome message;
- onboarding progress;
- today's tasks;
- upcoming meetings;
- overdue work;
- manager and buddy contacts;
- AI assistant;
- announcements;
- achievements;
- help and support.

## 5.4.2 Guided Daily Journey

A default seven-day model:

### Day 0 — Pre-boarding

- confirm personal details;
- sign documents;
- read first-day instructions;
- submit equipment preferences;
- meet assigned buddy virtually.

### Day 1 — Welcome and Setup

- company introduction;
- HR orientation;
- account setup;
- device security;
- team introductions.

### Day 2 — Company and Product

- mission, values, customers, and products;
- organizational structure;
- key terminology;
- product walkthrough;
- short assessment.

### Day 3 — Role and Tools

- department processes;
- role expectations;
- tools and access;
- practical setup exercise.

### Day 4 — Guided Practice

- simulation or sandbox task;
- manager review;
- feedback and remediation.

### Day 5 — First Meaningful Contribution

- small real-world task;
- peer collaboration;
- reflection;
- first-week review.

### Days 6–7 — Reinforcement

- knowledge check;
- unresolved questions;
- development plan;
- readiness certification.

## 5.4.3 AI Onboarding Assistant

The assistant can:

- answer questions using authorized company sources;
- cite documents used;
- explain company acronyms;
- identify system owners;
- locate resources;
- recommend the next task;
- explain why a task is required;
- create a support request when uncertain;
- escalate sensitive questions to a human;
- collect feedback on answer usefulness.

The assistant must not:

- invent company policies;
- reveal unauthorized information;
- make binding HR decisions;
- approve access requests;
- answer from unapproved sources;
- hide uncertainty.

## 5.4.4 Employee Support

- ask HR;
- ask IT;
- ask manager;
- report onboarding blocker;
- request deadline extension;
- provide anonymous feedback where enabled.

---

# 6. Functional Requirements by Domain

## 6.1 Authentication and Identity

Foundation (Phase 0 / MVP):

- custom Go authentication service (no third-party IdP for core auth);
- email and password registration and login;
- password hashing with bcrypt or argon2id;
- JWT access tokens and Redis-backed refresh/session tokens;
- password reset;
- invitation acceptance;
- session revocation;
- organization switching for multi-organization users.

Later phases:

- MFA;
- trusted-device controls;
- SSO using OIDC and SAML;
- SCIM user lifecycle synchronization;
- domain-based SSO discovery.

## 6.2 Multi-Tenancy

Every tenant-owned document must include `organizationId` unless it belongs to a globally isolated collection (for example platform staff or marketing CMS).

Requirements:

- strict tenant scoping at repository level on every MongoDB query;
- compound indexes that always lead with `organizationId` for tenant collections;
- tenant-aware Redis cache keys;
- tenant-aware object-storage paths;
- tenant-aware search indexes;
- tenant-aware event metadata;
- no cross-tenant analytics without platform authorization.

## 6.3 RBAC and Permission Model

Permission format:

```text
resource.action
```

Examples:

```text
employees.read
employees.create
journeys.publish
journeys.assign
assessments.grade
integrations.manage
billing.read
audit.export
```

Roles are collections of permissions. Organizations may create custom roles on eligible plans.

## 6.4 Audit Logging

Audit all security-sensitive and business-critical actions.

Each audit event must contain:

- event ID;
- organization ID;
- actor ID and actor type;
- action;
- resource type and ID;
- timestamp;
- IP address;
- user agent;
- request ID;
- before and after values where safe;
- impersonation context;
- result;
- failure reason.

## 6.5 Notifications

Channels:

- in-app;
- email;
- Slack;
- Microsoft Teams;
- optional SMS.

Notification types:

- invitation;
- task assigned;
- due soon;
- overdue;
- approval required;
- meeting scheduled;
- journey completed;
- certificate issued;
- employee blocked;
- integration failed.

---

# 7. Non-Functional Requirements

## 7.1 Availability

- Target MVP availability: 99.9% monthly.
- Enterprise target: 99.95% or higher.
- Stateless APIs must scale horizontally.
- Background jobs must be retryable and idempotent.

## 7.2 Performance

- Public website LCP under 2.5 seconds at the 75th percentile.
- Standard API read requests under 300 ms at p95 excluding external providers.
- Standard API write requests under 500 ms at p95 excluding external providers.
- Search results under 2 seconds at p95.
- Dashboard initial data under 2 seconds at p95.

## 7.3 Security

- TLS 1.2+.
- encryption at rest.
- MFA for privileged roles.
- least-privilege access.
- secrets manager.
- secure headers.
- rate limiting.
- CSRF protection where cookie auth is used.
- input validation.
- file malware scanning.
- signed upload URLs.
- dependency and container scanning.
- penetration testing before enterprise launch.

## 7.4 Privacy and Compliance

Design for:

- GDPR;
- CCPA-style privacy controls;
- SOC 2 readiness;
- ISO 27001 readiness;
- regional data retention;
- right to access, export, and delete;
- data-processing agreements;
- subprocessor register.

## 7.5 Accessibility

Target WCAG 2.2 AA.

## 7.6 Observability

- structured logs;
- distributed tracing;
- metrics;
- error tracking;
- uptime checks;
- audit logs;
- business events;
- alerting and runbooks.

---

# 8. Recommended Technical Architecture

## 8.1 Architecture Strategy

Start as a **modular monolith** with clear bounded contexts. Split into services only when scaling, ownership, availability, or compliance needs justify it.

This reduces early operational complexity while preserving future service boundaries.

## 8.2 Technology Stack

### Frontend

- Next.js with TypeScript.
- React.
- Tailwind CSS.
- component library built with accessible primitives.
- TanStack Query for server-state management.
- React Hook Form and Zod.

### Backend

- Go.
- REST API for public and application clients.
- OpenAPI 3.1 as the contract.
- internal events through Kafka or a simpler managed queue during MVP.

### Data

- MongoDB as primary application datastore.
- Redis for caching, rate limiting, session/refresh tokens, and short-lived job coordination.
- S3-compatible object storage.
- MongoDB Atlas Search or collection text indexes for initial knowledge search.
- vector embeddings stored in MongoDB (Atlas Vector Search) for MVP AI retrieval.

### Infrastructure

- Docker.
- Kubernetes for production when scale requires it.
- Terraform.
- GitHub Actions.
- Argo CD for GitOps deployments.
- OpenTelemetry, Prometheus, Grafana, and an error-tracking platform.

## 8.3 High-Level Components

```mermaid
flowchart LR
    Visitor[Public Visitor] --> Marketing[Marketing Web]
    Staff[Platform Staff] --> PlatformAdmin[Platform Admin]
    Admin[Organization Admin] --> OrgAdmin[Organization Admin Web]
    Employee[Employee] --> EmployeeApp[Employee Web/Mobile]

    Marketing --> API[API Gateway / Backend]
    PlatformAdmin --> API
    OrgAdmin --> API
    EmployeeApp --> API

    API --> Auth[Identity Module]
    API --> Org[Organization Module]
    API --> Journey[Journey Module]
    API --> Learning[Learning Module]
    API --> Knowledge[Knowledge & AI Module]
    API --> Billing[Billing Module]
    API --> Integrations[Integration Module]
    API --> Analytics[Analytics Module]

    Auth --> DB[(MongoDB)]
    Org --> DB
    Journey --> DB
    Learning --> DB
    Billing --> DB
    API --> Redis[(Redis)]
    API --> Storage[(Object Storage)]
    API --> Events[(Event Bus)]
    Knowledge --> Vector[(MongoDB Vector Search)]
```

---

# 9. Repository and Code Structure

Use a monorepo so contracts, shared UI, infrastructure, and documentation evolve together.

```text
launchpad/
├── README.md
├── Makefile
├── docker-compose.yml
├── .env.example
├── .github/
│   └── workflows/
│       ├── ci.yml
│       ├── backend.yml
│       ├── frontend.yml
│       └── security.yml
├── apps/
│   ├── marketing-web/            # Public website
│   ├── platform-admin-web/       # Internal LaunchPad administration
│   ├── organization-admin-web/   # Customer administration portal
│   ├── employee-web/             # Employee onboarding application
│   └── api/                      # Go backend entry point
├── internal/
│   ├── auth/
│   ├── organizations/
│   ├── employees/
│   ├── journeys/
│   ├── workflows/
│   ├── learning/
│   ├── assessments/
│   ├── knowledge/
│   ├── ai/
│   ├── integrations/
│   ├── notifications/
│   ├── analytics/
│   ├── billing/
│   ├── platformadmin/
│   ├── marketing/
│   ├── audit/
│   └── support/
├── pkg/
│   ├── database/
│   ├── events/
│   ├── httpx/
│   ├── logging/
│   ├── middleware/
│   ├── security/
│   ├── validation/
│   └── testutil/
├── contracts/
│   ├── openapi/
│   ├── events/
│   └── webhooks/
├── packages/
│   ├── ui/
│   ├── eslint-config/
│   ├── typescript-config/
│   ├── api-client/
│   └── analytics-client/
├── migrations/                   # MongoDB index bootstrap and data migration scripts
├── seeds/
├── infra/
│   ├── terraform/
│   ├── kubernetes/
│   ├── helm/
│   └── argocd/
├── docs/
│   ├── product/
│   ├── architecture/
│   ├── api/
│   ├── security/
│   ├── runbooks/
│   └── decisions/
└── scripts/
```

## 9.1 Backend Module Structure

Each domain module should follow a consistent internal structure.

```text
internal/journeys/
├── domain.go          # Entities, value objects, domain rules
├── errors.go          # Domain-specific errors
├── repository.go      # Repository interfaces
├── service.go         # Application use cases
├── handlers.go        # HTTP handlers
├── routes.go          # Route registration
├── events.go          # Domain and integration events
├── validator.go       # Input validation
├── mongo.go           # MongoDB repository implementation
├── mapper.go          # DTO and domain mapping
└── service_test.go
```

Rules:

- HTTP handlers must not contain business logic.
- Services must not depend directly on HTTP frameworks.
- Repository interfaces belong to the domain using them.
- Database models must not leak directly into API responses.
- Cross-module calls should use explicit interfaces or application services.
- Every write use case must define authorization and audit behavior.

## 9.2 Frontend Application Structure

```text
apps/organization-admin-web/
├── app/
│   ├── (auth)/
│   ├── dashboard/
│   ├── employees/
│   ├── journeys/
│   ├── content/
│   ├── analytics/
│   ├── integrations/
│   └── settings/
├── components/
├── features/
│   ├── employees/
│   ├── journey-builder/
│   ├── assessments/
│   └── analytics/
├── lib/
│   ├── api.ts
│   ├── auth.ts
│   ├── permissions.ts
│   └── telemetry.ts
├── hooks/
├── types/
└── tests/
```

Rules:

- Organize domain-specific UI under `features`.
- Keep generic components in the shared `packages/ui` package.
- Generate API clients from OpenAPI.
- Validate server responses at trust boundaries.
- Enforce permissions both in navigation and at the API.
- Do not rely on hidden buttons as authorization.

---

# 10. Core Data Model

## 10.1 Primary Entities

- platform_users;
- organizations;
- organization_domains;
- organization_memberships;
- departments;
- teams;
- job_roles;
- employees;
- employee_managers;
- journey_templates;
- journey_template_versions;
- journey_steps;
- journey_step_dependencies;
- journey_assignments;
- step_assignments;
- task_submissions;
- assessments;
- questions;
- assessment_attempts;
- meetings;
- approvals;
- documents;
- knowledge_sources;
- knowledge_chunks;
- integrations;
- integration_connections;
- access_requests;
- equipment_requests;
- notifications;
- audit_events;
- subscriptions;
- invoices;
- feature_flags;
- support_tickets;
- marketing_leads;
- cms_pages;
- blog_posts.

## 10.2 Example MongoDB Collections

Document IDs use UUIDs stored as strings (or BSON UUID). Timestamps use UTC `date` fields. Tenant collections always include `organizationId`.

```javascript
// organizations
{
  _id: "uuid",
  name: "Acme Corp",
  slug: "acme",
  status: "trial",          // trial | active | suspended | cancelled
  planCode: "starter",
  timezone: "UTC",
  createdAt: ISODate(),
  updatedAt: ISODate()
}
// indexes: unique(slug)

// users
{
  _id: "uuid",
  email: "owner@acme.com",  // stored lowercase
  displayName: "Alex Owner",
  passwordHash: "...",      // argon2id or bcrypt
  status: "active",
  mfaEnabled: false,
  createdAt: ISODate(),
  updatedAt: ISODate()
}
// indexes: unique(email)

// organization_memberships
{
  _id: "uuid",
  organizationId: "uuid",
  userId: "uuid",
  roleCode: "organization_owner",
  status: "active",
  createdAt: ISODate()
}
// indexes: unique(organizationId, userId), index(userId)

// employees
{
  _id: "uuid",
  organizationId: "uuid",
  userId: "uuid",           // optional until invitation accepted
  employeeNumber: "E-1001",
  firstName: "Jordan",
  lastName: "Lee",
  workEmail: "jordan@acme.com",
  jobRoleId: "uuid",
  departmentId: "uuid",
  managerEmployeeId: "uuid",
  startDate: ISODate("2026-08-01"),
  status: "invited",
  metadata: {},
  createdAt: ISODate(),
  updatedAt: ISODate()
}
// indexes: unique(organizationId, workEmail), index(organizationId, status)

// journey_templates
{
  _id: "uuid",
  organizationId: "uuid",
  name: "Software Engineer Day 1–7",
  description: "...",
  status: "draft",          // draft | published | archived
  currentVersion: 1,
  createdBy: "uuid",
  createdAt: ISODate(),
  updatedAt: ISODate()
}
// indexes: index(organizationId, status)

// journey_steps
{
  _id: "uuid",
  organizationId: "uuid",
  journeyTemplateId: "uuid",
  version: 1,
  stepType: "document",     // document | quiz | task | meeting | approval | ...
  title: "Read employee handbook",
  instructions: "...",
  position: 1,
  dueOffsetDays: 1,
  config: {},
  completionRule: {},
  createdAt: ISODate()
}
// indexes: index(organizationId, journeyTemplateId, version, position)

// journey_assignments
{
  _id: "uuid",
  organizationId: "uuid",
  employeeId: "uuid",
  journeyTemplateId: "uuid",
  templateVersion: 1,
  status: "scheduled",
  startsAt: ISODate(),
  dueAt: ISODate(),
  progressPercent: 0,
  completedAt: null,
  createdAt: ISODate()
}
// indexes: unique(organizationId, employeeId, journeyTemplateId, startsAt),
//          index(organizationId, employeeId)

// sessions (also mirrored/cached in Redis)
{
  _id: "uuid",
  userId: "uuid",
  organizationId: "uuid",   // active org context
  refreshTokenHash: "...",
  expiresAt: ISODate(),
  revokedAt: null,
  createdAt: ISODate()
}
// indexes: index(userId), index(expiresAt)

// audit_events
{
  _id: "uuid",
  organizationId: "uuid",   // null for platform-level events
  actorUserId: "uuid",
  action: "organization.created",
  resourceType: "organization",
  resourceId: "uuid",
  metadata: {},
  createdAt: ISODate()
}
// indexes: index(organizationId, createdAt), index(actorUserId, createdAt)
```

Required MongoDB index rule: every tenant-scoped query filter must include `organizationId` as the first predicate, matching a compound index.
---

# 11. API Design

## 11.1 API Conventions

Base path:

```text
/api/v1
```

Standard response:

```json
{
  "data": {},
  "meta": {
    "request_id": "req_123"
  }
}
```

Standard error:

```json
{
  "error": {
    "code": "JOURNEY_NOT_FOUND",
    "message": "The requested journey does not exist.",
    "details": []
  },
  "meta": {
    "request_id": "req_123"
  }
}
```

## 11.2 Important Endpoints

### Public and Marketing

```text
POST   /api/v1/public/leads
POST   /api/v1/public/trials
GET    /api/v1/public/pricing
GET    /api/v1/public/integrations
GET    /api/v1/public/templates
GET    /api/v1/public/blog/posts
```

### Organization Administration

```text
GET    /api/v1/organizations/current
PATCH  /api/v1/organizations/current
GET    /api/v1/employees
POST   /api/v1/employees
POST   /api/v1/employees/import
GET    /api/v1/employees/{employee_id}
PATCH  /api/v1/employees/{employee_id}
POST   /api/v1/employees/{employee_id}/invite

GET    /api/v1/journey-templates
POST   /api/v1/journey-templates
GET    /api/v1/journey-templates/{template_id}
PATCH  /api/v1/journey-templates/{template_id}
POST   /api/v1/journey-templates/{template_id}/publish
POST   /api/v1/journey-templates/{template_id}/assign

GET    /api/v1/analytics/onboarding
GET    /api/v1/audit-events
GET    /api/v1/integrations
POST   /api/v1/integrations/{provider}/connect
```

### Employee Application

```text
GET    /api/v1/me
GET    /api/v1/me/journeys
GET    /api/v1/me/journeys/{assignment_id}
POST   /api/v1/me/steps/{step_assignment_id}/start
POST   /api/v1/me/steps/{step_assignment_id}/submit
POST   /api/v1/me/steps/{step_assignment_id}/complete
POST   /api/v1/me/assistant/messages
POST   /api/v1/me/blockers
```

### Platform Administration

```text
GET    /api/v1/platform/organizations
GET    /api/v1/platform/organizations/{organization_id}
POST   /api/v1/platform/organizations/{organization_id}/suspend
POST   /api/v1/platform/support-sessions
GET    /api/v1/platform/subscriptions
GET    /api/v1/platform/operations/jobs
GET    /api/v1/platform/security/events
```

---

# 12. Event Model

Events must include tenant and trace context.

```json
{
  "event_id": "b5d5d78d-32d3-42cd-90a6-888d64ea44a1",
  "event_type": "journey.assignment.created",
  "event_version": 1,
  "occurred_at": "2026-07-23T12:00:00Z",
  "organization_id": "org_uuid",
  "actor_id": "user_uuid",
  "trace_id": "trace_uuid",
  "payload": {
    "assignment_id": "assignment_uuid",
    "employee_id": "employee_uuid",
    "journey_template_id": "template_uuid"
  }
}
```

Important events:

- organization.created;
- employee.invited;
- employee.activated;
- journey.template.published;
- journey.assignment.created;
- journey.step.completed;
- journey.assignment.completed;
- assessment.completed;
- approval.requested;
- approval.completed;
- access.requested;
- access.provisioned;
- integration.failed;
- subscription.changed;
- support.impersonation.started.

---

# 13. Starter Backend Code

The following code is representative starter code. It is not the complete product, but establishes conventions an AI coding agent should follow.

## 13.1 Go Application Entry Point

```go
package main

import (
    "context"
    "errors"
    "log/slog"
    "net/http"
    "os"
    "os/signal"
    "syscall"
    "time"

    "launchpad/pkg/database"
    "launchpad/pkg/httpx"
)

func main() {
    logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

    db, err := database.Open(os.Getenv("DATABASE_URL"))
    if err != nil {
        logger.Error("database connection failed", "error", err)
        os.Exit(1)
    }
    defer db.Close()

    router := http.NewServeMux()
    registerRoutes(router, db, logger)

    server := &http.Server{
        Addr:              envOrDefault("HTTP_ADDR", ":8080"),
        Handler:           httpx.WithStandardMiddleware(router, logger),
        ReadHeaderTimeout: 5 * time.Second,
        ReadTimeout:       15 * time.Second,
        WriteTimeout:      30 * time.Second,
        IdleTimeout:       60 * time.Second,
    }

    go func() {
        logger.Info("api started", "address", server.Addr)
        if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
            logger.Error("api stopped unexpectedly", "error", err)
            os.Exit(1)
        }
    }()

    stop := make(chan os.Signal, 1)
    signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
    <-stop

    ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
    defer cancel()

    if err := server.Shutdown(ctx); err != nil {
        logger.Error("graceful shutdown failed", "error", err)
    }
}

func envOrDefault(key, fallback string) string {
    if value := os.Getenv(key); value != "" {
        return value
    }
    return fallback
}
```

## 13.2 Journey Domain and Service

```go
package journeys

import (
    "context"
    "errors"
    "time"

    "github.com/google/uuid"
)

var ErrTemplateNotPublished = errors.New("journey template is not published")

type Template struct {
    ID             uuid.UUID
    OrganizationID uuid.UUID
    Name           string
    Status         string
    CurrentVersion int
}

type Assignment struct {
    ID                uuid.UUID
    OrganizationID    uuid.UUID
    EmployeeID        uuid.UUID
    JourneyTemplateID uuid.UUID
    TemplateVersion   int
    Status            string
    StartsAt          time.Time
}

type Repository interface {
    FindTemplate(ctx context.Context, organizationID, templateID uuid.UUID) (Template, error)
    CreateAssignment(ctx context.Context, assignment Assignment) error
    AssignmentExists(ctx context.Context, organizationID, employeeID, templateID uuid.UUID, startsAt time.Time) (bool, error)
}

type EventPublisher interface {
    Publish(ctx context.Context, eventType string, payload any) error
}

type AuditWriter interface {
    Write(ctx context.Context, action string, resourceID uuid.UUID, metadata map[string]any) error
}

type Service struct {
    repo   Repository
    events EventPublisher
    audit  AuditWriter
}

func NewService(repo Repository, events EventPublisher, audit AuditWriter) *Service {
    return &Service{repo: repo, events: events, audit: audit}
}

type AssignInput struct {
    OrganizationID uuid.UUID
    EmployeeID     uuid.UUID
    TemplateID     uuid.UUID
    StartsAt       time.Time
}

func (s *Service) Assign(ctx context.Context, input AssignInput) (Assignment, error) {
    template, err := s.repo.FindTemplate(ctx, input.OrganizationID, input.TemplateID)
    if err != nil {
        return Assignment{}, err
    }
    if template.Status != "published" {
        return Assignment{}, ErrTemplateNotPublished
    }

    exists, err := s.repo.AssignmentExists(
        ctx,
        input.OrganizationID,
        input.EmployeeID,
        input.TemplateID,
        input.StartsAt,
    )
    if err != nil {
        return Assignment{}, err
    }
    if exists {
        return Assignment{}, errors.New("duplicate journey assignment")
    }

    assignment := Assignment{
        ID:                uuid.New(),
        OrganizationID:    input.OrganizationID,
        EmployeeID:        input.EmployeeID,
        JourneyTemplateID: input.TemplateID,
        TemplateVersion:   template.CurrentVersion,
        Status:            "scheduled",
        StartsAt:          input.StartsAt.UTC(),
    }

    if err := s.repo.CreateAssignment(ctx, assignment); err != nil {
        return Assignment{}, err
    }

    _ = s.events.Publish(ctx, "journey.assignment.created", assignment)
    _ = s.audit.Write(ctx, "journeys.assign", assignment.ID, map[string]any{
        "employee_id": input.EmployeeID,
        "template_id": input.TemplateID,
    })

    return assignment, nil
}
```

## 13.3 HTTP Handler

```go
package journeys

import (
    "encoding/json"
    "net/http"
    "time"

    "github.com/google/uuid"
    "launchpad/pkg/httpx"
    "launchpad/pkg/security"
)

type Handler struct {
    service *Service
}

func NewHandler(service *Service) *Handler {
    return &Handler{service: service}
}

type assignJourneyRequest struct {
    EmployeeID string    `json:"employee_id"`
    StartsAt   time.Time `json:"starts_at"`
}

func (h *Handler) Assign(w http.ResponseWriter, r *http.Request) {
    principal, ok := security.PrincipalFromContext(r.Context())
    if !ok {
        httpx.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication is required")
        return
    }
    if !principal.Can("journeys.assign") {
        httpx.WriteError(w, http.StatusForbidden, "FORBIDDEN", "Missing required permission")
        return
    }

    templateID, err := uuid.Parse(r.PathValue("template_id"))
    if err != nil {
        httpx.WriteError(w, http.StatusBadRequest, "INVALID_TEMPLATE_ID", "Invalid template ID")
        return
    }

    var request assignJourneyRequest
    decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
    decoder.DisallowUnknownFields()
    if err := decoder.Decode(&request); err != nil {
        httpx.WriteError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
        return
    }

    employeeID, err := uuid.Parse(request.EmployeeID)
    if err != nil {
        httpx.WriteError(w, http.StatusBadRequest, "INVALID_EMPLOYEE_ID", "Invalid employee ID")
        return
    }

    assignment, err := h.service.Assign(r.Context(), AssignInput{
        OrganizationID: principal.OrganizationID,
        EmployeeID:     employeeID,
        TemplateID:     templateID,
        StartsAt:       request.StartsAt,
    })
    if err != nil {
        httpx.WriteDomainError(w, err)
        return
    }

    httpx.WriteJSON(w, http.StatusCreated, map[string]any{"data": assignment})
}
```

## 13.4 Tenant-Scoping Middleware

```go
package middleware

import (
    "net/http"

    "launchpad/pkg/httpx"
    "launchpad/pkg/security"
)

func RequireOrganization(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        principal, ok := security.PrincipalFromContext(r.Context())
        if !ok || principal.OrganizationID.String() == "" {
            httpx.WriteError(
                w,
                http.StatusForbidden,
                "ORGANIZATION_CONTEXT_REQUIRED",
                "An organization context is required",
            )
            return
        }
        next.ServeHTTP(w, r)
    })
}
```

---

# 14. Starter Frontend Code

## 14.1 Marketing Home Page

```tsx
import Link from "next/link";

export default function HomePage() {
  return (
    <main>
      <section className="mx-auto max-w-7xl px-6 py-24 text-center">
        <p className="mb-4 text-sm font-semibold uppercase tracking-wide">
          Employee onboarding, orchestrated
        </p>
        <h1 className="mx-auto max-w-4xl text-5xl font-bold tracking-tight">
          Help every new employee become confident and productive faster.
        </h1>
        <p className="mx-auto mt-6 max-w-2xl text-lg text-neutral-600">
          Build guided onboarding journeys, automate setup, deliver role-based
          training, and measure progress from one secure platform.
        </p>
        <div className="mt-10 flex justify-center gap-4">
          <Link
            href="/signup"
            className="rounded-lg bg-black px-6 py-3 text-white"
          >
            Start free trial
          </Link>
          <Link
            href="/demo"
            className="rounded-lg border border-neutral-300 px-6 py-3"
          >
            Book a demo
          </Link>
        </div>
      </section>
    </main>
  );
}
```

## 14.2 Organization Admin Dashboard Page

```tsx
import { Suspense } from "react";
import { MetricCard } from "@launchpad/ui/metric-card";
import { getOnboardingSummary } from "@/features/analytics/api";

async function DashboardMetrics() {
  const summary = await getOnboardingSummary();

  return (
    <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
      <MetricCard
        label="Active onboardings"
        value={summary.activeAssignments}
      />
      <MetricCard
        label="Completion rate"
        value={`${summary.completionRate}%`}
      />
      <MetricCard
        label="Overdue tasks"
        value={summary.overdueSteps}
      />
      <MetricCard
        label="Average completion"
        value={`${summary.averageCompletionDays} days`}
      />
    </div>
  );
}

export default function DashboardPage() {
  return (
    <main className="space-y-8 p-8">
      <div>
        <h1 className="text-3xl font-semibold">Onboarding overview</h1>
        <p className="text-neutral-600">
          Track progress, blockers, approvals, and upcoming employee starts.
        </p>
      </div>
      <Suspense fallback={<p>Loading onboarding metrics…</p>}>
        <DashboardMetrics />
      </Suspense>
    </main>
  );
}
```

## 14.3 Typed API Client

```ts
import { z } from "zod";

const onboardingSummarySchema = z.object({
  activeAssignments: z.number().int().nonnegative(),
  completionRate: z.number().min(0).max(100),
  overdueSteps: z.number().int().nonnegative(),
  averageCompletionDays: z.number().nonnegative(),
});

export type OnboardingSummary = z.infer<typeof onboardingSummarySchema>;

export async function getOnboardingSummary(): Promise<OnboardingSummary> {
  const response = await fetch(
    `${process.env.API_BASE_URL}/api/v1/analytics/onboarding/summary`,
    {
      headers: { Accept: "application/json" },
      cache: "no-store",
    },
  );

  if (!response.ok) {
    throw new Error(`Unable to load onboarding summary: ${response.status}`);
  }

  const body: unknown = await response.json();
  return onboardingSummarySchema.parse(
    (body as { data: unknown }).data,
  );
}
```

---

# 15. Workflow Execution Rules

## 15.1 Assignment Creation

When a journey is assigned:

1. freeze the published template version;
2. create all immediately eligible step assignments;
3. calculate due dates using organization working days;
4. create required approval and meeting records;
5. publish an assignment-created event;
6. notify the employee and manager;
7. write an audit event.

## 15.2 Step Completion

A step can be completed only when:

- the employee has access to it;
- prerequisites are complete;
- required submission exists;
- required assessment score is achieved;
- required approval is complete;
- completion rule evaluates to true.

## 15.3 Idempotency

All externally retried writes must support an `Idempotency-Key` header.

Examples:

- employee invitations;
- journey assignment;
- payment operations;
- access provisioning;
- webhook processing.

---

# 16. AI and Knowledge Architecture

## 16.1 Ingestion Flow

1. source connection created;
2. source permissions validated;
3. documents synchronized;
4. malware and file validation performed;
5. text extracted;
6. content classified;
7. sensitive sections filtered or access-scoped;
8. chunks created;
9. embeddings generated;
10. source and chunk metadata stored;
11. search index updated;
12. ingestion report generated.

## 16.2 Retrieval Requirements

- filter by organization;
- filter by user permissions;
- prioritize authoritative and recent sources;
- include source citations;
- expose uncertainty;
- refuse when no reliable source exists;
- log answer feedback;
- prevent prompt injection from indexed content.

## 16.3 AI Evaluation

Maintain an evaluation dataset containing:

- common employee questions;
- expected sources;
- expected answer elements;
- forbidden disclosures;
- unsupported questions;
- multilingual queries.

Track:

- groundedness;
- citation correctness;
- answer usefulness;
- refusal correctness;
- latency;
- cost;
- escalation rate.

---

# 17. Testing Strategy

## 17.1 Backend

- unit tests for domain rules;
- repository integration tests using MongoDB;
- HTTP contract tests;
- authorization tests;
- tenant-isolation tests;
- event-schema tests;
- idempotency tests;
- migration tests for index bootstrap scripts;
- load tests for critical endpoints.

## 17.2 Frontend

- component tests;
- accessibility tests;
- API mocking tests;
- end-to-end tests with Playwright;
- responsive-layout tests;
- permission-visibility tests;
- form validation tests.

## 17.3 Security

- SAST;
- dependency scanning;
- secret scanning;
- container scanning;
- DAST;
- penetration testing;
- cross-tenant access testing;
- privilege-escalation testing;
- AI prompt-injection testing.

## 17.4 Minimum Critical End-to-End Scenarios

1. Organization signs up from the marketing website.
2. Owner completes setup wizard.
3. HR imports an employee.
4. HR assigns a published journey.
5. Employee accepts invitation.
6. Employee completes a document and quiz step.
7. Manager approves a practical exercise.
8. Journey completes and certificate is issued.
9. Analytics reflect completion.
10. Every action appears in audit logs.

---

# 18. DevOps and Deployment

## 18.1 Environments

- local;
- test;
- development;
- staging;
- production.

## 18.2 CI Pipeline

Required stages:

1. formatting;
2. linting;
3. unit tests;
4. integration tests;
5. API contract validation;
6. frontend build;
7. backend build;
8. vulnerability scan;
9. container build;
10. signed artifact publication;
11. deployment to staging;
12. smoke tests;
13. controlled production promotion.

## 18.3 Local Docker Compose

```yaml
services:
  mongo:
    image: mongo:7
    environment:
      MONGO_INITDB_DATABASE: launchpad
      MONGO_INITDB_ROOT_USERNAME: launchpad
      MONGO_INITDB_ROOT_PASSWORD: launchpad
    ports:
      - "27017:27017"
    healthcheck:
      test: ["CMD", "mongosh", "--eval", "db.adminCommand('ping')"]
      interval: 5s
      timeout: 5s
      retries: 10

  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"

  api:
    build:
      context: .
      dockerfile: apps/api/Dockerfile
    environment:
      MONGODB_URI: mongodb://launchpad:launchpad@mongo:27017/launchpad?authSource=admin
      REDIS_URL: redis://redis:6379/0
      HTTP_ADDR: :8080
      JWT_SECRET: local-dev-only-change-me
    ports:
      - "8080:8080"
    depends_on:
      mongo:
        condition: service_healthy
      redis:
        condition: service_started
```

---

# 19. MVP Release Definition

The MVP is complete when an organization can:

- create an account;
- configure its profile;
- invite administrators;
- create departments, roles, and employees;
- build and publish an onboarding journey;
- assign the journey;
- allow an employee to complete tasks and quizzes;
- allow a manager to approve work;
- send notifications;
- view progress dashboards;
- review audit logs;
- use a basic AI assistant over approved documents;
- manage subscription status;
- obtain support through the platform.

The LaunchPad company must also be able to:

- market the product through the public website;
- collect and manage leads;
- manage tenants;
- manage plans and feature flags;
- inspect platform health;
- support customers securely;
- publish content and templates.

---

# 20. Delivery Roadmap

## Phase 0 — Foundation

- product design;
- design system;
- monorepo;
- CI;
- authentication;
- organization model;
- audit foundation;
- public marketing site foundation.

## Phase 1 — Core Onboarding MVP

- employees;
- roles and departments;
- journey builder;
- assignments;
- tasks;
- quizzes;
- approvals;
- notifications;
- employee dashboard;
- organization admin dashboard.

## Phase 2 — Business Operations

- platform admin;
- billing;
- feature flags;
- support tickets;
- CMS;
- lead management;
- analytics.

## Phase 3 — AI and Integrations

- knowledge ingestion;
- AI assistant;
- Slack and Teams;
- Google and Microsoft calendars;
- HRIS synchronization;
- SSO and SCIM;
- GitHub and Jira integration.

## Phase 4 — Enterprise

- advanced compliance;
- regional data controls;
- marketplace;
- custom roles;
- advanced workflow conditions;
- mobile applications;
- customer-managed encryption options.

---

# 21. AI Coding Agent Execution Rules

An AI agent implementing this product must:

1. read this specification before creating code;
2. implement one bounded module at a time;
3. create or update OpenAPI contracts before implementing endpoints;
4. create MongoDB index/schema migration scripts for collection changes;
5. preserve tenant isolation in every query;
6. add authorization checks to every protected operation;
7. write audit events for privileged and critical actions;
8. add tests with each feature;
9. avoid introducing a new service without an architecture decision record;
10. never commit secrets;
11. maintain backward compatibility or version contracts;
12. update documentation when behavior changes;
13. expose operational metrics for background jobs and integrations;
14. use idempotency for retryable writes;
15. produce a migration and rollback plan for risky changes.

## Definition of Done for Every Feature

- requirements implemented;
- acceptance criteria satisfied;
- unit and integration tests pass;
- tenant isolation verified;
- authorization verified;
- audit behavior verified;
- API documentation updated;
- observability added;
- accessibility reviewed;
- security risks reviewed;
- deployment and rollback considered;
- no unresolved critical defects.

---

# 22. Next Required Specifications

This document defines the build-ready product foundation. The following companion documents should be created as implementation proceeds:

- detailed user stories and acceptance criteria;
- complete RBAC matrix;
- full database dictionary and ERD;
- complete OpenAPI specification;
- event contract catalogue;
- workflow expression language;
- UI wireframes and design system;
- integration-by-integration specifications;
- AI threat model and evaluation plan;
- billing and metering specification;
- data retention and deletion specification;
- operational runbooks;
- disaster recovery plan;
- release checklist.

---

# 23. Final Product Principle

LaunchPad should never make onboarding feel like a folder of documents. The product must always show the employee:

- what to do now;
- why it matters;
- who can help;
- where the required resource is;
- how success will be measured;
- what happens next.
