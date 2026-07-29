import { LogoMark } from "./logo-mark";

/**
 * LogoTile is the brand lockup used in navbars, sidebars, and auth screens:
 * the white mark on the brand-gradient rounded tile.
 */
export function LogoTile({
  size = 32,
  className = "",
}: {
  size?: number;
  className?: string;
}) {
  return (
    <span
      className={`grid shrink-0 place-items-center text-white${className ? ` ${className}` : ""}`}
      style={{
        width: size,
        height: size,
        borderRadius: Math.round(size * 0.28),
        background: "var(--lp-cta-gradient)",
      }}
      aria-hidden="true"
    >
      <LogoMark style={{ width: size * 0.64, height: size * 0.64 }} />
    </span>
  );
}
