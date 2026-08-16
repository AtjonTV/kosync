//
// File:        webui/src/stores/collections.ts
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

import { computed, ref } from 'vue'
import { defineStore } from 'pinia'
import { pb, Collections } from '@/pb'
import type { BookCollection } from '@/models'

/** Sorted the way they are listed: by name, the way a shelf is looked for. */
const byName = (a: BookCollection, b: BookCollection) =>
  a.name.localeCompare(b.name, undefined, { sensitivity: 'base' }) || a.name.localeCompare(b.name)

/**
 * The shelves the signed in account has put together.
 *
 * A shelf is the one thing in the library that is entirely somebody's own
 * opinion: everything else is read out of a file or reported by a device. So
 * unlike the books, these are created and thrown away from here, through the
 * ordinary collection API.
 */
export const useCollectionsStore = defineStore('collections', () => {
  const collections = ref<BookCollection[]>([])
  const loading = ref(false)
  const loaded = ref(false)

  let unsubscribe: (() => void) | null = null

  /** One shelf by id, for the page that shows a single one. */
  const byId = computed(() => new Map(collections.value.map((one) => [one.id, one])))

  /** Which shelves a book stands on, keyed by book id. */
  const byBook = computed(() => {
    const shelves = new Map<string, BookCollection[]>()

    for (const collection of collections.value) {
      for (const book of collection.books ?? []) {
        const holding = shelves.get(book)
        if (holding) holding.push(collection)
        else shelves.set(book, [collection])
      }
    }

    return shelves
  })

  async function load(): Promise<void> {
    loading.value = true
    try {
      collections.value = await pb
        .collection(Collections.bookCollections)
        .getFullList<BookCollection>({ sort: 'name' })
      loaded.value = true
    } finally {
      loading.value = false
    }
  }

  /**
   * Makes a shelf. The owner has to be set explicitly: the create rule checks it
   * against the caller, so the server does not fill it in.
   */
  async function create(name: string, description = ''): Promise<BookCollection> {
    const owner = pb.authStore.record?.id
    if (!owner) throw new Error('You are not signed in.')

    const created = await pb
      .collection(Collections.bookCollections)
      .create<BookCollection>({ owner, name, description })
    await load()

    return created
  }

  async function update(id: string, changes: Partial<BookCollection>): Promise<void> {
    await pb.collection(Collections.bookCollections).update(id, changes)
    await load()
  }

  async function remove(id: string): Promise<void> {
    await pb.collection(Collections.bookCollections).delete(id)
    await load()
  }

  /**
   * Puts a book on a shelf, at the end of it.
   *
   * Sent as PocketBase's own append rather than as the whole list, so that two
   * open tabs cannot overwrite each other's shelf: what is sent is the change,
   * not a version of the list that was read some time ago.
   */
  async function addBook(id: string, book: string): Promise<void> {
    await pb.collection(Collections.bookCollections).update(id, { 'books+': book })
    await load()
  }

  /** Takes a book off a shelf, leaving the book in the library. */
  async function removeBook(id: string, book: string): Promise<void> {
    await pb.collection(Collections.bookCollections).update(id, { 'books-': book })
    await load()
  }

  /**
   * Puts the shelf in a given order.
   *
   * The whole list, because that is what an order is: there is no modifier for
   * "third, not fifth". The cost is that a shelf being rearranged in one tab
   * while a book is added in another loses the addition, which is a fair trade
   * for the thing this page exists to do.
   */
  async function reorder(id: string, books: string[]): Promise<void> {
    await pb.collection(Collections.bookCollections).update(id, { books })
    await load()
  }

  /** Starts applying live changes to the loaded shelves. */
  async function subscribe(): Promise<void> {
    if (unsubscribe) return

    unsubscribe = await pb
      .collection(Collections.bookCollections)
      .subscribe<BookCollection>('*', (event) => {
        const existing = collections.value.findIndex((one) => one.id === event.record.id)

        if (event.action === 'delete') {
          if (existing >= 0) collections.value.splice(existing, 1)

          return
        }

        if (existing >= 0) {
          collections.value[existing] = event.record
        } else {
          collections.value.push(event.record)
        }

        collections.value.sort(byName)
      })
  }

  function unsubscribeAll(): void {
    unsubscribe?.()
    unsubscribe = null
  }

  function clear(): void {
    unsubscribeAll()
    collections.value = []
    loaded.value = false
  }

  return {
    collections,
    loading,
    loaded,
    byId,
    byBook,
    load,
    create,
    update,
    remove,
    addBook,
    removeBook,
    reorder,
    subscribe,
    unsubscribe: unsubscribeAll,
    clear,
  }
})
