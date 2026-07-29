"use client";

/**
 * ToggleSwitch — the RentOS settings toggle: a pill track with a sliding
 * thumb, driven by `checked`/`onChange`. Theme-aware via tokens.
 */
export function ToggleSwitch({
  checked,
  onChange,
  label,
}: {
  checked: boolean;
  onChange: (next: boolean) => void;
  label: string;
}) {
  return (
    <button
      type="button"
      role="switch"
      aria-checked={checked}
      aria-label={label}
      style={{ width: 44, height: 24, flex: "0 0 44px" }}
      onClick={() => {
        onChange(!checked);
      }}
      className={`relative h-6 w-11 shrink-0 rounded-full transition-colors ${
        checked ? "bg-[var(--lp-brand)]" : "bg-[var(--lp-border)]"
      }`}
    >
      <span
        className="absolute left-0 top-0.5 rounded-full bg-white shadow transition-transform"
        style={{
          width: 20,
          height: 20,
          transform: checked ? "translateX(22px)" : "translateX(2px)",
        }}
      />
    </button>
  );
}
