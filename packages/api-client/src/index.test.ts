import { afterEach, describe, expect, it, vi } from "vitest";

import { ApiError, createLaunchPadClient } from "./index";

function jsonResponse(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

const organizationPayload = {
  id: "org-1",
  name: "Acme",
  slug: "acme",
  status: "active",
  planCode: "growth",
  timezone: "UTC",
  createdAt: "2026-01-01T00:00:00Z",
  updatedAt: "2026-01-02T00:00:00Z",
};

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("API response envelope handling", () => {
  it("returns parsed data for a valid envelope", async () => {
    const fetchMock = vi.fn<typeof fetch>(async () => jsonResponse({ data: organizationPayload }));
    vi.stubGlobal("fetch", fetchMock);

    const client = createLaunchPadClient({ baseUrl: "http://api.test/" });
    const organization = await client.getCurrentOrganization();

    expect(organization).toEqual(organizationPayload);
    expect(fetchMock).toHaveBeenCalledTimes(1);
    expect(fetchMock.mock.calls[0]?.[0]).toBe("http://api.test/api/v1/organizations/current");
  });

  it("rejects a malformed envelope as INVALID_RESPONSE", async () => {
    vi.stubGlobal("fetch", vi.fn(async () => jsonResponse({ data: { unexpected: true } })));

    const client = createLaunchPadClient({ baseUrl: "http://api.test" });
    const error = await client.getCurrentOrganization().catch((e: unknown) => e);

    expect(error).toBeInstanceOf(ApiError);
    expect(error).toMatchObject({ status: 200, code: "INVALID_RESPONSE" });
  });

  it("rejects a success envelope missing data as INVALID_RESPONSE", async () => {
    vi.stubGlobal("fetch", vi.fn(async () => jsonResponse({})));

    const client = createLaunchPadClient({ baseUrl: "http://api.test" });
    const error = await client.getCurrentOrganization().catch((e: unknown) => e);

    expect(error).toBeInstanceOf(ApiError);
    expect(error).toMatchObject({ status: 200, code: "INVALID_RESPONSE" });
  });

  it("maps an error envelope to ApiError with status and code", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async () =>
        jsonResponse({ error: { code: "FORBIDDEN", message: "Insufficient role" } }, 403),
      ),
    );

    const client = createLaunchPadClient({ baseUrl: "http://api.test" });
    const error = await client.getCurrentOrganization().catch((e: unknown) => e);

    expect(error).toBeInstanceOf(ApiError);
    expect(error).toMatchObject({
      status: 403,
      code: "FORBIDDEN",
      message: "Insufficient role",
    });
  });

  it("attaches the bearer token for explicit-token calls (meWithToken)", async () => {
    const mePayload = {
      user: { id: "u-1", email: "a@b.co", displayName: "A", status: "active" },
      organization: organizationPayload,
      roleCode: "employee",
      sessionId: "s-1",
    };
    const fetchMock = vi.fn<typeof fetch>(async () => jsonResponse({ data: mePayload }));
    vi.stubGlobal("fetch", fetchMock);

    const client = createLaunchPadClient({ baseUrl: "http://api.test" });
    await client.meWithToken("token-123");

    const headers = fetchMock.mock.calls[0]?.[1]?.headers as Headers;
    expect(headers.get("Authorization")).toBe("Bearer token-123");
    expect(headers.get("Accept")).toBe("application/json");
  });
});

