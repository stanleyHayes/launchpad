import { Icon, type IconName } from "./icon";

export type MetricCardProps = {
  label: string;
  value: string | number;
  hint?: string;
  icon?: IconName;
  /** Accent color for the left border, gradient tint, icon chip, and watermark. */
  accent?: string;
  href?: string;
  /** Signed delta in percent; renders a trend chip instead of the link arrow. */
  trend?: number;
};

/**
 * KPI stat card (RentOS pattern): 3px accent left border, accent-tinted
 * diagonal gradient, icon chip, and the icon repeated as a faint watermark
 * bleeding off the bottom-right corner. Wraps in a link when href is given.
 */
export function MetricCard({
  label,
  value,
  hint,
  icon,
  accent = "var(--lp-brand)",
  href,
  trend,
}: MetricCardProps) {
  const resolvedIcon = icon ?? "chart";
  const card = (
    <div
      className="lp-card group relative min-h-44 h-full overflow-hidden border-l-[3px] p-6"
      style={{
        borderLeftColor: accent,
        background: `linear-gradient(135deg, color-mix(in srgb, ${accent} 8%, transparent), transparent 58%)`,
      }}
    >
      <span
        aria-hidden="true"
        className="pointer-events-none absolute -bottom-5 -right-4 size-28 select-none opacity-[0.11] transition-transform duration-300 group-hover:scale-105"
        style={{ color: accent }}
      >
        <Icon name={resolvedIcon} style={{ width: "100%", height: "100%" }} />
      </span>

      <div className="relative">
        <div className="flex items-start justify-between gap-2">
          <span
            className="grid h-10 w-10 place-items-center rounded-[11px] transition-transform group-hover:scale-105"
            style={{
              background: `color-mix(in srgb, ${accent} 12%, transparent)`,
              color: accent,
            }}
          >
            <Icon name={resolvedIcon} className="h-5 w-5" />
          </span>
          {trend !== undefined && trend !== 0 ? (
            <span
              className="flex items-center gap-0.5 text-[10px] font-bold"
              style={{ color: trend > 0 ? "var(--lp-success)" : "var(--lp-danger)" }}
            >
              {trend > 0 ? "▲" : "▼"} {trend > 0 ? "+" : ""}
              {trend}%
            </span>
          ) : href ? (
            <Icon
              name="arrow-right"
              className="h-4 w-4 text-[var(--lp-ink-muted)]/40 transition-colors group-hover:text-[var(--lp-brand)]"
            />
          ) : null}
        </div>
        <p
          className="mt-5 truncate text-[clamp(2.35rem,3.4vw,3.75rem)] font-semibold leading-none tracking-[-0.045em] tabular-nums text-[var(--lp-ink)]"
          style={{ fontFamily: "var(--lp-font-display)" }}
        >
          {value}
        </p>
        <p className="mt-2 truncate text-sm font-medium text-[var(--lp-ink-muted)]">{label}</p>
        {hint ? (
          <p className="mt-1.5 truncate text-[11px] font-semibold" style={{ color: accent }}>
            {hint}
          </p>
        ) : null}
      </div>
    </div>
  );

  if (href) {
    return (
      <a href={href} className="block h-full" style={{ textDecoration: "none" }}>
        {card}
      </a>
    );
  }

  return card;
}
