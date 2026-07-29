"use client";

import { useState } from "react";
import { Icon, type IconName } from "./icon";

/**
 * FormField is the standard labeled input: a start icon for scannability,
 * and — for password fields — an end-icon visibility toggle. Styling comes
 * from the theme-aware `.lp-input`/`.lp-field-*` classes, so fields follow
 * the active design system.
 */
export function FormField({
  label,
  name,
  type = "text",
  required,
  minLength,
  startIcon,
  autoComplete,
  placeholder,
}: {
  label: string;
  name: string;
  type?: string;
  required?: boolean;
  minLength?: number;
  startIcon?: IconName;
  autoComplete?: string;
  placeholder?: string;
}) {
  const [showPassword, setShowPassword] = useState(false);
  const isPassword = type === "password";

  return (
    <label className="block text-sm font-medium text-[var(--lp-ink)]">
      {label}
      <span className="lp-field mt-1.5 block">
        {startIcon ? (
          <span className="lp-field-icon" aria-hidden="true">
            <Icon name={startIcon} className="h-5 w-5" />
          </span>
        ) : null}
        <input
          className={`lp-input${startIcon ? " lp-input--with-start" : ""}${isPassword ? " lp-input--with-end" : ""}`}
          name={name}
          type={isPassword && showPassword ? "text" : type}
          required={required}
          minLength={minLength}
          autoComplete={autoComplete ?? (isPassword ? "new-password" : "on")}
          placeholder={placeholder}
        />
        {isPassword ? (
          <button
            type="button"
            className="lp-field-icon lp-field-icon--end lp-field-toggle"
            onClick={() => {
              setShowPassword((value) => !value);
            }}
            aria-label={showPassword ? "Hide password" : "Show password"}
            aria-pressed={showPassword}
          >
            <Icon name={showPassword ? "eye-off" : "eye"} className="h-5 w-5" />
          </button>
        ) : null}
      </span>
    </label>
  );
}