describe("RBAC and platform oversight endpoints", () => {
  const rolePayload = {
    id: "role-1",
    organizationId: "org-1",
    name: "team_lead",
    permissions: ["employees.read", "journeys.assign"],
    builtin: false,
    createdAt: "2026-01-01T00:00:00Z",
    updatedAt: "2026-01-01T00:00:00Z",
  };

  it("lists roles from the envelope", async () => {
    const fetchMock = vi.fn<typeof fetch>(async () => jsonResponse({ data: [rolePayload] }));
    vi.stubGlobal("fetch", fetchMock);

    const client = createLaunchPadClient({ baseUrl: "http://api.test" });
    const roles = await client.listRoles();

    expect(roles).toEqual([rolePayload]);
    expect(fetchMock.mock.calls[0]?.[0]).toBe(
      "http://api.test/api/v1/organizations/current/roles",
    );
  });

  it("sends PATCH when changing a member role", async () => {
    const fetchMock = vi.fn<typeof fetch>(async () =>
      jsonResponse({ data: { userId: "u-1", organizationId: "org-1", roleCode: "manager" } }),
    );
    vi.stubGlobal("fetch", fetchMock);

    const client = createLaunchPadClient({ baseUrl: "http://api.test" });
    await client.updateMemberRole("u-1", { roleCode: "manager" });

    expect(fetchMock.mock.calls[0]?.[0]).toBe(
      "http://api.test/api/v1/organizations/current/members/u-1",
    );
    expect(fetchMock.mock.calls[0]?.[1]?.method).toBe("PATCH");
    expect(fetchMock.mock.calls[0]?.[1]?.body).toBe(JSON.stringify({ roleCode: "manager" }));
  });

  it("surfaces the last-owner conflict as a 409 ApiError", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async () =>
        jsonResponse(
          { error: { code: "LAST_OWNER", message: "cannot demote the last organization owner" } },
          409,
        ),
      ),
    );

    const client = createLaunchPadClient({ baseUrl: "http://api.test" });
    const error = await client
      .updateMemberRole("u-1", { roleCode: "employee" })
      .catch((e: unknown) => e);

    expect(error).toBeInstanceOf(ApiError);
    expect(error).toMatchObject({ status: 409, code: "LAST_OWNER" });
  });

  it("resolves deleteRole on a 204 with no body", async () => {
    const fetchMock = vi.fn<typeof fetch>(async () => new Response(null, { status: 204 }));
    vi.stubGlobal("fetch", fetchMock);

    const client = createLaunchPadClient({ baseUrl: "http://api.test" });
    await expect(client.deleteRole("role-1")).resolves.toBeUndefined();
    expect(fetchMock.mock.calls[0]?.[1]?.method).toBe("DELETE");
  });

  it("maps deleteRole error envelopes to ApiError", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async () =>
        jsonResponse({ error: { code: "BUILTIN_ROLE", message: "built-in roles cannot be modified" } }, 400),
      ),
    );

    const client = createLaunchPadClient({ baseUrl: "http://api.test" });
    const error = await client.deleteRole("role-1").catch((e: unknown) => e);

    expect(error).toBeInstanceOf(ApiError);
    expect(error).toMatchObject({ status: 400, code: "BUILTIN_ROLE" });
  });

  it("parses the launch-readiness report", async () => {
    const report = {
      checks: [
        { name: "mongodb", status: "ready", summary: "ping ok", action: "" },
        { name: "redis", status: "blocked", summary: "ping failed", action: "start redis" },
      ],
    };
    vi.stubGlobal("fetch", vi.fn(async () => jsonResponse({ data: report })));

    const client = createLaunchPadClient({ baseUrl: "http://api.test" });
    const readiness = await client.getLaunchReadiness();

    expect(readiness).toEqual(report);
  });
});

describe("assistant endpoints", () => {
  const answerPayload = {
    interactionId: "int-1",
    text: "Your first week covers [1].",
    citations: [
      {
        documentId: "doc-1",
        documentTitle: "Onboarding handbook",
        documentUri: "https://example.com/handbook",
        snippet: "Your first week covers orientation.",
      },
    ],
    grounded: true,
    refused: false,
  };

  it("posts the question and parses the grounded answer", async () => {
    const fetchMock = vi.fn<typeof fetch>(async () => jsonResponse({ data: answerPayload }));
    vi.stubGlobal("fetch", fetchMock);

    const client = createLaunchPadClient({ baseUrl: "http://api.test" });
    const answer = await client.askAssistant("What does week one cover?");

    expect(answer).toEqual(answerPayload);
    expect(fetchMock.mock.calls[0]?.[0]).toBe("http://api.test/api/v1/assistant/ask");
    expect(fetchMock.mock.calls[0]?.[1]?.method).toBe("POST");
    expect(fetchMock.mock.calls[0]?.[1]?.body).toBe(
      JSON.stringify({ question: "What does week one cover?" }),
    );
  });

  it("parses a refusal answer with no citations", async () => {
    const refusal = {
      interactionId: "int-2",
      text: "I don't have a reliable source for that.",
      citations: [],
      grounded: false,
      refused: true,
    };
    vi.stubGlobal("fetch", vi.fn(async () => jsonResponse({ data: refusal })));

    const client = createLaunchPadClient({ baseUrl: "http://api.test" });
    const answer = await client.askAssistant("Unanswerable?");

    expect(answer.refused).toBe(true);
    expect(answer.citations).toEqual([]);
  });

  it("resolves submitAssistantFeedback on a 204 with no body", async () => {
    const fetchMock = vi.fn<typeof fetch>(async () => new Response(null, { status: 204 }));
    vi.stubGlobal("fetch", fetchMock);

    const client = createLaunchPadClient({ baseUrl: "http://api.test" });
    await expect(
      client.submitAssistantFeedback("int-1", { helpful: true }),
    ).resolves.toBeUndefined();

    expect(fetchMock.mock.calls[0]?.[0]).toBe(
      "http://api.test/api/v1/assistant/interactions/int-1/feedback",
    );
    expect(fetchMock.mock.calls[0]?.[1]?.body).toBe(JSON.stringify({ helpful: true }));
  });
});

