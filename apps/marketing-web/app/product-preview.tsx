import { Icon } from "./ui-icon";

const steps = [
  { label: "Sign paperwork & policies", done: true },
  { label: "Provision laptop & accounts", done: true },
  { label: "Role training · Week 1", done: false },
  { label: "Meet your onboarding buddy", done: false },
];

/**
 * ProductPreview is a decorative mock of the onboarding workspace shown beside
 * the hero copy — a browser-style card with a progress ring, a checklist, and a
 * stat chip. Its surfaces use the theme tokens (.lp-card / .lp-preview-well /
 * .lp-preview-row), so the mock itself demonstrates the active design system.
 */
export function ProductPreview() {
  return (
    <div className="lp-card lp-dark-scope lp-preview relative w-full p-4">
      {/* Window chrome */}
      <div className="flex items-center gap-1.5 pb-3">
        <span className="h-2.5 w-2.5 rounded-full bg-[#e5573f]" />
        <span className="h-2.5 w-2.5 rounded-full bg-[#e6b23f]" />
        <span className="h-2.5 w-2.5 rounded-full bg-[#3fbf7f]" />
        <span className="ml-3 text-xs font-medium text-[var(--lp-ink-muted)]">
          launchpad · Priya&apos;s onboarding
        </span>
      </div>

      <div className="lp-preview-well p-4">
        {/* Progress header */}
        <div className="lp-preview-row flex items-center justify-between p-4">
          <div>
            <p className="text-xs font-medium uppercase tracking-wide text-[var(--lp-ink-muted)]">
              Day 3 of 30
            </p>
            <p
              className="mt-1 text-lg font-semibold text-[var(--lp-ink)]"
              style={{ fontFamily: "var(--lp-font-display)" }}
            >
              Onboarding progress
            </p>
          </div>
          <div
            className="grid h-14 w-14 place-items-center rounded-full text-sm font-semibold text-[var(--lp-brand)]"
            style={{
              background:
                "conic-gradient(var(--lp-brand) 0% 62%, var(--lp-brand-soft) 62% 100%)",
            }}
          >
            <span className="grid h-10 w-10 place-items-center rounded-full bg-[var(--lp-paper-elevated)]">
              62%
            </span>
          </div>
        </div>

        {/* Checklist */}
        <ul className="mt-3 space-y-2">
          {steps.map((step) => (
            <li
              key={step.label}
              className="lp-preview-row flex items-center gap-3 px-3 py-2.5"
            >
              <span
                className={
                  step.done
                    ? "grid h-5 w-5 place-items-center rounded-full bg-[var(--lp-success)] text-white"
                    : "grid h-5 w-5 place-items-center rounded-full border border-[var(--lp-border)] text-transparent"
                }
              >
                <Icon name="check" className="h-3.5 w-3.5" />
              </span>
              <span
                className={
                  step.done
                    ? "text-sm text-[var(--lp-ink-muted)] line-through"
                    : "text-sm font-medium text-[var(--lp-ink)]"
                }
              >
                {step.label}
              </span>
            </li>
          ))}
        </ul>
      </div>

      {/* Floating stat chip */}
      <div className="lp-card absolute -bottom-5 -left-5 hidden items-center gap-3 px-4 py-3 sm:flex">
        <span className="lp-icon-chip lp-icon-chip--sm">
          <Icon name="clock" className="h-5 w-5" />
        </span>
        <div>
          <p className="text-sm font-semibold text-[var(--lp-ink)]">-41% ramp time</p>
          <p className="text-xs text-[var(--lp-ink-muted)]">vs. last quarter</p>
        </div>
      </div>
    </div>
  );
}
