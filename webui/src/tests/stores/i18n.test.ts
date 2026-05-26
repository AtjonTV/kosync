//
// File:        webui/src/tests/stores/i18n.test.ts
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useI18nStore } from '@/stores/i18n.ts'

describe('useI18nStore', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    localStorage.clear()
  })

  afterEach(() => {
    vi.unstubAllGlobals()
  })

  describe('initialization and pre-selection', () => {
    it('defaults to en when navigator language is English', () => {
      vi.stubGlobal('navigator', { language: 'en-US' })
      const store = useI18nStore()
      expect(store.locale).toBe('en')
    })

    it('pre-selects de when navigator language starts with de', () => {
      vi.stubGlobal('navigator', { language: 'de-DE' })
      const store = useI18nStore()
      expect(store.locale).toBe('de')
    })

    it('restores locale from localStorage if available', () => {
      localStorage.setItem('locale', 'de')
      vi.stubGlobal('navigator', { language: 'en-US' })
      const store = useI18nStore()
      expect(store.locale).toBe('de')
    })
  })

  describe('locale switching and persistence', () => {
    it('allows changing the locale', () => {
      const store = useI18nStore()
      store.setLocale('de')
      expect(store.locale).toBe('de')
      expect(localStorage.getItem('locale')).toBe('de')
    })
  })

  describe('translation and formatting', () => {
    it('returns key itself if translation is missing in both active and fallback language', () => {
      const store = useI18nStore()
      expect(store.t('non_existent_key')).toBe('non_existent_key')
    })

    it('returns the English translation if key exists in English but active is German and missing', () => {
      const store = useI18nStore()
      store.setLocale('de')
      expect(store.t('title')).toBe('KOsync')
    })

    it('translates known keys in English', () => {
      const store = useI18nStore()
      store.setLocale('en')
      expect(store.t('total_books')).toBe('Total Books')
    })

    it('translates known keys in German', () => {
      const store = useI18nStore()
      store.setLocale('de')
      expect(store.t('total_books')).toBe('Bücher insgesamt')
    })

    it('interpolates parameters using %s placeholder', () => {
      const store = useI18nStore()
      expect(store.t('Hello %s!', 'World')).toBe('Hello World!')
    })

    it('interpolates parameters using {0} placeholder', () => {
      const store = useI18nStore()
      expect(store.t('Hello {0} and {1}!', 'Alice', 'Bob')).toBe('Hello Alice and Bob!')
    })
  })
})
