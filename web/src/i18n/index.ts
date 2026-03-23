import { createContext, useContext } from 'react';
import en, { type TranslationKey } from './en.ts';
import zhTW from './zh-TW.ts';

export type Locale = 'en' | 'zh-TW';

const translations: Record<Locale, Record<TranslationKey, string>> = {
  en,
  'zh-TW': zhTW,
};

const STORAGE_KEY = 'arb-locale';

export function getStoredLocale(): Locale {
  const stored = localStorage.getItem(STORAGE_KEY);
  if (stored === 'en' || stored === 'zh-TW') return stored;
  return 'zh-TW'; // default
}

export function storeLocale(locale: Locale) {
  localStorage.setItem(STORAGE_KEY, locale);
}

export function t(key: TranslationKey, locale: Locale): string {
  return translations[locale]?.[key] ?? translations.en[key] ?? key;
}

export type TFunc = (key: TranslationKey) => string;

export const LocaleContext = createContext<{ locale: Locale; setLocale: (l: Locale) => void; t: TFunc }>({
  locale: 'zh-TW',
  setLocale: () => {},
  t: (key) => en[key] ?? key,
});

export function useLocale() {
  return useContext(LocaleContext);
}

export { type TranslationKey };
