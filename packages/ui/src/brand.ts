import type { CSSProperties } from "react";

/**
 * OrgBranding is an organization's optional brand overrides, mirroring the
 * `branding` object returned by the API on the current organization.
 */
export interface OrgBranding {
  primaryColor?: string;
  primaryHoverColor?: string;
  accentColor?: string;
  logoUrl?: string;
}

const HEX_COLOR = /^#[0-9a-fA-F]{6}$/;

function isHexColor(value: string | undefined): value is string {
  return typeof value === "string" && HEX_COLOR.test(value);
}

/**
 * brandCssVars maps an organization's brand overrides onto the overridable
 * LaunchPad brand tokens. Apply the result as an inline `style` on a wrapper
 * element at the tenant boundary; it overrides the `:root` defaults from
 * styles.css for that subtree only, leaving the fixed neutral and semantic
 * tokens (text contrast, success/warning/danger) untouched.
 *
 * Only well-formed `#RRGGBB` values are applied, so a malformed stored value
 * can never break the theme — it simply falls back to the Ocean Depths default.
 *
 * @example
 * <div style={brandCssVars(org.branding)}>{children}</div>
 */
export function brandCssVars(branding?: OrgBranding | null): CSSProperties {
  const vars: Record<string, string> = {};

  if (!branding) {
    return vars as CSSProperties;
  }

  if (isHexColor(branding.primaryColor)) {
    vars["--lp-brand"] = branding.primaryColor;
  }

  if (isHexColor(branding.primaryHoverColor)) {
    vars["--lp-brand-strong"] = branding.primaryHoverColor;
  }

  if (isHexColor(branding.accentColor)) {
    vars["--lp-signal"] = branding.accentColor;
  }

  return vars as CSSProperties;
}
