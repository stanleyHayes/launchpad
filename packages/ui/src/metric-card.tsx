export function MetricCard({
  label,
  value,
  hint,
}: {
  label: string;
  value: string | number;
  hint?: string;
}) {
  return (
    <div className="lp-card relative overflow-hidden border-l-[3px] border-l-[var(--lp-accent)] p-5">
      <p className="text-sm text-[var(--lp-ink-muted)]">{label}</p>
      <p
        className="mt-2 text-3xl font-semibold tabular-nums text-[var(--lp-ink)]"
        style={{ fontFamily: "var(--lp-font-display)" }}
      >
        {value}
      </p>
      {hint ? <p className="mt-2 text-xs text-[var(--lp-ink-muted)]">{hint}</p> : null}
    </div>
  );
}
