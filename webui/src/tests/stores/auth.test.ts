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
    browserTimezone: actual.browserTimezone,
    timezoneNames: actual.timezoneNames,
  }
})

import { useAuthStore } from '@/stores/auth'
import { browserTimezone } from '@/pb'

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
      // Registration is the one moment the zone can be learned without asking:
      // no device ever tells the server what time it thinks it is.
      timezone: browserTimezone(),
      // Somebody registering in a browser is asking to hear about their reading;
      // an account created by a script is not, which is why the field is off
      // until something says otherwise.
      achievement_mail: true,
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

  it('changes the password and signs back in with it', async () => {
    pbMockModule.authStore.record = { id: 'user-a', email: 'alice@example.com' }
    pbMockModule.authStore.isValid = true

    const store = useAuthStore()
    await store.changePassword('the-old-password', 'the-new-password')

    // The old password has to be sent along, otherwise PocketBase refuses.
    expect(pbMockModule.collection('users').update).toHaveBeenCalledWith('user-a', {
      oldPassword: 'the-old-password',
      password: 'the-new-password',
      passwordConfirm: 'the-new-password',
    })

    // Changing the password invalidates the session, so the store has to get a
    // new one instead of leaving the user on a dead token.
    expect(pbMockModule.collection('users').authWithPassword).toHaveBeenCalledWith(
      'alice@example.com',
      'the-new-password',
    )
  })

  it('refuses to change the password without a session', async () => {
    const store = useAuthStore()

    await expect(store.changePassword('old', 'new')).rejects.toThrow()
    expect(pbMockModule.collection('users').update).not.toHaveBeenCalled()
  })

  it('does not sign back in when the password change fails', async () => {
    pbMockModule.authStore.record = { id: 'user-a', email: 'alice@example.com' }
    pbMockModule.authStore.isValid = true
    pbMockModule.collection('users').update.mockRejectedValue(new Error('wrong password'))

    const store = useAuthStore()

    await expect(store.changePassword('wrong', 'the-new-password')).rejects.toThrow()
    expect(pbMockModule.collection('users').authWithPassword).not.toHaveBeenCalled()
  })

  it('asks PocketBase to confirm a new address', async () => {
    pbMockModule.authStore.record = { id: 'user-a', email: 'alice@invalid.local' }
    pbMockModule.authStore.isValid = true

    const store = useAuthStore()
    await store.requestEmailChange('alice@example.com')

    // The confirmation goes to the new address, which is what makes this usable
    // for the placeholder addresses the legacy import hands out.
    expect(pbMockModule.collection('users').requestEmailChange).toHaveBeenCalledWith(
      'alice@example.com',
    )
  })

  it('changes the timezone and keeps the returned record', async () => {
    pbMockModule.authStore.record = { id: 'user-a', timezone: 'UTC' }
    pbMockModule
      .collection('users')
      .update.mockResolvedValue({ id: 'user-a', timezone: 'Europe/Vienna' })

    const store = useAuthStore()
    store.record = { id: 'user-a', collectionId: 'c', collectionName: 'users', timezone: 'UTC' }
    await store.changeTimezone('Europe/Vienna')

    expect(pbMockModule.collection('users').update).toHaveBeenCalledWith('user-a', {
      timezone: 'Europe/Vienna',
    })
    expect(store.timezone).toBe('Europe/Vienna')
  })

  it('falls back to UTC when the account has no timezone', () => {
    const store = useAuthStore()
    store.record = { id: 'user-a', collectionId: 'c', collectionName: 'users' }

    expect(store.timezone).toBe('UTC')
  })

  it('turns the achievement notices off and on', async () => {
    const store = useAuthStore()
    store.record = { id: 'user-a', collectionId: 'c', collectionName: 'users' }
    await store.setAchievementMail(false)

    expect(pbMockModule.collection('users').update).toHaveBeenCalledWith('user-a', {
      achievement_mail: false,
    })
  })

  it('chooses how often a reading summary arrives', async () => {
    const store = useAuthStore()
    store.record = { id: 'user-a', collectionId: 'c', collectionName: 'users' }
    await store.setSummaryMail('weekly')

    expect(pbMockModule.collection('users').update).toHaveBeenCalledWith('user-a', {
      summary_mail: 'weekly',
    })
  })

  // An account that has never been asked hears nothing, which is what makes the
  // whole feature safe to add to a server people are already using.
  it('reads an unset cadence as off', () => {
    const store = useAuthStore()
    store.record = { id: 'user-a', collectionId: 'c', collectionName: 'users' }

    expect(store.summaryMail).toBe('off')
  })
})