describe("integrations endpoints", () => {
  const connectionPayload = {
    id: "conn-1",
    provider: "github",
    status: "connected",
    accountHandle: "acme-bot",
    lastSyncAt: "2026-07-01T00:00:00Z",
    createdBy: "u-1",
    createdAt: "2026-06-01T00:00:00Z",
    updatedAt: "2026-07-01T00:00:00Z",
  };

  it("lists integration connections from the envelope", async () => {
    const fetchMock = vi.fn<typeof fetch>(async () => jsonResponse({ data: [connectionPayload] }));
    vi.stubGlobal("fetch", fetchMock);

    const client = createLaunchPadClient({ baseUrl: "http://api.test" });
    const connections = await client.listIntegrations();

    expect(connections).toEqual([connectionPayload]);
    expect(fetchMock.mock.calls[0]?.[0]).toBe("http://api.test/api/v1/integrations");
  });

  it("posts the credential when connecting a provider", async () => {
    const fetchMock = vi.fn<typeof fetch>(async () => jsonResponse({ data: connectionPayload }));
    vi.stubGlobal("fetch", fetchMock);

    const client = createLaunchPadClient({ baseUrl: "http://api.test" });
    await client.connectIntegration("github", { token: "ghp_secret" });

    expect(fetchMock.mock.calls[0]?.[0]).toBe(
      "http://api.test/api/v1/integrations/github/connect",
    );
    expect(fetchMock.mock.calls[0]?.[1]?.body).toBe(JSON.stringify({ token: "ghp_secret" }));
  });

  it("surfaces a rejected credential as INVALID_CREDENTIAL", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async () =>
        jsonResponse(
          { error: { code: "INVALID_CREDENTIAL", message: "credential rejected by provider" } },
          400,
        ),
      ),
    );

    const client = createLaunchPadClient({ baseUrl: "http://api.test" });
    const error = await client
      .connectIntegration("jira", { token: "bad", baseUrl: "https://acme.atlassian.net", email: "a@b.co" })
      .catch((e: unknown) => e);

    expect(error).toBeInstanceOf(ApiError);
    expect(error).toMatchObject({ status: 400, code: "INVALID_CREDENTIAL" });
  });

  it("resolves disconnectIntegration on a 204 with no body", async () => {
    const fetchMock = vi.fn<typeof fetch>(async () => new Response(null, { status: 204 }));
    vi.stubGlobal("fetch", fetchMock);

    const client = createLaunchPadClient({ baseUrl: "http://api.test" });
    await expect(client.disconnectIntegration("jira")).resolves.toBeUndefined();

    expect(fetchMock.mock.calls[0]?.[0]).toBe(
      "http://api.test/api/v1/integrations/jira/connect",
    );
    expect(fetchMock.mock.calls[0]?.[1]?.method).toBe("DELETE");
  });

  it("re-checks integration health with a POST", async () => {
    const fetchMock = vi.fn<typeof fetch>(async () => jsonResponse({ data: connectionPayload }));
    vi.stubGlobal("fetch", fetchMock);

    const client = createLaunchPadClient({ baseUrl: "http://api.test" });
    const connection = await client.checkIntegrationHealth("github");

    expect(connection).toEqual(connectionPayload);
    expect(fetchMock.mock.calls[0]?.[0]).toBe(
      "http://api.test/api/v1/integrations/github/health",
    );
    expect(fetchMock.mock.calls[0]?.[1]?.method).toBe("POST");
  });
});

