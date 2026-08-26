//
// File:        webui/src/stores/bookStats.ts
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

import { computed, ref } from 'vue'
import { defineStore } from 'pinia'
import { pb, Collections } from '@/pb'
import type { ReadingBookDay } from '@/models'

/**
 * The per-book daily statistics.
 *
 * The server computes one row per book and day, so this store only reads. The
 * numbers are deliberately not the day totals split up: the reading time of a
 * day includes the gaps spent switching between books, which belong to no book
 * at all, so a book's times add up to less than the day's and are meant to.
 */
export const useBookStatsStore = defineStore('bookStats', () => {
  const days = ref<ReadingBookDay[]>([])
  const loading = ref(false)
  const bookId = ref('')

  let unsubscribe: (() => void) | null = null

  /** Loads every day recorded for one book, oldest first. */
  async function load(id: string): Promise<void> {
    loading.value = true
    bookId.value = id
    try {
      days.value = await pb.collection(Collections.readingBookDays).getFullList<ReadingBookDay>({
        filter: pb.filter('book = {:book}', { book: id }),
        sort: 'date',
      })
    } finally {
      loading.value = false
    }
  }

  /**
   * Starts applying live changes for the loaded book.
   *
   * The subscription covers the whole collection because PocketBase filters it
   * by the collection's own list rule, not by a client filter; rows for other
   * books are dropped here.
   */
  async function subscribe(): Promise<void> {
    if (unsubscribe) return

    unsubscribe = await pb
      .collection(Collections.readingBookDays)
      .subscribe<ReadingBookDay>('*', (event) => {
        if (event.record.book !== bookId.value) return

        const index = days.value.findIndex((day) => day.id === event.record.id)

        if (event.action === 'delete') {
          if (index !== -1) days.value.splice(index, 1)

          return
        }

        if (index === -1) {
          days.value.push(event.record)
          days.value.sort((a, b) => a.date.localeCompare(b.date))
        } else {
          days.value[index] = event.record
        }
      })
  }

  function stop(): void {
    unsubscribe?.()
    unsubscribe = null
  }

  /** Everything the book's own rows add up to. */
  const totals = computed(() => ({
    days: days.value.length,
    readingTime: days.value.reduce((sum, day) => sum + (day.reading_time || 0), 0),
    pagesRead: days.value.reduce((sum, day) => sum + (day.pages_read || 0), 0),
    updates: days.value.reduce((sum, day) => sum + (day.update_count || 0), 0),
    first: days.value.at(0)?.date ?? '',
    last: days.value.at(-1)?.date ?? '',
  }))

  /** The day the book saw the most reading, by time. */
  const bestDay = computed(() => {
    let best: ReadingBookDay | null = null

    for (const day of days.value) {
      if (!best || day.reading_time > best.reading_time) best = day
    }

    return best
  })

  function clear(): void {
    stop()
    days.value = []
    bookId.value = ''
  }

  return { days, loading, bookId, totals, bestDay, load, subscribe, stop, clear }
})
