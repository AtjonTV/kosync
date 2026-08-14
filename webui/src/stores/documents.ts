//
// File:        webui/src/stores/documents.ts
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

import { ref } from 'vue'
import { defineStore } from 'pinia'
import { pb, Collections, KosyncApi } from '@/pb'
import type { DocumentRecord, DocumentWithHistory, HistoryRecord } from '@/models'

/**
 * The documents of the signed in account and the states they went through.
 *
 * Live updates come from PocketBase subscriptions, which are filtered by the
 * very same collection rules that protect the REST API. The legacy server
 * needed a custom WebSocket protocol for this.
 */
export const useDocumentsStore = defineStore('documents', () => {
  const documents = ref<DocumentWithHistory[]>([])
  const loading = ref(false)
  const loaded = ref(false)

  let unsubscribeDocuments: (() => void) | null = null
  let unsubscribeHistory: (() => void) | null = null

  /** Loads every document with its history. */
  async function load(): Promise<void> {
    loading.value = true
    try {
      const [records, history] = await Promise.all([
        pb.collection(Collections.documents).getFullList<DocumentRecord>({ sort: '-last_read_at' }),
        pb
          .collection(Collections.documentHistory)
          .getFullList<HistoryRecord>({ sort: '-last_read_at' }),
      ])

      const byDocument = new Map<string, HistoryRecord[]>()
      for (const entry of history) {
        const entries = byDocument.get(entry.document_ref) ?? []
        entries.push(entry)
        byDocument.set(entry.document_ref, entries)
      }

      documents.value = records.map((record) => ({
        ...record,
        history: byDocument.get(record.id) ?? [],
      }))
      loaded.value = true
    } finally {
      loading.value = false
    }
  }

  /** Starts applying live changes to the loaded documents. */
  async function subscribe(): Promise<void> {
    if (unsubscribeDocuments) return

    unsubscribeDocuments = await pb
      .collection(Collections.documents)
      .subscribe<DocumentRecord>('*', (event) => {
        applyDocumentEvent(event.action, event.record)
      })

    unsubscribeHistory = await pb
      .collection(Collections.documentHistory)
      .subscribe<HistoryRecord>('*', (event) => {
        applyHistoryEvent(event.action, event.record)
      })
  }

  /** Stops the live updates, for example on sign out. */
  function unsubscribe(): void {
    unsubscribeDocuments?.()
    unsubscribeHistory?.()
    unsubscribeDocuments = null
    unsubscribeHistory = null
  }

  function applyDocumentEvent(action: string, record: DocumentRecord): void {
    const index = documents.value.findIndex((entry) => entry.id === record.id)

    if (action === 'delete') {
      if (index !== -1) documents.value.splice(index, 1)
      return
    }

    if (index === -1) {
      documents.value.unshift({ ...record, history: [] })
    } else {
      // Keep the history that is already loaded; it arrives through its own
      // subscription.
      documents.value[index] = { ...record, history: documents.value[index]!.history }
    }

    sortByLastRead()
  }

  function applyHistoryEvent(action: string, record: HistoryRecord): void {
    const document = documents.value.find((entry) => entry.id === record.document_ref)
    if (!document) return

    const index = document.history.findIndex((entry) => entry.id === record.id)

    if (action === 'delete') {
      if (index !== -1) document.history.splice(index, 1)
      return
    }

    if (index === -1) {
      document.history.unshift(record)
    } else {
      document.history[index] = record
    }

    document.history.sort((a, b) => b.last_read_at.localeCompare(a.last_read_at))
  }

  function sortByLastRead(): void {
    documents.value.sort((a, b) => b.last_read_at.localeCompare(a.last_read_at))
  }

  async function updateTitle(id: string, title: string): Promise<void> {
    await pb.collection(Collections.documents).update(id, { title })
  }

  async function remove(id: string): Promise<void> {
    await pb.collection(Collections.documents).delete(id)
  }

  async function removeHistoryEntry(id: string): Promise<void> {
    await pb.collection(Collections.documentHistory).delete(id)
  }

  /** Puts a document back into an earlier state. */
  async function restoreHistoryEntry(documentId: string, historyId: string): Promise<void> {
    await pb.send(KosyncApi.restoreHistory(documentId, historyId), { method: 'POST' })
    await load()
  }

  /**
   * Folds several documents into one and returns what the server called it.
   *
   * The server moves the history of the merged documents with a single update
   * rather than record by record, which is the right thing for a document with
   * thousands of entries but means those rows arrive through no subscription.
   * Reloading afterwards is what puts the page back in step.
   */
  async function merge(into: string, from: string[]): Promise<string> {
    const response = await pb.send<{ message?: string }>(KosyncApi.mergeDocuments, {
      method: 'POST',
      body: { into, from },
    })
    await load()

    return response?.message ?? 'The documents were merged.'
  }

  function clear(): void {
    unsubscribe()
    documents.value = []
    loaded.value = false
  }

  return {
    documents,
    loading,
    loaded,
    load,
    subscribe,
    unsubscribe,
    updateTitle,
    remove,
    removeHistoryEntry,
    restoreHistoryEntry,
    merge,
    clear,
  }
})