describe("knowledge document endpoints", () => {
  const documentPayload = {
    id: "doc-1",
    organizationId: "org-1",
    title: "Benefits guide",
    source: "manual",
    body: "How to enroll in benefits.",
    accessScope: "organization",
    status: "draft",
    version: 1,
    ownerUserId: "u-1",
    createdByUserId: "u-1",
    createdAt: "2026-07-01T00:00:00Z",
    updatedAt: "2026-07-01T00:00:00Z",
  };

  it("lists knowledge documents from the envelope", async () => {
    const fetchMock = vi.fn<typeof fetch>(async () => jsonResponse({ data: [documentPayload] }));
    vi.stubGlobal("fetch", fetchMock);

    const client = createLaunchPadClient({ baseUrl: "http://api.test" });
    const documents = await client.listKnowledgeDocuments();

    expect(documents).toEqual([documentPayload]);
    expect(fetchMock.mock.calls[0]?.[0]).toBe("http://api.test/api/v1/knowledge/documents");
  });

  it("posts the create payload and parses the draft document", async () => {
    const fetchMock = vi.fn<typeof fetch>(async () => jsonResponse({ data: documentPayload }, 201));
    vi.stubGlobal("fetch", fetchMock);

    const client = createLaunchPadClient({ baseUrl: "http://api.test" });
    const document = await client.createKnowledgeDocument({
      title: "Benefits guide",
      source: "manual",
      body: "How to enroll in benefits.",
      accessScope: "organization",
    });

    expect(document).toEqual(documentPayload);
    expect(fetchMock.mock.calls[0]?.[1]?.method).toBe("POST");
    expect(fetchMock.mock.calls[0]?.[1]?.body).toBe(
      JSON.stringify({
        title: "Benefits guide",
        source: "manual",
        body: "How to enroll in benefits.",
        accessScope: "organization",
      }),
    );
  });

  it("sends PATCH when updating a draft document", async () => {
    const fetchMock = vi.fn<typeof fetch>(async () => jsonResponse({ data: documentPayload }));
    vi.stubGlobal("fetch", fetchMock);

    const client = createLaunchPadClient({ baseUrl: "http://api.test" });
    await client.updateKnowledgeDocument("doc-1", { title: "Benefits guide v2" });

    expect(fetchMock.mock.calls[0]?.[0]).toBe(
      "http://api.test/api/v1/knowledge/documents/doc-1",
    );
    expect(fetchMock.mock.calls[0]?.[1]?.method).toBe("PATCH");
    expect(fetchMock.mock.calls[0]?.[1]?.body).toBe(
      JSON.stringify({ title: "Benefits guide v2" }),
    );
  });

  it("posts to the lifecycle action endpoints", async () => {
    const fetchMock = vi.fn<typeof fetch>(async () => jsonResponse({ data: documentPayload }));
    vi.stubGlobal("fetch", fetchMock);

    const client = createLaunchPadClient({ baseUrl: "http://api.test" });
    await client.approveKnowledgeDocument("doc-1");
    await client.indexKnowledgeDocument("doc-1");
    await client.archiveKnowledgeDocument("doc-1");

    const urls = fetchMock.mock.calls.map((call) => call[0]);
    expect(urls).toEqual([
      "http://api.test/api/v1/knowledge/documents/doc-1/approve",
      "http://api.test/api/v1/knowledge/documents/doc-1/index",
      "http://api.test/api/v1/knowledge/documents/doc-1/archive",
    ]);
    for (const call of fetchMock.mock.calls) {
      expect(call[1]?.method).toBe("POST");
    }
  });

  it("maps an invalid-state conflict to ApiError", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async () =>
        jsonResponse(
          { error: { code: "INVALID_STATE", message: "document is not a draft" } },
          409,
        ),
      ),
    );

    const client = createLaunchPadClient({ baseUrl: "http://api.test" });
    const error = await client.approveKnowledgeDocument("doc-1").catch((e: unknown) => e);

    expect(error).toBeInstanceOf(ApiError);
    expect(error).toMatchObject({ status: 409, code: "INVALID_STATE" });
  });
});

