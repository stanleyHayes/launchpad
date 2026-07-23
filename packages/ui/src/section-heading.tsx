export function SectionHeading({
  title,
  description,
}: {
  title: string;
  description?: string;
}) {
  return (
    <div className="max-w-2xl">
      <h2
        className="text-3xl font-semibold tracking-tight text-[var(--lp-ink)]"
        style={{ fontFamily: "var(--lp-font-display)" }}
      >
        {title}
      </h2>
      {description ? (
        <p className="mt-3 text-base text-[var(--lp-ink-muted)]">{description}</p>
      ) : null}
    </div>
  );
}
