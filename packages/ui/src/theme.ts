/**
 * Theme constants shared by the client <ThemeSwitcher> and the server layouts
 * (which inject the init script). Server-safe: no "use client" here.
 */

export const LP_THEMES = [
  { id: "neumorphic", label: "Neu", name: "Neumorphic" },
  { id: "glass", label: "Glass", name: "Glassmorphism" },
  { id: "clay", label: "Clay", name: "Claymorphism" },
] as const;

export type LpTheme = (typeof LP_THEMES)[number]["id"];

export const LP_THEME_STORAGE_KEY = "lp-theme";
export const LP_THEME_DEFAULT: LpTheme = "neumorphic";

/**
 * Blocking snippet for each app's <head>: applies the stored theme before
 * first paint so non-default users never see a flash of neumorphism.
 * Usage: <script dangerouslySetInnerHTML={{ __html: lpThemeInitScript }} />
 */
export const lpThemeInitScript = `try{var t=localStorage.getItem("${LP_THEME_STORAGE_KEY}");if(t)document.documentElement.dataset.lpTheme=t}catch(e){}`;
