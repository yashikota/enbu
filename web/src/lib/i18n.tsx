import { createContext, useContext, useMemo, useState, type ReactNode } from "react";
import type { Messages } from "../locales/lang";
import { en } from "../locales/en";
import { ja } from "../locales/ja";

const dictionaries = { en, ja };
export type Locale = keyof typeof dictionaries;

type TranslationParams = Record<string, string | number>;
type I18nContextValue = {
  locale: Locale;
  setLocale: (locale: Locale) => void;
  t: (key: MessageKey, params?: TranslationParams) => string;
};

type NestedKeys<T> = {
  [K in keyof T]: T[K] extends string ? K & string : `${K & string}.${NestedKeys<T[K]>}`;
}[keyof T];

export type MessageKey = NestedKeys<Messages>;

const I18nContext = createContext<I18nContextValue | null>(null);

export function I18nProvider({
  initialLocale,
  children,
}: {
  initialLocale?: Locale;
  children: ReactNode;
}) {
  const [locale, setLocaleState] = useState<Locale>(() => initialLocale ?? detectLocale());

  const value = useMemo<I18nContextValue>(
    () => ({
      locale,
      setLocale: (next) => {
        localStorage.setItem("enbu_locale", next);
        setLocaleState(next);
      },
      t: (key, params) => translate(locale, key, params),
    }),
    [locale],
  );

  return <I18nContext.Provider value={value}>{children}</I18nContext.Provider>;
}

export function useI18n() {
  const ctx = useContext(I18nContext);
  if (!ctx) {
    throw new Error("useI18n must be used inside I18nProvider");
  }
  return ctx;
}

export function detectLocale(): Locale {
  const saved = typeof localStorage !== "undefined" ? localStorage.getItem("enbu_locale") : null;
  if (isLocale(saved)) {
    return saved;
  }
  const language = navigator.language.toLowerCase();
  if (language.startsWith("ja")) {
    return "ja";
  }
  return "en";
}

export function translate(locale: Locale, key: MessageKey, params: TranslationParams = {}) {
  const template = getMessage(dictionaries[locale], key) ?? getMessage(en, key) ?? key;
  return Object.entries(params).reduce(
    (text, [name, value]) => text.replaceAll(`{${name}}`, String(value)),
    template,
  );
}

function getMessage(messages: Messages, key: MessageKey): string | undefined {
  return key.split(".").reduce<unknown>((value, part) => {
    if (value && typeof value === "object" && part in value) {
      return (value as Record<string, unknown>)[part];
    }
    return undefined;
  }, messages) as string | undefined;
}

function isLocale(value: string | null): value is Locale {
  return value !== null && value in dictionaries;
}