describe("org support ticket endpoints", () => {
  const ticketPayload = {
    id: "t-1",
    organizationId: "org-1",
    createdByUserId: "u-1",
    subject: "Need a laptop",
    body: "No device yet.",
    priority: "high",
    category: "it",
    status: "open",
    createdAt: "2026-07-01T00:00:00Z",
    updatedAt: "2026-07-01T00:00:00Z",
  };

  it("posts subject, body, priority and category when creating a ticket", async () => {
    const fetchMock = vi.fn<typeof fetch>(async () => jsonResponse({ data: ticketPayload }, 201));
    vi.stubGlobal("fetch", fetchMock);

    const client = createLaunchPadClient({ baseUrl: "http://api.test" });
    const ticket = await client.createSupportTicket({
      subject: "Need a laptop",
      body: "No device yet.",
      priority: "high",
      category: "it",
    });

    expect(ticket).toEqual(ticketPayload);
    expect(fetchMock.mock.calls[0]?.[0]).toBe("http://api.test/api/v1/support/tickets");
    expect(fetchMock.mock.calls[0]?.[1]?.body).toBe(
      JSON.stringify({
        subject: "Need a laptop",
        body: "No device yet.",
        priority: "high",
        category: "it",
      }),
    );
  });

  it("lists org tickets from the envelope", async () => {
    const fetchMock = vi.fn<typeof fetch>(async () => jsonResponse({ data: [ticketPayload] }));
    vi.stubGlobal("fetch", fetchMock);

    const client = createLaunchPadClient({ baseUrl: "http://api.test" });
    const tickets = await client.listSupportTickets();

    expect(tickets).toEqual([ticketPayload]);
    expect(fetchMock.mock.calls[0]?.[0]).toBe("http://api.test/api/v1/support/tickets");
  });

  it("fetches a single org ticket", async () => {
    const fetchMock = vi.fn<typeof fetch>(async () => jsonResponse({ data: ticketPayload }));
    vi.stubGlobal("fetch", fetchMock);

    const client = createLaunchPadClient({ baseUrl: "http://api.test" });
    const ticket = await client.getSupportTicket("t-1");

    expect(ticket).toEqual(ticketPayload);
    expect(fetchMock.mock.calls[0]?.[0]).toBe("http://api.test/api/v1/support/tickets/t-1");
  });

  it("maps an invalid-category rejection to ApiError", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async () =>
        jsonResponse(
          { error: { code: "INVALID_INPUT", message: "Category must be one of hr, it, manager, other" } },
          400,
        ),
      ),
    );

    const client = createLaunchPadClient({ baseUrl: "http://api.test" });
    const error = await client
      .createSupportTicket({ subject: "s", body: "b", category: "other" })
      .catch((e: unknown) => e);

    expect(error).toBeInstanceOf(ApiError);
    expect(error).toMatchObject({ status: 400, code: "INVALID_INPUT" });
  });
});

