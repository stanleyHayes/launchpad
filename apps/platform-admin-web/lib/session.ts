"use client";

import { createSessionStorage } from "@launchpad/api-client";

// Session tokens live in HttpOnly cookies set by the API; only a
// non-sensitive "signed in" flag is kept in browser storage.
const storage = createSessionStorage("lp.platform");

export const { saveSession, clearSession, getAccessToken } = storage;
