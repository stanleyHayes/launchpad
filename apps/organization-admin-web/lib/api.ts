"use client";

import { createLaunchPadClient } from "@launchpad/api-client";
import { apiBaseUrl } from "./env";

// Singleton so the client's single-flight token refresh is shared app-wide.
const client = createLaunchPadClient({ baseUrl: apiBaseUrl });

export function getClient() {
  return client;
}
