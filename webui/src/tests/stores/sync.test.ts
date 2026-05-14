//
// File:        webui/src/tests/stores/sync.test.ts
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2025-2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

import { describe, it, expect, vi, beforeEach } from 'vitest'
import { useSyncStore } from '@/stores/sync.ts'
import type { DocumentWithHistory } from '@/models/document.ts'

// Mock JMPClient so no real WebSocket connections are made
const mockRpc = vi.fn().mockResolvedValue(null)
const mockSubscribe = vi.fn()
const mockConnect = vi.fn((cb: () => void) => cb())

vi.mock('jmp-client-js', () => {
  function MockJMPClient() {
    return { rpc: mockRpc, subscribe: mockSubscribe, connect: mockConnect }
  }
  return { default: MockJMPClient }
})

// Mock getWebSocketUrl so it returns a fake URL (avoids needing a logged-in user)
vi.mock('@/api.ts', () => ({
  getWebSocketUrl: vi.fn().mockReturnValue('ws://localhost/api/ws/test-token'),
  fetchApi: vi.fn(),
}))

function makeDocument(overrides: Partial<DocumentWithHistory> = {}): DocumentWithHistory {
  return {
    id: 'doc-1',
    owner_id: 'user-1',
    title: 'Test Book',
    current_location: 'chapter-1',
    progress: 0.5,
    last_read_on_device: 'Kindle',
    last_read_on_device_id: 'device-1',
    last_read_at: 1000,
    history: [],
    ...overrides,
  }
}

