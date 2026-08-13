//
// File:        webui/src/tests/stores/auth.test.ts
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

import { beforeEach, describe, expect, it, vi } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import * as pbMockModule from '../mocks/pb'

vi.mock('@/pb', async () => {
  const mock = await import('../mocks/pb')
  const actual = await vi.importActual<typeof import('@/pb')>('@/pb')

  return {
    pb: mock.pbMock,
    Collections: actual.Collections,
    KosyncApi: actual.KosyncApi,
    errorMessage: actual.errorMessage,
  }
})

import { useAuthStore } from '@/stores/auth'

describe('auth store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    pbMockModule.reset()
  })

  it('signs in with an identity and password', async () => {
    const store = useAuthStore()
    await store.login('alice@example.com', 'a-long-enough-password')

    expect(pbMockModule.collection('users').authWithPassword).toHaveBeenCalledWith(
      'alice@example.com',
      'a-long-enough-password',
    )
  })

  it('registers, signs in and asks for a verification mail', async () => {
    const store = useAuthStore()
    await store.register('new@example.com', 'a-long-enough-password', 'Newcomer')

    expect(pbMockModule.collection('users').create).toHaveBeenCalledWith({
      email: 'new@example.com',
      password: 'a-long-enough-password',
      passwordConfirm: 'a-long-enough-password',
      name: 'Newcomer',
    })
    expect(pbMockModule.collection('users').authWithPassword).toHaveBeenCalled()
    expect(pbMockModule.collection('users').requestVerification).toHaveBeenCalledWith(
      'new@example.com',
    )
  })

  it('still registers when the server cannot send mail', async () => {
    pbMockModule
      .collection('users')
      .requestVerification.mockRejectedValue(new Error('no SMTP configured'))

    const store = useAuthStore()

    await expect(
      store.register('new@example.com', 'a-long-enough-password', ''),
    ).resolves.toBeUndefined()
  })

  it('reports an expired session and clears it', async () => {
    pbMockModule.authStore.isValid = true
    pbMockModule.collection('users').authRefresh.mockRejectedValue(new Error('expired'))

    const store = useAuthStore()

    expect(await store.refresh()).toBe(false)
    expect(pbMockModule.authStore.clear).toHaveBeenCalled()
  })

  it('does not call the server when there is no session', async () => {
    const store = useAuthStore()

    expect(await store.refresh()).toBe(false)
    expect(pbMockModule.collection('users').authRefresh).not.toHaveBeenCalled()
  })
})
