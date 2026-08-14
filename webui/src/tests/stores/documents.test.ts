//
// File:        webui/src/tests/stores/documents.test.ts
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

import { useDocumentsStore } from '@/stores/documents'
import type { DocumentRecord, HistoryRecord } from '@/models'

function document(id: string, overrides: Partial<DocumentRecord> = {}): DocumentRecord {
  return {
    id,
    collectionId: 'c',
    collectionName: 'documents',
    created: '2026-03-01 10:00:00.000Z',
    updated: '2026-03-01 10:00:00.000Z',
    owner: 'user-a',
    document: 'hash-' + id,
    title: '',
    current_location: '/body',
    progress: 0.25,
    last_device: 'Kobo',
    last_device_id: 'AAA',
    last_read_at: '2026-03-01 10:00:00.000Z',
    source_account: 'account-a',
    book: '',
    ...overrides,
  }
}

function history(
  id: string,
  documentRef: string,
  overrides: Partial<HistoryRecord> = {},
): HistoryRecord {
  return {
    id,
    collectionId: 'c',
    collectionName: 'document_history',
    created: '2026-03-01 09:00:00.000Z',
    updated: '2026-03-01 09:00:00.000Z',
    owner: 'user-a',
    document_ref: documentRef,
    title: '',
    current_location: '/body',
    progress: 0.1,
    last_device: 'Kobo',
    last_device_id: 'AAA',
    last_read_at: '2026-03-01 09:00:00.000Z',
    ...overrides,
  }
}

describe('documents store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    pbMockModule.reset()
  })

  it('joins the history onto its document', async () => {
    pbMockModule
      .collection('documents')
      .getFullList.mockResolvedValue([document('doc-1'), document('doc-2')])
    pbMockModule
      .collection('document_history')
      .getFullList.mockResolvedValue([history('hist-1', 'doc-1'), history('hist-2', 'doc-1')])

    const store = useDocumentsStore()
    await store.load()

    expect(store.documents).toHaveLength(2)
    expect(store.documents[0]!.history).toHaveLength(2)
    expect(store.documents[1]!.history).toHaveLength(0)
    expect(store.loaded).toBe(true)
  })

  it('sorts the documents by the most recently read', async () => {
    pbMockModule
      .collection('documents')
      .getFullList.mockResolvedValue([
        document('old', { last_read_at: '2026-02-01 10:00:00.000Z' }),
        document('new', { last_read_at: '2026-03-01 10:00:00.000Z' }),
      ])

    const store = useDocumentsStore()
    await store.load()
    await store.subscribe()

    pbMockModule.emit(
      'documents',
      'update',
      document('old', { last_read_at: '2026-04-01 10:00:00.000Z' }),
    )

    expect(store.documents[0]!.id).toBe('old')
  })

  it('adds a document that appears while subscribed', async () => {
    const store = useDocumentsStore()
    await store.load()
    await store.subscribe()

    pbMockModule.emit('documents', 'create', document('fresh'))

    expect(store.documents.map((entry) => entry.id)).toEqual(['fresh'])
    expect(store.documents[0]!.history).toEqual([])
  })

  it('removes a document that was deleted elsewhere', async () => {
    pbMockModule.collection('documents').getFullList.mockResolvedValue([document('doc-1')])

    const store = useDocumentsStore()
    await store.load()
    await store.subscribe()

    pbMockModule.emit('documents', 'delete', document('doc-1'))

    expect(store.documents).toHaveLength(0)
  })

  it('keeps the loaded history when the document itself changes', async () => {
    pbMockModule.collection('documents').getFullList.mockResolvedValue([document('doc-1')])
    pbMockModule
      .collection('document_history')
      .getFullList.mockResolvedValue([history('hist-1', 'doc-1')])

    const store = useDocumentsStore()
    await store.load()
    await store.subscribe()

    pbMockModule.emit('documents', 'update', document('doc-1', { progress: 0.9 }))

    expect(store.documents[0]!.progress).toBe(0.9)
    expect(store.documents[0]!.history).toHaveLength(1)
  })

  it('folds a new history entry into its document', async () => {
    pbMockModule.collection('documents').getFullList.mockResolvedValue([document('doc-1')])

    const store = useDocumentsStore()
    await store.load()
    await store.subscribe()

    pbMockModule.emit('document_history', 'create', history('hist-1', 'doc-1'))

    expect(store.documents[0]!.history).toHaveLength(1)
  })

  it('ignores a history entry of an unknown document', async () => {
    const store = useDocumentsStore()
    await store.load()
    await store.subscribe()

    expect(() =>
      pbMockModule.emit('document_history', 'create', history('hist-1', 'nope')),
    ).not.toThrow()
    expect(store.documents).toHaveLength(0)
  })

  it('restores through the KOsync API and reloads', async () => {
    const store = useDocumentsStore()
    await store.restoreHistoryEntry('doc-1', 'hist-1')

    expect(pbMockModule.send).toHaveBeenCalledWith('/api/kosync/documents/doc-1/restore/hist-1', {
      method: 'POST',
    })
    expect(pbMockModule.collection('documents').getFullList).toHaveBeenCalled()
  })

  it('merges through the KOsync API and reloads', async () => {
    pbMockModule.send.mockResolvedValue({ message: '2 documents merged into one.' })

    const store = useDocumentsStore()
    const message = await store.merge('doc-1', ['doc-2', 'doc-3'])

    expect(pbMockModule.send).toHaveBeenCalledWith('/api/kosync/documents/merge', {
      method: 'POST',
      body: { into: 'doc-1', from: ['doc-2', 'doc-3'] },
    })
    expect(message).toBe('2 documents merged into one.')
    // The merged history is moved server side in one statement, so no
    // subscription reports it and the page has to be read again.
    expect(pbMockModule.collection('documents').getFullList).toHaveBeenCalled()
  })

  it('clears everything on sign out', async () => {
    pbMockModule.collection('documents').getFullList.mockResolvedValue([document('doc-1')])

    const store = useDocumentsStore()
    await store.load()
    await store.subscribe()
    store.clear()

    expect(store.documents).toHaveLength(0)
    expect(store.loaded).toBe(false)
  })
})