describe('useSyncStore', () => {
  beforeEach(() => {
    mockRpc.mockReset()
    mockRpc.mockResolvedValue(null)
    mockSubscribe.mockReset()
    mockConnect.mockReset()
    mockConnect.mockImplementation((cb: () => void) => cb())
  })

  describe('clear', () => {
    it('resets documents and lastSync', () => {
      const store = useSyncStore()
      store.sync.documents = [makeDocument()]
      store.sync.lastSync = 12345
      store.clear()
      expect(store.sync.documents).toEqual([])
      expect(store.sync.lastSync).toBe(-1)
    })

    it('removes syncState from sessionStorage', () => {
      sessionStorage.setItem('syncState', btoa(JSON.stringify({ lastSync: 1, documents: [] })))
      const store = useSyncStore()
      store.clear()
      expect(sessionStorage.getItem('syncState')).toBeNull()
    })
  })

  describe('sessionStorage state restoration', () => {
    it('restores sync state from sessionStorage on store creation', () => {
      const doc = makeDocument()
      const state = { lastSync: 9999, documents: [doc] }
      sessionStorage.setItem('syncState', btoa(JSON.stringify(state)))
      // setup.ts beforeEach already set a fresh pinia; store reads sessionStorage at creation
      const store = useSyncStore()
      expect(store.sync.lastSync).toBe(9999)
      expect(store.sync.documents).toHaveLength(1)
      expect(store.sync.documents[0]?.id).toBe('doc-1')
    })
  })

  describe('doSync', () => {
    it('skips sync when lastSync is recent and forceRefresh is false', async () => {
      const store = useSyncStore()
      store.sync.lastSync = Date.now()
      await store.doSync(false)
      expect(mockRpc).not.toHaveBeenCalled()
    })

    it('fetches documents and updates state when forceRefresh is true', async () => {
      const doc = makeDocument()
      mockRpc.mockResolvedValue([doc])

      const store = useSyncStore()
      await store.doSync(true)

      expect(mockRpc).toHaveBeenCalledWith('documents.all', {})
      expect(store.sync.documents).toHaveLength(1)
      expect(store.sync.documents[0]?.id).toBe('doc-1')
    })

    it('does not update state when rpc returns null', async () => {
      mockRpc.mockResolvedValue(null)
      const store = useSyncStore()
      store.sync.documents = [makeDocument()]
      await store.doSync(true)
      expect(store.sync.documents).toHaveLength(1)
    })

    it('handles errors gracefully without throwing', async () => {
      mockConnect.mockImplementation(() => { throw new Error('connection failed') })
      const store = useSyncStore()
      await expect(store.doSync(true)).resolves.not.toThrow()
    })
  })

  describe('deleteHistoryItem', () => {
    it('calls rpc with correct arguments', async () => {
      const store = useSyncStore()
      await store.deleteHistoryItem('doc-1', 1000)
      expect(mockRpc).toHaveBeenCalledWith('documents.history.delete', {
        document_id: 'doc-1',
        last_read_at: 1000,
      })
    })

    it('handles errors gracefully without throwing', async () => {
      mockRpc.mockRejectedValue(new Error('rpc failed'))
      const store = useSyncStore()
      await expect(store.deleteHistoryItem('doc-1', 1000)).resolves.not.toThrow()
    })
  })

  describe('restoreHistoryItem', () => {
    it('calls rpc with correct arguments', async () => {
      const store = useSyncStore()
      await store.restoreHistoryItem('doc-1', 1000)
      expect(mockRpc).toHaveBeenCalledWith('documents.history.restore', {
        document_id: 'doc-1',
        last_read_at: 1000,
      })
    })

    it('handles errors gracefully without throwing', async () => {
      mockRpc.mockRejectedValue(new Error('rpc failed'))
      const store = useSyncStore()
      await expect(store.restoreHistoryItem('doc-1', 1000)).resolves.not.toThrow()
    })
  })

  describe('updateDocument', () => {
    it('calls rpc with correct arguments', async () => {
      const store = useSyncStore()
      const docData = { id: 'doc-1', title: 'Updated Title' }
      await store.updateDocument(docData)
      expect(mockRpc).toHaveBeenCalledWith('documents.update', { document: docData })
    })

    it('handles errors gracefully without throwing', async () => {
      mockRpc.mockRejectedValue(new Error('rpc failed'))
      const store = useSyncStore()
      await expect(store.updateDocument({ id: 'doc-1' })).resolves.not.toThrow()
    })
  })

  describe('deleteDocument', () => {
    it('calls rpc with correct arguments', async () => {
      const store = useSyncStore()
      await store.deleteDocument('doc-1')
      expect(mockRpc).toHaveBeenCalledWith('documents.delete', { document_id: 'doc-1' })
    })

    it('handles errors gracefully without throwing', async () => {
      mockRpc.mockRejectedValue(new Error('rpc failed'))
      const store = useSyncStore()
      await expect(store.deleteDocument('doc-1')).resolves.not.toThrow()
    })
  })

  describe('doPubSubSync - document updates', () => {
    it('updates an existing document in the list on Document typeHint', async () => {
      const original = makeDocument({ id: 'doc-1', progress: 0.3, history: [] })
      const updated = makeDocument({ id: 'doc-1', progress: 0.8, history: [] })

      let capturedCallback: Function | null = null
      mockSubscribe.mockImplementation((_topic: string, cb: Function) => {
        capturedCallback = cb
      })

      const store = useSyncStore()
      store.sync.documents = [original]
      await store.doPubSubSync()
      capturedCallback!(updated, 'Document', null)

      expect(store.sync.documents[0]?.progress).toBe(0.8)
    })

    it('removes a document from the list on DocumentDeletion typeHint', async () => {
      let capturedCallback: Function | null = null
      mockSubscribe.mockImplementation((_topic: string, cb: Function) => {
        capturedCallback = cb
      })

      const store = useSyncStore()
      store.sync.documents = [makeDocument({ id: 'doc-1' })]
      await store.doPubSubSync()
      capturedCallback!({ document_id: 'doc-1' }, 'DocumentDeletion', null)

      expect(store.sync.documents).toHaveLength(0)
    })

    it('removes a history entry on HistoryDeletion typeHint', async () => {
      const historyEntry = makeDocument({ id: 'doc-1', last_read_at: 500 })
      const doc = makeDocument({ id: 'doc-1', history: [historyEntry] })

      let capturedCallback: Function | null = null
      mockSubscribe.mockImplementation((_topic: string, cb: Function) => {
        capturedCallback = cb
      })

      const store = useSyncStore()
      store.sync.documents = [doc]
      await store.doPubSubSync()
      capturedCallback!({ document_id: 'doc-1', last_read_at: 500 }, 'HistoryDeletion', null)

      expect(store.sync.documents[0]?.history).toHaveLength(0)
    })

    it('does nothing when errors are present in the callback', async () => {
      let capturedCallback: Function | null = null
      mockSubscribe.mockImplementation((_topic: string, cb: Function) => {
        capturedCallback = cb
      })

      const store = useSyncStore()
      store.sync.documents = [makeDocument({ id: 'doc-1', progress: 0.3 })]
      await store.doPubSubSync()
      capturedCallback!(null, 'Document', ['some error'])

      expect(store.sync.documents[0]?.progress).toBe(0.3)
    })
  })
})
