import { useCallback, useEffect, useState } from "react";

export type Theme = "light" | "dark";
const KEY = "sentinel-theme";

/** The saved choice, or the system preference on a first visit. */
function initial(): Theme {
  try {
    const saved = localStorage.getItem(KEY);
    if (saved === "light" || saved === "dark") return saved;
  } catch {
    // storage can be blocked; fall through to the system preference
  }
  return window.matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light";
}

/** Set before the first render so the page never flashes the wrong theme. */
export function applyInitialTheme(): void {
  document.documentElement.dataset.theme = initial();
}

export function useTheme(): [Theme, () => void] {
  const [theme, setTheme] = useState<Theme>(
    () => (document.documentElement.dataset.theme as Theme | undefined) ?? initial(),
  );
  useEffect(() => {
    document.documentElement.dataset.theme = theme;
    try {
      localStorage.setItem(KEY, theme);
    } catch {
      // not persisting is fine
    }
  }, [theme]);
  const toggle = useCallback(() => setTheme((t) => (t === "dark" ? "light" : "dark")), []);
  return [theme, toggle];
}
