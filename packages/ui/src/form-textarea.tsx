import { Icon, type IconName } from "./icon";

/**
 * FormTextArea — the multi-line companion to FormField. The start icon aligns
 * with the first line of text instead of the vertical middle.
 */
export function FormTextArea({
  label,
  name,
  required,
  startIcon,
  rows = 4,
  placeholder,
}: {
  label: string;
  name: string;
  required?: boolean;
  startIcon?: IconName;
  rows?: number;
  placeholder?: string;
}) {
  return (
    <label className="block text-sm font-medium text-[var(--lp-ink)]">
      {label}
      <span className="lp-field mt-1.5 block">
        {startIcon ? (
          <span className="lp-field-icon lp-field-icon--area" aria-hidden="true">
            <Icon name={startIcon} className="h-5 w-5" />
          </span>
        ) : null}
        <textarea
          className={`lp-input lp-input--area${startIcon ? " lp-input--with-start" : ""}`}
          name={name}
          required={required}
          rows={rows}
          placeholder={placeholder}
        />
      </span>
    </label>
  );
}
