//
// File:        webui/src/tests/stores/devices.test.ts
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
    fileUrl: actual.fileUrl,
  }
})

import { useDevicesStore } from '@/stores/devices'
import type { Device } from '@/models'

// The identifier from the reference database, which is exactly the sort of
// thing nobody should have to read on screen.
const go7Id = '865F46C0C0F4401D9A05768B6B0BF3AC'

function device(overrides: Partial<Device> = {}): Device {
  return {
    id: 'device-a',
    collectionId: 'c',
    collectionName: 'devices',
    created: '',
    updated: '',
    owner: 'user-a',
    device_id: go7Id,
    reported_name: 'go7',
    name: '',
    last_seen: '2026-03-01 10:00:00.000Z',
    ...overrides,
  }
}

describe('devices store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    pbMockModule.reset()
  })

  it('asks for the devices most recently seen first', async () => {
    const store = useDevicesStore()
    await store.load()

    expect(pbMockModule.collections.get('devices')?.getFullList).toHaveBeenCalledWith({
      sort: '-last_seen',
    })
  })

  it('prefers the chosen name, then the reported one, then the identifier', async () => {
    const collection = pbMockModule.collection('devices')
    collection.getFullList.mockResolvedValue([device({ name: 'Boox Go 7' })])

    const store = useDevicesStore()
    await store.load()

    expect(store.nameOf(go7Id)).toBe('Boox Go 7')

    collection.getFullList.mockResolvedValue([device({ name: '' })])
    await store.load()
    expect(store.nameOf(go7Id)).toBe('go7')

    collection.getFullList.mockResolvedValue([device({ name: '', reported_name: '' })])
    await store.load()
    expect(store.nameOf(go7Id)).toBe(go7Id)
  })

  // A device that has not been loaded, or one from before the registry existed,
  // still has to render as something.
  it('falls back to the identifier for a device it does not know', () => {
    const store = useDevicesStore()

    expect(store.nameOf(go7Id)).toBe(go7Id)
    expect(store.nameOf('')).toBe('')
  })

  it('sends only the name when renaming', async () => {
    const store = useDevicesStore()
    await store.rename('device-a', 'Boox Go 7')

    expect(pbMockModule.collections.get('devices')?.update).toHaveBeenCalledWith('device-a', {
      name: 'Boox Go 7',
    })
  })

  it('folds live changes into the loaded devices', async () => {
    const collection = pbMockModule.collection('devices')
    collection.getFullList.mockResolvedValue([device()])

    const store = useDevicesStore()
    await store.load()
    await store.subscribe()

    pbMockModule.emit('devices', 'update', device({ name: 'Boox Go 7' }))
    expect(store.nameOf(go7Id)).toBe('Boox Go 7')

    pbMockModule.emit(
      'devices',
      'create',
      device({ id: 'device-b', device_id: 'els', name: 'N39' }),
    )
    expect(store.nameOf('els')).toBe('N39')

    pbMockModule.emit('devices', 'delete', device())
    expect(store.devices).toHaveLength(1)
  })

  it('drops everything when it is cleared', async () => {
    const collection = pbMockModule.collection('devices')
    collection.getFullList.mockResolvedValue([device()])

    const store = useDevicesStore()
    await store.load()
    store.clear()

    expect(store.devices).toHaveLength(0)
    expect(store.loaded).toBe(false)
  })
})
