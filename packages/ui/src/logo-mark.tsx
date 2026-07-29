import type { SVGProps } from "react";

/**
 * LogoMark is the LaunchPad mark: a launch arrow lifting off a pad baseline.
 * Renders in currentColor — pair with text color or drop it into <LogoTile>.
 */
export function LogoMark(props: SVGProps<SVGSVGElement>) {
  return (
    <svg
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth={2.2}
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
      {...props}
    >
      <path d="M5.5 18.5h13" />
      <path d="M7.5 14.5 16.5 5.5" />
      <path d="M11.5 5.5h5v5" />
    </svg>
  );
}
