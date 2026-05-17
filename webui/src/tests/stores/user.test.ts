//
// File:        webui/src/tests/stores/user.test.ts
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2025-2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

import { describe, it, expect, vi, beforeEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useUserStore } from '@/stores/user.ts'
// Note: global beforeEach in setup.ts resets Pinia and storage before each test

// A minimal valid JWT with payload { "username": "testuser" }
const VALID_JWT = [
  'header',
  btoa(JSON.stringify({ username: 'testuser' })),
  'signature',
].join('.')

describe('useUserStore', () => {
  describe('hasToken', () => {
    it('returns false when no token is set', () => {
      const store = useUserStore()
      expect(store.hasToken()).toBe(false)
    })

    it('returns true after login', async () => {
      const store = useUserStore()
      await store.login(VALID_JWT)
      expect(store.hasToken()).toBe(true)
    })

    it('returns false after logout', async () => {
      const store = useUserStore()
      await store.login(VALID_JWT)
      store.logout()
      expect(store.hasToken()).toBe(false)
    })
  })


  describe('login', () => {
    it('sets the access token', async () => {
      const store = useUserStore()
      await store.login(VALID_JWT)
      expect(store.user.accessToken).toBe(VALID_JWT)
    })

    it('persists state to localStorage', async () => {
      const store = useUserStore()
      await store.login(VALID_JWT)
      const stored = localStorage.getItem('userState')
      expect(stored).not.toBeNull()
      const parsed = JSON.parse(atob(stored!))
      expect(parsed.accessToken).toBe(VALID_JWT)
    })

    it('returns true', async () => {
      const store = useUserStore()
      const result = await store.login(VALID_JWT)
      expect(result).toBe(true)
    })
  })

  describe('logout', () => {
    it('clears the access token', async () => {
      const store = useUserStore()
      await store.login(VALID_JWT)
      store.logout()
      expect(store.user.accessToken).toBe('')
    })

    it('removes userState from localStorage', async () => {
      const store = useUserStore()
      await store.login(VALID_JWT)
      store.logout()
      expect(localStorage.getItem('userState')).toBeNull()
    })
  })

  describe('getUsername', () => {
    it('returns null when not logged in', () => {
      const store = useUserStore()
      expect(store.getUsername()).toBeNull()
    })

    it('returns the username from the JWT payload', async () => {
      const store = useUserStore()
      await store.login(VALID_JWT)
      expect(store.getUsername()).toBe('testuser')
    })

    it('returns null for a malformed token without enough parts', async () => {
      const store = useUserStore()
      await store.login('onlyonepart')
      expect(store.getUsername()).toBeNull()
    })

    it('returns null when JWT payload has no username claim', async () => {
      const store = useUserStore()
      const tokenWithoutUsername = [
        'header',
        btoa(JSON.stringify({ sub: '123' })),
        'signature',
      ].join('.')
      await store.login(tokenWithoutUsername)
      expect(store.getUsername()).toBeNull()
    })
  })


  describe('loginWithCredentials', () => {
    it('returns false when the server responds with an error', async () => {
      vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: false }))
      const store = useUserStore()
      const result = await store.loginWithCredentials('user', 'wrongpass')
      expect(result).toBe(false)
      vi.unstubAllGlobals()
    })

    it('logs in and returns true on success', async () => {
      vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
        ok: true,
        text: async () => VALID_JWT,
      }))
      const store = useUserStore()
      const result = await store.loginWithCredentials('user', 'pass')
      expect(result).toBe(true)
      expect(store.user.accessToken).toBe(VALID_JWT)
      vi.unstubAllGlobals()
    })
  })


  describe('isLoggedIn', () => {
    it('returns false when no token is set', async () => {
      const store = useUserStore()
      expect(await store.isLoggedIn()).toBe(false)
    })

    it('returns true without a server call when lastCheck is recent', async () => {
      const store = useUserStore()
      await store.login(VALID_JWT)
      store.user.lastCheck = Date.now()
      expect(await store.isLoggedIn()).toBe(true)
    })

    it('calls the server and returns true when token is valid', async () => {
      vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
        ok: true,
        headers: { get: (h: string) => h === 'content-type' ? 'text/plain' : null },
        text: async () => 'OK',
      }))
      const store = useUserStore()
      await store.login(VALID_JWT)
      store.user.lastCheck = 0
      expect(await store.isLoggedIn()).toBe(true)
      vi.unstubAllGlobals()
    })

    it('logs out and returns false when server returns Unauthorized', async () => {
      vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
        ok: false,
        statusText: 'Unauthorized',
        headers: { get: () => null },
      }))
      const store = useUserStore()
      // Set token directly to avoid triggering fetch during login
      store.user.accessToken = VALID_JWT
      store.user.lastCheck = 0
      expect(await store.isLoggedIn()).toBe(false)
      expect(store.hasToken()).toBe(false)
      vi.unstubAllGlobals()
    })
  })


  describe('localStorage state restoration', () => {
    it('restores token from localStorage on store creation', () => {
      const state = { accessToken: VALID_JWT, lastCheck: 0 }
      localStorage.setItem('userState', btoa(JSON.stringify(state)))
      // Re-create pinia after setting localStorage so the store reads it
      setActivePinia(createPinia())
      const store = useUserStore()
      expect(store.user.accessToken).toBe(VALID_JWT)
    })

    it('ignores legacy state with username/password fields', () => {
      const legacyState = { username: 'user', password: 'pass', accessToken: '' }
      localStorage.setItem('userState', btoa(JSON.stringify(legacyState)))
      setActivePinia(createPinia())
      const store = useUserStore()
      expect(store.user.accessToken).toBe('')
    })
  })
})
