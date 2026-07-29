import { LogoMark } from "./logo-mark";

/**
 * LogoWatermark places a giant, faint LaunchPad mark as decorative background
 * (the RentOS watermark pattern). Position, size, and rotation are
 * caller-controlled via className (e.g. "-right-24 top-16 size-[30rem]
 * rotate-[-9deg]"); the parent needs `relative overflow-hidden`. Use onDark on
 * dark hero/story surfaces.
 */
export function LogoWatermark({
  onDark = false,
  className = "",
}: {
  onDark?: boolean;
  className?: string;
}) {
  return (
    <span
      className={`lp-watermark${onDark ? " lp-watermark--on-dark" : ""}${className ? ` ${className}` : ""}`}
      aria-hidden="true"
    >
      <LogoMark />
    </span>
  );
}
