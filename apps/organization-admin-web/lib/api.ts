"use client";

import { createLaunchPadClient } from "@launchpad/api-client";
import { getAccessToken } from "@/lib/session";

const apiBaseUrl =
  process.env.NEXT_PUBLIC_API_BASE_URL ?? "http://localhost:8080";

export function getClient() {
  return createLaunchPadClient({
    baseUrl: apiBaseUrl,
    getAccessToken,
  });
}
