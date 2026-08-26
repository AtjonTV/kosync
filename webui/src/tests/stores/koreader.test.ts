//
// File:        webui/src/tests/stores/koreader.test.ts
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

import { useKoreaderStore } from '@/stores/koreader'

describe('koreader credentials store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    pbMockModule.reset()
  })

  it('creates a credential through the KOsync API, never the collection API', async () => {
    const store = useKoreaderStore()
    await store.create('alice-kobo', 'device-password', 'Kobo Clara')

    // The password has to be hashed by the server, so this must not become a
    // plain collection create.
    expect(pbMockModule.send).toHaveBeenCalledWith('/api/kosync/koreader-accounts', {
      method: 'POST',
      body: { username: 'alice-kobo', password: 'device-password', label: 'Kobo Clara' },
    })
    expect(pbMockModule.collection('koreader_accounts').create).not.toHaveBeenCalled()
    expect(pbMockModule.collection('koreader_accounts').getFullList).toHaveBeenCalled()
  })

  it('changes a password through the KOsync API', async () => {
    const store = useKoreaderStore()
    await store.changePassword('account-1', 'a-brand-new-password')

    expect(pbMockModule.send).toHaveBeenCalledWith(
      '/api/kosync/koreader-accounts/account-1/password',
      { method: 'POST', body: { password: 'a-brand-new-password' } },
    )
    expect(pbMockModule.collection('koreader_accounts').update).not.toHaveBeenCalled()
  })

  it('renames a device through the collection API', async () => {
    const store = useKoreaderStore()
    await store.setLabel('account-1', 'Kobo Clara')

    // A rename touches nothing but the label, so it must never go near the
    // username or the password.
    expect(pbMockModule.collection('koreader_accounts').update).toHaveBeenCalledWith('account-1', {
      label: 'Kobo Clara',
    })
    expect(pbMockModule.send).not.toHaveBeenCalled()
    expect(pbMockModule.collection('koreader_accounts').getFullList).toHaveBeenCalled()
  })

  it('allows clearing the device name', async () => {
    const store = useKoreaderStore()
    await store.setLabel('account-1', '')

    expect(pbMockModule.collection('koreader_accounts').update).toHaveBeenCalledWith('account-1', {
      label: '',
    })
  })

  it('toggles the disabled flag through the collection API', async () => {
    const store = useKoreaderStore()
    await store.setDisabled('account-1', true)

    expect(pbMockModule.collection('koreader_accounts').update).toHaveBeenCalledWith('account-1', {
      disabled: true,
    })
  })

  it('reloads after deleting a credential', async () => {
    const store = useKoreaderStore()
    await store.remove('account-1')

    expect(pbMockModule.collection('koreader_accounts').delete).toHaveBeenCalledWith('account-1')
    expect(pbMockModule.collection('koreader_accounts').getFullList).toHaveBeenCalled()
  })
})
