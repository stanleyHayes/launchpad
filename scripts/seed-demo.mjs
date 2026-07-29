const apiBaseUrl = (process.env.API_BASE_URL ?? "http://localhost:8080").replace(/\/$/, "");

const credentials = {
  owner: {
    email: process.env.DEMO_OWNER_EMAIL ?? "owner@demo.launchpad.local",
    password: process.env.DEMO_OWNER_PASSWORD ?? "LaunchPadDemo!2026",
  },
  manager: {
    email: "manager@demo.launchpad.local",
    password: "LaunchPadDemo!2026",
  },
  employee: {
    email: "employee@demo.launchpad.local",
    password: "LaunchPadDemo!2026",
  },
};

let accessToken = "";

function dateOnly(timestamp = Date.now()) {
  return new Date(timestamp).toISOString().slice(0, 10);
}

async function request(path, { method = "GET", body, allow = [] } = {}) {
  const response = await fetch(`${apiBaseUrl}${path}`, {
    method,
    headers: {
      Accept: "application/json",
      ...(body === undefined ? {} : { "Content-Type": "application/json" }),
      ...(accessToken ? { Authorization: `Bearer ${accessToken}` } : {}),
    },
    ...(body === undefined ? {} : { body: JSON.stringify(body) }),
  });

  const payload = await response.json().catch(() => ({}));
  if (!response.ok && !allow.includes(response.status)) {
    const message = payload?.error?.message ?? `${method} ${path} failed`;
    throw new Error(`${message} (${response.status})`);
  }

  return { status: response.status, data: payload?.data };
}

async function authenticateOwner() {
  const login = await request("/api/v1/auth/login", {
    method: "POST",
    body: credentials.owner,
    allow: [401],
  });

  if (login.status === 200) {
    accessToken = login.data.tokens.accessToken;
    return login.data;
  }

  const registration = await request("/api/v1/auth/register", {
    method: "POST",
    body: {
      ...credentials.owner,
      displayName: "Ama Mensah",
      organizationName: "Northstar Labs",
      organizationSlug: "northstar-labs",
      timezone: "Africa/Accra",
    },
  });
  accessToken = registration.data.tokens.accessToken;
  return registration.data;
}

async function ensureNamed(path, name, body) {
  const list = await request(path);
  const existing = list.data.find((item) => item.name === name);
  if (existing) return existing;
  return (await request(path, { method: "POST", body })).data;
}

async function ensureEmployee(payload, password) {
  const list = await request("/api/v1/employees?limit=100");
  let employee = list.data.find((item) => item.workEmail === payload.workEmail);
  if (!employee) {
    employee = (await request("/api/v1/employees", { method: "POST", body: payload })).data;
  }

  if (employee.status === "invited") {
    await request(`/api/v1/employees/${encodeURIComponent(employee.id)}/provision`, {
      method: "POST",
      body: {
        password,
        displayName: `${employee.firstName} ${employee.lastName}`,
      },
    });
    employee = (await request(`/api/v1/employees/${encodeURIComponent(employee.id)}`)).data;
  }
  return employee;
}

async function ensureJourney() {
  const journeys = (await request("/api/v1/journeys")).data;
  let journey = journeys.find((item) => item.name === "Engineering first 30 days");
  if (!journey) {
    journey = (await request("/api/v1/journeys", {
      method: "POST",
      body: {
        name: "Engineering first 30 days",
        description: "A practical path from pre-boarding through the first production contribution.",
      },
    })).data;
  }

  if (journey.status === "draft") {
    const steps = (await request(`/api/v1/journeys/${encodeURIComponent(journey.id)}/steps`)).data;
    if (steps.length === 0) {
      const stepInputs = [
        {
          stepType: "document",
          title: "Read the engineering handbook",
          instructions: "Review team principles, communication norms, and the delivery lifecycle.",
          dueOffsetDays: 0,
          businessDays: true,
          stage: "Day one",
        },
        {
          stepType: "access_request",
          title: "Request development access",
          instructions: "Request GitHub, Jira, staging, and observability access.",
          dueOffsetDays: 1,
          businessDays: true,
          stage: "Day one",
          config: { item: "Engineering toolchain" },
        },
        {
          stepType: "meeting",
          title: "Meet your onboarding buddy",
          instructions: "Agree on check-in times and capture the first-week questions.",
          dueOffsetDays: 2,
          businessDays: true,
          stage: "Week one",
          parallelGroup: "relationships",
        },
        {
          stepType: "task",
          title: "Run the product locally",
          instructions: "Boot the stack, execute the test suite, and document one improvement.",
          dueOffsetDays: 4,
          businessDays: true,
          stage: "Week one",
          parallelGroup: "practice",
        },
        {
          stepType: "approval",
          title: "Manager readiness check",
          instructions: "Review access, context, and first-delivery readiness with your manager.",
          dueOffsetDays: 10,
          businessDays: true,
          stage: "Week two",
        },
      ];

      for (const step of stepInputs) {
        await request(`/api/v1/journeys/${encodeURIComponent(journey.id)}/steps`, {
          method: "POST",
          body: step,
        });
      }
    }
    journey = (await request(`/api/v1/journeys/${encodeURIComponent(journey.id)}/publish`, {
      method: "POST",
    })).data;
  }

  return journey;
}

async function ensureAssignments(journey, employees) {
  const assignments = (await request("/api/v1/assignments")).data;
  for (const employee of employees) {
    const exists = assignments.some(
      (item) =>
        item.employeeId === employee.id &&
        item.journeyTemplateId === journey.id,
    );
    if (!exists) {
      await request("/api/v1/assignments", {
        method: "POST",
        body: {
          employeeId: employee.id,
          journeyTemplateId: journey.id,
        },
      });
    }
  }
}