describe("MFA endpoints", () => {
  const authPayload = {
    user: {
      id: "u-1",
      email: "owner@acme.test",
      displayName: "Owner",
      status: "active",
      preferences: {
        emailNotifications: true,
        inAppNotifications: true,
        digestFrequency: "daily",
        locale: "en",
        timezone: "UTC",
      },
    },
    organization: null,
    tokens: { accessToken: "a", refreshToken: "r", tokenType: "Bearer", expiresIn: 900 },
  };

  it("parses an mfaRequired login response", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async () => jsonResponse({ data: { mfaRequired: true, mfaTicket: "mfa_123" } })),
    );

    const client = createLaunchPadClient({ baseUrl: "http://api.test" });
    const result = await client.login({ email: "owner@acme.test", password: "strong-password-1" });

    expect(result).toEqual({ mfaRequired: true, mfaTicket: "mfa_123" });
  });

  it("parses a token login response", async () => {
    vi.stubGlobal("fetch", vi.fn(async () => jsonResponse({ data: authPayload })));

    const client = createLaunchPadClient({ baseUrl: "http://api.test" });
    const result = await client.login({ email: "owner@acme.test", password: "strong-password-1" });

    expect(result).toEqual(authPayload);
  });

  it("completes an MFA login", async () => {
    const fetchMock = vi.fn<typeof fetch>(async () => jsonResponse({ data: authPayload }));
    vi.stubGlobal("fetch", fetchMock);

    const client = createLaunchPadClient({ baseUrl: "http://api.test" });
    const result = await client.loginMFA({ ticket: "mfa_123", code: "123456" });

    expect(result).toEqual(authPayload);
    expect(fetchMock.mock.calls[0]?.[0]).toBe("http://api.test/api/v1/auth/login/mfa");
    expect(fetchMock.mock.calls[0]?.[1]?.body).toBe(JSON.stringify({ ticket: "mfa_123", code: "123456" }));
  });

  it("enrolls MFA and parses the one-time secret payload", async () => {
    const enrollPayload = {
      secret: "JBSWY3DPEHPK3PXP",
      otpauthUrl: "otpauth://totp/LaunchPad:owner@acme.test?secret=JBSWY3DPEHPK3PXP",
      backupCodes: ["AAAA-BBBB", "CCCC-DDDD"],
    };
    const fetchMock = vi.fn<typeof fetch>(async () => jsonResponse({ data: enrollPayload }, 201));
    vi.stubGlobal("fetch", fetchMock);

    const client = createLaunchPadClient({ baseUrl: "http://api.test" });
    const result = await client.mfaEnroll();

    expect(result).toEqual(enrollPayload);
    expect(fetchMock.mock.calls[0]?.[0]).toBe("http://api.test/api/v1/auth/mfa/enroll");
  });

  it("confirms and disables MFA with a code", async () => {
    const fetchMock = vi.fn<typeof fetch>(async () => jsonResponse({ data: { status: "ok" } }));
    vi.stubGlobal("fetch", fetchMock);

    const client = createLaunchPadClient({ baseUrl: "http://api.test" });
    await client.mfaConfirm("123456");
    await client.mfaDisable("123456");

    expect(fetchMock.mock.calls[0]?.[0]).toBe("http://api.test/api/v1/auth/mfa/confirm");
    expect(fetchMock.mock.calls[1]?.[0]).toBe("http://api.test/api/v1/auth/mfa/disable");
  });
});

describe("platform staff endpoints", () => {
  const staffPayload = {
    id: "staff-1",
    userId: "user-1",
    email: "agent@example.com",
    displayName: "Support Agent",
    roleCode: "support_agent",
    status: "active",
    createdAt: "2026-01-01T00:00:00Z",
  };

  it("lists platform staff from the envelope", async () => {
    const fetchMock = vi.fn<typeof fetch>(async () => jsonResponse({ data: [staffPayload] }));
    vi.stubGlobal("fetch", fetchMock);

    const client = createLaunchPadClient({ baseUrl: "http://api.test" });
    const staff = await client.listPlatformStaff();

    expect(staff).toEqual([staffPayload]);
    expect(fetchMock.mock.calls[0]?.[0]).toBe("http://api.test/api/v1/platform/staff");
  });

  it("creates staff and parses the one-time credential payload", async () => {
    const created = { staff: staffPayload, tempPassword: "tmp-secret", invited: false };
    const fetchMock = vi.fn<typeof fetch>(async () => jsonResponse({ data: created }, 201));
    vi.stubGlobal("fetch", fetchMock);

    const client = createLaunchPadClient({ baseUrl: "http://api.test" });
    const result = await client.createPlatformStaff({
      email: "agent@example.com",
      displayName: "Support Agent",
      roleCode: "support_agent",
    });

    expect(result).toEqual(created);
    expect(fetchMock.mock.calls[0]?.[1]?.method).toBe("POST");
    expect(fetchMock.mock.calls[0]?.[1]?.body).toBe(
      JSON.stringify({
        email: "agent@example.com",
        displayName: "Support Agent",
        roleCode: "support_agent",
      }),
    );
  });

  it("sends PATCH when changing a staff role", async () => {
    const fetchMock = vi.fn<typeof fetch>(async () => jsonResponse({ data: staffPayload }));
    vi.stubGlobal("fetch", fetchMock);

    const client = createLaunchPadClient({ baseUrl: "http://api.test" });
    await client.updatePlatformStaffRole("staff-1", { roleCode: "analyst" });

    expect(fetchMock.mock.calls[0]?.[0]).toBe(
      "http://api.test/api/v1/platform/staff/staff-1",
    );
    expect(fetchMock.mock.calls[0]?.[1]?.method).toBe("PATCH");
    expect(fetchMock.mock.calls[0]?.[1]?.body).toBe(JSON.stringify({ roleCode: "analyst" }));
  });

  it("posts to the deactivate and reactivate endpoints", async () => {
    const fetchMock = vi.fn<typeof fetch>(async () => jsonResponse({ data: staffPayload }));
    vi.stubGlobal("fetch", fetchMock);

    const client = createLaunchPadClient({ baseUrl: "http://api.test" });
    await client.deactivatePlatformStaff("staff-1");
    await client.reactivatePlatformStaff("staff-1");

    expect(fetchMock.mock.calls[0]?.[0]).toBe(
      "http://api.test/api/v1/platform/staff/staff-1/deactivate",
    );
    expect(fetchMock.mock.calls[1]?.[0]).toBe(
      "http://api.test/api/v1/platform/staff/staff-1/reactivate",
    );
    expect(fetchMock.mock.calls[0]?.[1]?.method).toBe("POST");
  });
});

