//
// File:        webui/src/stores/achievements.ts
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

import { computed, ref } from 'vue'
import { defineStore } from 'pinia'
import { pb, Collections, KosyncApi } from '@/pb'
import type { Achievement } from '@/models'

/**
 * What the account has been recognised for, and what is still to come.
 *
 * The rules are not defined here. They come from the server with the progress
 * in one response, because they are code — "how many nights did you read past
 * midnight" is a timezone conversion, not a column — and a copy of their names
 * and thresholds in the browser would be a second place for the two to disagree
 * from.
 */
export const useAchievementsStore = defineStore('achievements', () => {
  const achievements = ref<Achievement[]>([])
  const loading = ref(false)
  const loaded = ref(false)

  let unsubscribe: (() => void) | null = null

  /** The ones with at least one tier, most recently earned first. */
  const earned = computed(() =>
    achievements.value
      .filter((entry) => entry.tier > 0)
      .slice()
      .sort((a, b) => latestOf(b).localeCompare(latestOf(a))),
  )

  const pending = computed(() => achievements.value.filter((entry) => entry.tier === 0))

  function latestOf(entry: Achievement): string {
    return entry.earned.reduce((latest, one) => (one.at > latest ? one.at : latest), '')
  }

  async function load(): Promise<void> {
    loading.value = true
    try {
      const response = await pb.send<{ achievements?: Achievement[] }>(KosyncApi.achievements, {
        method: 'GET',
      })
      achievements.value = response?.achievements ?? []
      loaded.value = true
    } finally {
      loading.value = false
    }
  }

  /**
   * Reloads when a new tier is awarded.
   *
   * The record that arrives says only which tier was earned, while the card
   * shows progress towards the next one as well, so the whole thing is asked for
   * again rather than patched from the event.
   */
  async function subscribe(): Promise<void> {
    if (unsubscribe) return

    unsubscribe = await pb.collection(Collections.achievements).subscribe('*', () => {
      void load()
    })
  }

  function unsubscribeAll(): void {
    unsubscribe?.()
    unsubscribe = null
  }

  function clear(): void {
    unsubscribeAll()
    achievements.value = []
    loaded.value = false
  }

  return {
    achievements,
    earned,
    pending,
    loading,
    loaded,
    load,
    subscribe,
    unsubscribe: unsubscribeAll,
    clear,
  }
})
