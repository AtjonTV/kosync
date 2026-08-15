<!--
  File:        webui/src/components/AchievementList.vue
  Project:     https://git.obth.eu/atjontv/kosync
  Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
-->
<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useAchievementsStore } from '@/stores/achievements'
import AchievementBadge from '@/components/AchievementBadge.vue'
import { errorMessage } from '@/pb'
import type { Achievement } from '@/models'

const store = useAchievementsStore()
const loadFailure = ref('')

// Earned first, then what is still to come. Both are shown: a badge nobody has
// yet is the only thing on the card that says what there is to aim at.
const ordered = computed(() => [...store.earned, ...store.pending])

const tierNames = ['Bronze', 'Silver', 'Gold']

const tierName = (entry: Achievement) => tierNames[entry.tier - 1] ?? ''

/** How far through the current tier the reading has got, as a percentage. */
const towardsNext = (entry: Achievement) => {
  if (!entry.next) return 100

  const floor = entry.tier > 0 ? (entry.tiers[entry.tier - 1] ?? 0) : 0
  const span = entry.next - floor
  if (span <= 0) return 0

  return Math.max(0, Math.min(100, ((entry.value - floor) / span) * 100))
}

const progressLabel = (entry: Achievement) => {
  if (!entry.next) return `${format(entry.value)} ${entry.unit} — every tier earned`

  return `${format(entry.value)} of ${format(entry.next)} ${entry.unit}`
}

const format = (value: number) => value.toLocaleString()

onMounted(async () => {
  try {
    if (!store.loaded) await store.load()
    await store.subscribe()
  } catch (error) {
    loadFailure.value = errorMessage(error, 'Your achievements could not be loaded.')
  }
})
onUnmounted(() => store.unsubscribe())
</script>

<template>
  <Card class="bg-surface-0 dark:bg-surface-900 border border-surface-200 dark:border-surface-700">
    <template #title>
      <div class="flex justify-between items-center gap-4">
        <span class="text-xl font-semibold">Achievements</span>
        <span class="text-sm text-surface-500 dark:text-surface-400">
          {{ store.earned.length }} of {{ store.achievements.length }}
        </span>
      </div>
    </template>

    <template #content>
      <Message v-if="loadFailure" severity="error" class="m-2">{{ loadFailure }}</Message>

      <div v-else-if="!store.loaded" class="p-8 flex justify-center">
        <ProgressSpinner style="width: 2.5rem; height: 2.5rem" aria-label="Loading achievements" />
      </div>

      <div v-else-if="ordered.length" class="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-4 gap-5">
        <div
          v-for="entry in ordered"
          :key="entry.rule"
          class="flex flex-col items-center text-center gap-2"
        >
          <div class="w-24">
            <AchievementBadge
              :icon="entry.icon"
              :fur="entry.fur"
              :tier="entry.tier"
              :label="entry.name"
            />
          </div>

          <div class="font-semibold leading-tight">{{ entry.name }}</div>

          <div v-if="entry.tier > 0" class="text-xs text-surface-500 dark:text-surface-400">
            {{ tierName(entry) }}
          </div>
          <div v-else class="text-xs text-surface-400 dark:text-surface-500">Not yet</div>

          <div class="w-full">
            <ProgressBar
              :value="towardsNext(entry)"
              :show-value="false"
              style="height: 5px"
            ></ProgressBar>
            <div class="mt-1 text-xs text-surface-500 dark:text-surface-400 tabular-nums">
              {{ progressLabel(entry) }}
            </div>
          </div>

          <div class="text-xs text-surface-400 dark:text-surface-500">{{ entry.summary }}</div>
        </div>
      </div>

      <p v-else class="p-4 text-center text-surface-500 dark:text-surface-400">
        Nothing to show yet. Sync some reading from a device and this fills in.
      </p>
    </template>
  </Card>
</template>
