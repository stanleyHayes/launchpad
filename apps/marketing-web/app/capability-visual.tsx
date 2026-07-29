import { Icon, type IconName } from "./ui-icon";

/**
 * CapabilityVisual is the hand-built product mock shown beside each capability
 * deep-dive on the product page (a designed artifact, not a screenshot — and
 * not a loading skeleton). One variant per capability icon.
 */
export function CapabilityVisual({ icon }: { icon: IconName }) {
  if (icon === "workflow") {
    return (
      <div className="lp-card p-6" aria-hidden="true">
        <div className="flex items-center justify-between gap-3">
          <p className="text-xs font-semibold uppercase tracking-wide text-[var(--lp-ink-muted)]">
            Engineering onboarding
          </p>
          <span className="rounded-md bg-[var(--lp-brand-soft)] px-2 py-1 text-[0.65rem] font-bold uppercase tracking-wide text-[var(--lp-brand)]">
            v3 · published
          </span>
        </div>
        <ul className="mt-4 space-y-2">
          {[
            { label: "Sign paperwork & policies", state: "done", meta: "Day 1" },
            { label: "Provision laptop & accounts", state: "done", meta: "Day 2" },
            { label: "Role training · Week 1", state: "active", meta: "Due Fri" },
          ].map((step) => (
            <li
              key={step.label}
              className="flex items-center gap-3 rounded-[var(--lp-radius-input)] bg-[var(--lp-paper)] px-3 py-2.5 shadow-[var(--lp-shadow-inset)]"
            >
              <span
                className={
                  step.state === "done"
                    ? "grid h-5 w-5 shrink-0 place-items-center rounded-full bg-[var(--lp-success)] text-white"
                    : "grid h-5 w-5 shrink-0 place-items-center rounded-full border-2 border-[var(--lp-brand)] text-transparent"
                }
              >
                <Icon name="check" className="h-3 w-3" />
              </span>
              <span
                className={
                  step.state === "done"
                    ? "text-sm text-[var(--lp-ink-muted)] line-through"
                    : "text-sm font-medium text-[var(--lp-ink)]"
                }
              >
                {step.label}
              </span>
              <span className="ml-auto text-xs text-[var(--lp-ink-muted)]">{step.meta}</span>
            </li>
          ))}
        </ul>
      </div>
    );
  }

  if (icon === "eye") {
    return (
      <div className="lp-card p-6" aria-hidden="true">
        <p className="text-xs font-semibold uppercase tracking-wide text-[var(--lp-ink-muted)]">
          Pending approval
        </p>
        <div className="mt-3 rounded-[var(--lp-radius-input)] bg-[var(--lp-paper)] p-4 shadow-[var(--lp-shadow-inset)]">
          <p className="text-sm font-semibold text-[var(--lp-ink)]">
            Provision laptop & accounts
          </p>
          <p className="mt-1 text-xs text-[var(--lp-ink-muted)]">
            Priya Shah · Engineering · due tomorrow
          </p>
        </div>
        <div className="mt-4 flex gap-3">
          <span className="lp-btn lp-btn--primary px-4 py-2 text-xs">Approve</span>
          <span className="lp-btn lp-btn--secondary px-4 py-2 text-xs">Reject</span>
        </div>
      </div>
    );
  }

  return (
    <div className="lp-card p-6" aria-hidden="true">
      <div className="ml-auto w-fit max-w-[85%] rounded-[var(--lp-radius-input)] bg-[var(--lp-brand)] px-3.5 py-2.5 text-sm text-white">
        How do I enrol in benefits?
      </div>
      <div className="mt-3 rounded-[var(--lp-radius-input)] bg-[var(--lp-paper)] p-4 shadow-[var(--lp-shadow-inset)]">
        <p className="text-sm leading-6 text-[var(--lp-ink)]">
          Benefits enrolment opens in your first week. HR sends the link on day
          three, and you have 14 days to complete it.
        </p>
        <p className="mt-3 flex items-center gap-2 text-xs font-medium text-[var(--lp-brand)]">
          <Icon name="check" className="h-3.5 w-3.5" />
          benefits-guide.pdf · page 4
        </p>
      </div>
    </div>
  );
}
