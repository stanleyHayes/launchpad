"use client";

import { useEffect, useState } from "react";
import { LP_THEME_DEFAULT, LP_THEME_STORAGE_KEY, LP_THEMES, type LpTheme } from "./theme";

function readStoredTheme(): LpTheme {
  try {
    const stored = window.localStorage.getItem(LP_THEME_STORAGE_KEY);
    if (stored === "glass" || stored === "clay" || stored === "neumorphic") {
      return stored;
    }
  } catch {
    // Storage unavailable (private mode) — fall through to default.
  }
  return LP_THEME_DEFAULT;
}

/**
 * Segmented control that lets the user pick the active design system
 * (neumorphic default, glass, clay). The control is styled by the theme's own
 * surface/shadow tokens, so it demos the current system in place.
 */
export function ThemeSwitcher({
  onDark = false,
  className = "",
}: {
  onDark?: boolean;
  className?: string;
}) {
  const [theme, setTheme] = useState<LpTheme>(LP_THEME_DEFAULT);

  useEffect(() => {
    setTheme(readStoredTheme());
  }, []);

  function choose(next: LpTheme) {
    setTheme(next);
    try {
      window.localStorage.setItem(LP_THEME_STORAGE_KEY, next);
    } catch {
      // Non-persistent is fine — the attribute still applies for this session.
    }
    if (next === LP_THEME_DEFAULT) {
      delete document.documentElement.dataset.lpTheme;
    } else {
      document.documentElement.dataset.lpTheme = next;
    }
  }

  return (
    <div
      role="group"
      aria-label="Design system"
      className={`lp-theme-switch${onDark ? " lp-theme-switch--on-dark" : ""}${className ? ` ${className}` : ""}`}
    >
      {LP_THEMES.map((option) => (
        <button
          key={option.id}
          type="button"
          title={option.name}
          aria-pressed={theme === option.id}
          onClick={() => {
            choose(option.id);
          }}
        >
          {option.label}
        </button>
      ))}
    </div>
  );
}
