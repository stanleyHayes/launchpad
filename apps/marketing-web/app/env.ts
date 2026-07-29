import { z } from "zod";

const publicUrlSchema = z.url();

/**
 * Validates a NEXT_PUBLIC_* URL at module load. A missing value falls back to
 * the local default only in development and during `next build` (NEXT_PHASE),
 * where inlined values are baked into the bundle; at production runtime a
 * missing value throws instead of silently pointing at localhost.
 */
function requirePublicUrl(name: string, value: string | undefined, devFallback: string): string {
  if (value) {
    return publicUrlSchema.parse(value);
  }
  if (
    process.env.NODE_ENV === "development" ||
    process.env.NEXT_PHASE === "phase-production-build"
  ) {
    return devFallback;
  }
  throw new Error(`${name} must be set outside development`);
}

export const apiBaseUrl = requirePublicUrl(
  "NEXT_PUBLIC_API_BASE_URL",
  process.env.NEXT_PUBLIC_API_BASE_URL,
  "http://localhost:8080",
);

export const orgAdminUrl = requirePublicUrl(
  "NEXT_PUBLIC_ORG_ADMIN_URL",
  process.env.NEXT_PUBLIC_ORG_ADMIN_URL,
  "http://localhost:3002",
);

export const siteUrl = requirePublicUrl(
  "NEXT_PUBLIC_MARKETING_URL",
  process.env.NEXT_PUBLIC_MARKETING_URL,
  "http://localhost:3000",
);