describe("support sessions", () => {
  const sessionPayload = {
    id: "ss-1",
    organizationId: "org-1",
    agentUserId: "agent-1",
    reason: "Investigating ticket 12345",
    createdAt: "2026-01-01T00:00:00Z",
    expiresAt: "2026-01-01T02:00:00Z",
  };

  it("posts the organization and reason to start a session", async () => {
    const fetchMock = vi.fn<typeof fetch>(async () =>
      jsonResponse(
        { data: { session: sessionPayload, token: "tok", tokenExpiresAt: "2026-01-01T00:15:00Z" } },
        201,
      ),
    );
    vi.stubGlobal("fetch", fetchMock);

    const client = createLaunchPadClient({ baseUrl: "http://api.test" });
    const created = await client.startSupportSession("org-1", "Investigating ticket 12345");

    expect(created.token).toBe("tok");
    expect(created.session.id).toBe("ss-1");
    expect(fetchMock.mock.calls[0]?.[0]).toBe("http://api.test/api/v1/platform/support-sessions");
    expect(fetchMock.mock.calls[0]?.[1]?.method).toBe("POST");
    expect(fetchMock.mock.calls[0]?.[1]?.body).toBe(
      JSON.stringify({ organizationId: "org-1", reason: "Investigating ticket 12345" }),
    );
  });

  it("ends a session and lists the organization trail", async () => {
    const fetchMock = vi.fn<typeof fetch>(async (input) =>
      String(input).includes("organizationId=")
        ? jsonResponse({ data: [sessionPayload] })
        : jsonResponse({ data: sessionPayload }),
    );
    vi.stubGlobal("fetch", fetchMock);

    const client = createLaunchPadClient({ baseUrl: "http://api.test" });
    await client.endSupportSession("ss-1");
    await client.listSupportSessions("org-1");

    expect(fetchMock.mock.calls[0]?.[0]).toBe(
      "http://api.test/api/v1/platform/support-sessions/ss-1/end",
    );
    expect(fetchMock.mock.calls[1]?.[0]).toBe(
      "http://api.test/api/v1/platform/support-sessions?organizationId=org-1",
    );
  });

  it("rejects a malformed create response", async () => {
    vi.stubGlobal("fetch", vi.fn(async () => jsonResponse({ data: { session: {} } }, 201)));

    const client = createLaunchPadClient({ baseUrl: "http://api.test" });
    const error = await client.startSupportSession("org-1", "Investigating ticket 12345").catch(
      (e: unknown) => e,
    );

    expect(error).toBeInstanceOf(ApiError);
    expect(error).toMatchObject({ code: "INVALID_RESPONSE" });
  });
});
