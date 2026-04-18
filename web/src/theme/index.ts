import { createContext, useContext } from 'react';

export type Theme = 'new' | 'classic';
const STORAGE_KEY = 'arb-theme';

export function getStoredTheme(): Theme {
  const stored = localStorage.getItem(STORAGE_KEY);
  return stored === 'new' ? 'new' : 'classic'; // default 'classic'
}

export function storeTheme(theme: Theme) {
  localStorage.setItem(STORAGE_KEY, theme);
}

export const ThemeContext = createContext<{ theme: Theme; setTheme: (t: Theme) => void }>({
  theme: 'classic',
  setTheme: () => {},
});

export function useTheme() {
  return useContext(ThemeContext);
}
