//
// File:        webui/src/stores/stats.ts
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

import { computed, ref } from 'vue'
import { defineStore } from 'pinia'
import { pb, Collections } from '@/pb'
import type { ReadingDay } from '@/models'

/** Formats a date the way the statistics are keyed, in UTC. */
export function toDateKey(date: Date): string {
  return date.toISOString().slice(0, 10)
}

/**
 * The reading statistics.
 *
 * The server precomputes one row per day, so this store only reads and
 * subscribes. Days without any reading have no row at all, which is why the
 * chart fills the gaps itself.
 */
export const useStatsStore = defineStore('stats', () => {
  const days = ref<ReadingDay[]>([])
  const loading = ref(false)
  const loadedDays = ref(0)

  let unsubscribe: (() => void) | null = null

  /** Loads the last `range` days, including today. */
  async function load(range = 14): Promise<void> {
    loading.value = true
    try {
      const from = new Date()
      from.setUTCDate(from.getUTCDate() - (range - 1))

      days.value = await pb.collection(Collections.readingDays).getFullList<ReadingDay>({
        filter: pb.filter('date >= {:from}', { from: toDateKey(from) }),
        sort: 'date',
      })
      loadedDays.value = range
    } finally {
      loading.value = false
    }
  }

  /** Starts applying live changes to the loaded statistics. */
  async function subscribe(): Promise<void> {
    if (unsubscribe) return

    unsubscribe = await pb
      .collection(Collections.readingDays)
      .subscribe<ReadingDay>('*', (event) => {
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

  /**
   * The loaded days, with the days without reading filled in as zeroes so the
   * chart shows a continuous timeline.
   */
  const series = computed(() => {
    const byDate = new Map(days.value.map((day) => [day.date, day]))
    const result: { date: string; updates: number; increase: number; readingTime: number }[] = []

    const cursor = new Date()
    cursor.setUTCDate(cursor.getUTCDate() - (loadedDays.value - 1))

    for (let i = 0; i < loadedDays.value; i++) {
      const key = toDateKey(cursor)
      const day = byDate.get(key)
      result.push({
        date: key,
        updates: day?.update_count ?? 0,
        increase: day?.progress_increase ?? 0,
        readingTime: day?.reading_time ?? 0,
      })
      cursor.setUTCDate(cursor.getUTCDate() + 1)
    }

    return result
  })

  /**
   * Total reading time of the loaded range, in seconds.
   *
   * Seconds because that is how every day of it is stored and how the page that
   * shows it wants it: writing the total as hours and minutes is one rounding,
   * and doing it here as well would be a second one.
   */
  const readingSeconds = computed(() =>
    days.value.reduce((sum, day) => sum + (day.reading_time || 0), 0),
  )

  function clear(): void {
    stop()
    days.value = []
    loadedDays.value = 0
  }

  return { days, loading, loadedDays, series, readingSeconds, load, subscribe, stop, clear }
})
