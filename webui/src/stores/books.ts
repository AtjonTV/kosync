//
// File:        webui/src/stores/books.ts
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

import { computed, ref } from 'vue'
import { defineStore } from 'pinia'
import { pb, Collections, KosyncApi } from '@/pb'
import type { Book, StorageUsage } from '@/models'

/**
 * The uploaded books of the signed in account.
 *
 * An upload is an ordinary collection create with the file attached; the server
 * reads the EPUB as it arrives and fills in the title, authors, cover, word
 * count and the two hashes KOReader identifies the book by. Nothing derived is
 * sent from here, because none of it would be trustworthy.
 */
export const useBooksStore = defineStore('books', () => {
  const books = ref<Book[]>([])
  const loading = ref(false)
  const loaded = ref(false)

  let unsubscribe: (() => void) | null = null

  /**
   * How much room the library takes, and how much it may take.
   *
   * The limit is an operator setting rather than data, so it cannot be summed
   * from the books the way the usage could be: the server has to say it. Both
   * halves come from the same answer so the bar and a refused upload can never
   * disagree about what full means.
   */
  const usage = ref<StorageUsage | null>(null)
  const limited = computed(() => (usage.value?.quota ?? 0) > 0)

  async function load(): Promise<void> {
    loading.value = true
    try {
      books.value = await pb.collection(Collections.books).getFullList<Book>({ sort: 'title' })
      loaded.value = true
    } finally {
      loading.value = false
    }

    // After the books, and never in a way that can fail the load: a library
    // that cannot show how full it is should still show its books.
    try {
      usage.value = await pb.send<StorageUsage>(KosyncApi.storage, { method: 'GET' })
    } catch {
      usage.value = null
    }
  }

  /**
   * Uploads an EPUB. The owner has to be set explicitly: the create rule checks
   * it against the caller, so the server does not fill it in.
   */
  async function upload(file: File): Promise<Book> {
    const owner = pb.authStore.record?.id
    if (!owner) throw new Error('You are not signed in.')

    const form = new FormData()
    form.append('owner', owner)
    form.append('file', file)

    const created = await pb.collection(Collections.books).create<Book>(form)
    await load()

    return created
  }

  /** Corrects the title. The fields derived from the file are read only. */
  async function rename(id: string, title: string): Promise<void> {
    await pb.collection(Collections.books).update(id, { title })
    await load()
  }

  async function remove(id: string): Promise<void> {
    await pb.collection(Collections.books).delete(id)
    await load()
  }

  /** Starts applying live changes to the loaded library. */
  async function subscribe(): Promise<void> {
    if (unsubscribe) return

    unsubscribe = await pb.collection(Collections.books).subscribe<Book>('*', (event) => {
      const existing = books.value.findIndex((book) => book.id === event.record.id)

      if (event.action === 'delete') {
        if (existing >= 0) books.value.splice(existing, 1)

        return
      }

      if (existing >= 0) {
        books.value[existing] = event.record
      } else {
        books.value.push(event.record)
      }

      books.value.sort((a, b) => a.title.localeCompare(b.title))
    })
  }

  function unsubscribeAll(): void {
    unsubscribe?.()
    unsubscribe = null
  }

  function clear(): void {
    unsubscribeAll()
    books.value = []
    usage.value = null
    loaded.value = false
  }

  return {
    books,
    loading,
    loaded,
    usage,
    limited,
    load,
    upload,
    rename,
    remove,
    subscribe,
    unsubscribe: unsubscribeAll,
    clear,
  }
})
