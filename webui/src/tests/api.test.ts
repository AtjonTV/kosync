//
// File:        webui/src/tests/api.test.ts
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2025-2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

import { describe, it, expect, vi, beforeEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { fetchApi, getWebSocketUrl } from '@/api.ts'
import { useUserStore } from '@/stores/user.ts'

const VALID_JWT = [
  'header',
  btoa(JSON.stringify({ username: 'testuser' })),
  'signature',
].join('.')

describe('fetchApi', () => {
  it('returns an error when not logged in', async () => {
    const { data, error } = await fetchApi('/some/route')
    expect(data).toBeNull()
    expect(error).toBe('Not logged in')
  })

  it('sends the Authorization header with the Bearer token', async () => {
    const mockFetch = vi.fn().mockResolvedValue({
      ok: true,
      headers: { get: () => 'application/json' },
      json: async () => ({ result: 'ok' }),
    })
    vi.stubGlobal('fetch', mockFetch)

    const store = useUserStore()
    await store.login(VALID_JWT)

    await fetchApi('/api/test')

    expect(mockFetch).toHaveBeenCalledOnce()
    const callArgs = mockFetch.mock.calls[0]
    expect(callArgs[1].headers['Authorization']).toBe(`Bearer ${VALID_JWT}`)
    vi.unstubAllGlobals()
  })

  it('returns parsed JSON data on a successful JSON response', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      ok: true,
      headers: { get: () => 'application/json' },
      json: async () => ({ id: 1, name: 'test' }),
    }))

    const store = useUserStore()
    await store.login(VALID_JWT)

    const { data, error } = await fetchApi<{ id: number; name: string }>('/api/test')
    expect(error).toBeNull()
    expect(data).toEqual({ id: 1, name: 'test' })
    vi.unstubAllGlobals()
  })

  it('returns text data when response is not JSON', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      ok: true,
      headers: { get: () => 'text/plain' },
      text: async () => 'OK',
    }))

    const store = useUserStore()
    await store.login(VALID_JWT)

    const { data, error } = await fetchApi<string>('/api/test')
    expect(error).toBeNull()
    expect(data).toBe('OK')
    vi.unstubAllGlobals()
  })

  it('returns an error when the response is not ok', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      ok: false,
      statusText: 'Not Found',
      headers: { get: () => null },
    }))

    const store = useUserStore()
    await store.login(VALID_JWT)

    const { data, error } = await fetchApi('/api/missing')
    expect(data).toBeNull()
    expect(error).toBe('Not Found')
    vi.unstubAllGlobals()
  })

  it('merges custom headers with the Authorization header', async () => {
    const mockFetch = vi.fn().mockResolvedValue({
      ok: true,
      headers: { get: () => 'application/json' },
      json: async () => ({}),
    })
    vi.stubGlobal('fetch', mockFetch)

    const store = useUserStore()
    await store.login(VALID_JWT)

    await fetchApi('/api/test', { headers: { 'X-Custom': 'value' } })

    const callArgs = mockFetch.mock.calls[0]
    expect(callArgs[1].headers['Authorization']).toBe(`Bearer ${VALID_JWT}`)
    expect(callArgs[1].headers['X-Custom']).toBe('value')
    vi.unstubAllGlobals()
  })
})

describe('getWebSocketUrl', () => {
  it('returns a string (not null) even without a token, because isLoggedIn() returns a Promise which is truthy', () => {
    // NOTE: getWebSocketUrl calls isLoggedIn() which is async and returns a Promise.
    // A Promise is always truthy, so the null-guard never triggers — this is a known
    // quirk of the current implementation.
    const url = getWebSocketUrl()
    expect(typeof url).toBe('string')
  })

  it('returns a WebSocket URL containing the access token when logged in', async () => {
    const store = useUserStore()
    await store.login(VALID_JWT)
    const url = getWebSocketUrl()
    expect(url).toContain(VALID_JWT)
    expect(url).toContain('/api/ws/')
  })
})
