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

  /**
   * Which documents have had their history fetched.
   *
   * A document whose history has never been asked for holds an empty array,
   * which is indistinguishable from one that genuinely has no history — hence
   * this rather than a check on the array itself.
   */
  const historyLoaded = ref(new Set<string>())
  const historyLoading = ref(false)

  /**
   * Loads the documents, and deliberately not their history.
   *
   * The history is every state every document has ever been in — thousands of
   * rows on an account that has been syncing for a year, fetched 500 at a time.
   * It used to be loaded here, on every page that shows a document, and it is
   * shown in exactly one place: a dialog, for one document, when somebody asks
   * for it. So it is fetched there instead.
   */
  async function load(): Promise<void> {
    loading.value = true
    try {
      const records = await pb
        .collection(Collections.documents)
        .getFullList<DocumentRecord>({ sort: '-last_read_at' })

      // Any history already in hand is kept: a reload should not empty a dialog
      // somebody is looking at.
      const existing = new Map(documents.value.map((entry) => [entry.id, entry.history]))

      documents.value = records.map((record) => ({
        ...record,
        history: existing.get(record.id) ?? [],
      }))
      loaded.value = true
    } finally {
      loading.value = false
    }
  }

  /**
   * Fetches the history of one document.
   *
   * Filtered server side and served by the index on (document_ref,
   * last_read_at), so this is one indexed read of the rows that are about to be
   * shown rather than a walk through everything the account has ever synced.
   */
  async function loadHistory(documentId: string, force = false): Promise<void> {
    if (!documentId) return
    if (historyLoaded.value.has(documentId) && !force) return

    historyLoading.value = true
    try {
      const history = await pb.collection(Collections.documentHistory).getFullList<HistoryRecord>({
        filter: pb.filter('document_ref = {:id}', { id: documentId }),
        sort: '-last_read_at',
      })

      const document = documents.value.find((entry) => entry.id === documentId)
      if (document) document.history = history

      historyLoaded.value = new Set(historyLoaded.value).add(documentId)
    } finally {
      historyLoading.value = false
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

    // A document whose history has not been fetched has an empty array, and
    // folding one live event into it would produce a list of one entry that
    // looks like the whole story. It is fetched in full when it is asked for.
    if (!historyLoaded.value.has(record.document_ref)) return

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
    // The restore consumes the entry it restored and archives the state it
    // replaced, so the list the dialog is showing is now wrong in two places.
    await loadHistory(documentId, true)
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

    // Every history that was loaded may now belong to a different document, so
    // none of them can be trusted; they are fetched again when asked for.
    historyLoaded.value = new Set()
    await load()

    return response?.message ?? 'The documents were merged.'
  }

  function clear(): void {
    unsubscribe()
    documents.value = []
    historyLoaded.value = new Set()
    loaded.value = false
  }

  return {
    documents,
    loading,
    loaded,
    historyLoading,
    load,
    loadHistory,
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