async function ensureKnowledge() {
  const documents = (await request("/api/v1/knowledge/documents")).data;
  if (documents.some((item) => item.title === "Software access guide")) return;

  const document = (await request("/api/v1/knowledge/documents", {
    method: "POST",
    body: {
      title: "Software access guide",
      source: "manual",
      body: "Request software through the Access Requests page. Select the application, explain the business need, and submit it for manager approval. Security-sensitive tools also require IT approval.",
      tags: ["access", "security", "day-one"],
      accessScope: "organization",
      reviewDate: new Date(Date.now() + 90 * 86_400_000).toISOString(),
      retentionDays: 365,
    },
  })).data;

  await request(`/api/v1/knowledge/documents/${encodeURIComponent(document.id)}/approve`, {
    method: "POST",
  });
  await request(`/api/v1/knowledge/documents/${encodeURIComponent(document.id)}/index`, {
    method: "POST",
  });
}

async function ensureAssessment() {
  const assessments = (await request("/api/v1/assessments")).data;
  if (assessments.some((item) => item.title === "Security essentials")) return;

  const assessment = (await request("/api/v1/assessments", {
    method: "POST",
    body: {
      title: "Security essentials",
      description: "A short readiness check for every new team member.",
      passingScore: 75,
      maxAttempts: 3,
      randomize: true,
      questions: [
        {
          type: "single_choice",
          text: "Where should a suspected phishing message be reported?",
          options: ["Public chat", "Security channel", "Personal notes"],
          correctOptions: [1],
          points: 1,
        },
        {
          type: "multiple_choice",
          text: "Which practices protect company accounts?",
          options: ["MFA", "Password sharing", "A password manager"],
          correctOptions: [0, 2],
          points: 2,
        },
        {
          type: "true_false",
          text: "Customer data may be copied into unapproved tools.",
          options: ["True", "False"],
          correctOptions: [1],
          points: 1,
        },
      ],
    },
  })).data;

  await request(`/api/v1/assessments/${encodeURIComponent(assessment.id)}/publish`, {
    method: "POST",
  });
}

async function ensureSupportTicket() {
  const tickets = (await request("/api/v1/support/tickets")).data;
  if (tickets.some((item) => item.subject === "Laptop delivery timing")) return;
  await request("/api/v1/support/tickets", {
    method: "POST",
    body: {
      subject: "Laptop delivery timing",
      body: "The new starter begins next Monday. Please confirm the laptop delivery and setup window.",
      priority: "high",
      category: "it",
    },
  });
}

async function main() {
  await authenticateOwner();

  const engineering = await ensureNamed("/api/v1/departments", "Engineering", {
    name: "Engineering",
    description: "Product engineering, platform, and quality.",
  });
  const operations = await ensureNamed("/api/v1/departments", "People Operations", {
    name: "People Operations",
    description: "Employee experience and workplace operations.",
  });
  const engineeringManager = await ensureNamed("/api/v1/job-roles", "Engineering Manager", {
    name: "Engineering Manager",
    description: "Leads delivery and develops the engineering team.",
  });
  const softwareEngineer = await ensureNamed("/api/v1/job-roles", "Software Engineer", {
    name: "Software Engineer",
    description: "Builds and operates LaunchPad product capabilities.",
  });

  const manager = await ensureEmployee({
    employeeNumber: "NS-100",
    firstName: "Kwame",
    lastName: "Asante",
    workEmail: credentials.manager.email,
    mobilePhone: "+233 24 555 0100",
    departmentId: engineering.id,
    jobRoleId: engineeringManager.id,
    team: "Platform",
    location: "Accra",
    startDate: dateOnly(Date.now() - 60 * 86_400_000),
  }, credentials.manager.password);

  const employee = await ensureEmployee({
    employeeNumber: "NS-118",
    firstName: "Esi",
    lastName: "Owusu",
    workEmail: credentials.employee.email,
    mobilePhone: "+233 20 555 0118",
    departmentId: engineering.id,
    jobRoleId: softwareEngineer.id,
    managerEmployeeId: manager.id,
    buddyEmployeeId: manager.id,
    team: "Platform",
    location: "Remote, Ghana",
    startDate: dateOnly(),
  }, credentials.employee.password);

  const secondEmployee = await ensureEmployee({
    employeeNumber: "NS-119",
    firstName: "Nana",
    lastName: "Boateng",
    workEmail: "nana.boateng@demo.launchpad.local",
    mobilePhone: "+233 50 555 0119",
    departmentId: operations.id,
    jobRoleId: softwareEngineer.id,
    managerEmployeeId: manager.id,
    buddyEmployeeId: employee.id,
    team: "Employee Experience",
    location: "Kumasi",
    startDate: dateOnly(Date.now() + 5 * 86_400_000),
  }, "LaunchPadDemo!2026");

  const journey = await ensureJourney();
  await ensureAssignments(journey, [employee, secondEmployee]);
  await ensureKnowledge();
  await ensureAssessment();
  await ensureSupportTicket();

  console.log(`
LaunchPad demo data is ready.

Organization admin
  URL:      http://localhost:3002/login
  Email:    ${credentials.owner.email}
  Password: ${credentials.owner.password}

Employee portal
  URL:      http://localhost:3003/login
  Email:    ${credentials.employee.email}
  Password: ${credentials.employee.password}

Manager employee account
  Email:    ${credentials.manager.email}
  Password: ${credentials.manager.password}

Platform admin
  URL:      http://localhost:3001/login
  Email:    platform@launchpad.local
  Password: platform-owner-password
`);
}

main().catch((error) => {
  console.error(`Demo seed failed: ${error.message}`);
  process.exitCode = 1;
});
