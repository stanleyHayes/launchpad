"use client";

import { useEffect, useState } from "react";
import { LP_THEME_DEFAULT, LP_THEME_STORAGE_KEY, LP_THEMES, type LpTheme } from "./theme";

function readStored(): LpTheme {
  try {
    const stored = window.localStorage.getItem(LP_THEME_STORAGE_KEY);
    if (stored === "glass" || stored === "clay" || stored === "neumorphic") {
      return stored;
    }
  } catch {
    // Fall through to default.
  }
  return LP_THEME_DEFAULT;
}

/**
 * ThemeTiles renders the three design systems as large selectable cards
 * (the RentOS appearance-tab pattern). Selection persists exactly like the
 * compact ThemeSwitcher — same storage key, same data attribute.
 */
export function ThemeTiles() {
  const [theme, setTheme] = useState<LpTheme>(LP_THEME_DEFAULT);

  useEffect(() => {
    setTheme(readStored());
  }, []);

  function choose(next: LpTheme) {
    setTheme(next);
    try {
      window.localStorage.setItem(LP_THEME_STORAGE_KEY, next);
    } catch {
      // Non-persistent is fine.
    }
    if (next === LP_THEME_DEFAULT) {
      delete document.documentElement.dataset.lpTheme;
    } else {
      document.documentElement.dataset.lpTheme = next;
    }
  }

  return (
    <div className="grid gap-3 sm:grid-cols-3">
      {LP_THEMES.map((option) => {
        const selected = theme === option.id;
        return (
          <button
            key={option.id}
            type="button"
            onClick={() => {
              choose(option.id);
            }}
            aria-pressed={selected}
            className={`lp-card relative p-4 text-left transition ${
              selected ? "ring-2 ring-[var(--lp-brand)]" : ""
            }`}
          >
            {selected ? (
              <span className="absolute right-3 top-3 grid h-5 w-5 place-items-center rounded-full bg-[var(--lp-brand)] text-[0.6rem] font-bold text-white">
                ✓
              </span>
            ) : null}
            <span className="block text-sm font-semibold text-[var(--lp-ink)]">
              {option.name}
            </span>
            <span className="mt-1 block text-xs text-[var(--lp-ink-muted)]">
              {option.id === "neumorphic"
                ? "Soft extruded surfaces, dual shadows"
                : option.id === "glass"
                  ? "Dark, frosted, translucent"
                  : "Warm, puffy, playful"}
            </span>
          </button>
        );
      })}
    </div>
  );
}
