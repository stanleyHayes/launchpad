import { Icon, type IconName } from "./icon";

/**
 * IconWatermark places a giant, faint version of an icon as decorative
 * background (the RentOS watermark pattern). Position, size, and rotation are
 * caller-controlled via className (e.g. "-right-8 -bottom-8 size-56
 * rotate-[-8deg]"); the parent needs `relative overflow-hidden`. Use onDark on
 * dark hero/story surfaces.
 */
export function IconWatermark({
  icon,
  onDark = false,
  className = "",
}: {
  icon: IconName;
  onDark?: boolean;
  className?: string;
}) {
  return (
    <span
      className={`lp-watermark${onDark ? " lp-watermark--on-dark" : ""}${className ? ` ${className}` : ""}`}
      aria-hidden="true"
    >
      <Icon name={icon} />
    </span>
  );
}
