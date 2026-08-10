'use client';

import { createContext, useContext, useEffect, useRef, useState, ReactNode } from 'react';

type Theme = 'dark' | 'light';

interface ThemeContextValue {
  theme: Theme;
  isLight: boolean;
  setTheme: (theme: Theme) => void;
  toggleTheme: () => void;
}

const STORAGE_KEY = 'ownwave:theme';
const DEFAULT_THEME: Theme = 'dark';

const ThemeContext = createContext<ThemeContextValue | undefined>(undefined);

function getInitialTheme(): Theme {
  if (typeof window === 'undefined') return DEFAULT_THEME;
  const stored = window.localStorage.getItem(STORAGE_KEY);
  if (stored === 'light' || stored === 'dark') return stored;
  return DEFAULT_THEME;
}

export function ThemeProvider({ children }: { children: ReactNode }) {
  const [theme, setThemeState] = useState<Theme>(DEFAULT_THEME);
  const initialized = useRef(false);

  useEffect(() => {
    const initial = getInitialTheme();
    document.documentElement.classList.toggle('light', initial === 'light');
    setThemeState(initial);
  }, []);

  useEffect(() => {
    if (initialized.current) {
      document.documentElement.classList.toggle('light', theme === 'light');
      window.localStorage.setItem(STORAGE_KEY, theme);
    }
  }, [theme]);

  useEffect(() => {
    initialized.current = true;
  }, []);

  const setTheme = (next: Theme) => setThemeState(next);
  const toggleTheme = () => setThemeState((t) => (t === 'light' ? 'dark' : 'light'));

  return (
    <ThemeContext.Provider value={{ theme, isLight: theme === 'light', setTheme, toggleTheme }}>
      {children}
    </ThemeContext.Provider>
  );
}

export function useTheme() {
  const ctx = useContext(ThemeContext);
  if (!ctx) throw new Error('useTheme must be used within a ThemeProvider');
  return ctx;
}
