import { createI18n } from 'vue-i18n'

import en from './locales/en'
import es from './locales/es'

export type SupportedLocale = 'es' | 'en'

export const defaultLocale: SupportedLocale = 'es'

export const i18n = createI18n({
  legacy: false,
  globalInjection: true,
  locale: defaultLocale,
  fallbackLocale: defaultLocale,
  messages: { es, en },
})

export function normalizeLocale(value?: string | null): SupportedLocale {
  return value === 'en' ? 'en' : 'es'
}

export function setLocale(value?: string | null): SupportedLocale {
  const locale = normalizeLocale(value)
  i18n.global.locale.value = locale
  document.documentElement.lang = locale
  return locale
}

export function dateLocale(locale?: string | null): string {
  return normalizeLocale(locale) === 'en' ? 'en-US' : 'es-CO'
}
