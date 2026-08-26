<!--
  File:        webui/src/components/DashboardMetrics.vue
  Project:     https://git.obth.eu/atjontv/kosync
  Copyright:   © 2025-2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
-->
<script setup lang="ts">
import { computed } from 'vue'
import { useDocumentsStore } from '@/stores/documents'
import { useStatsStore } from '@/stores/stats'
import { formatDuration } from '@/lib/duration'

const documents = useDocumentsStore()
const stats = useStatsStore()

const totalDocuments = computed(() => documents.documents.length)

const averageProgress = computed(() => {
  const list = documents.documents
  if (list.length === 0) return '0.0'

  const total = list.reduce((sum, doc) => sum + (doc.progress || 0), 0)
  return ((total / list.length) * 100).toFixed(1)
})

/**
 * The reading of the loaded range, written the way the book page writes it.
 * A month of it runs to thousands of minutes, which nobody reads as an amount
 * of time.
 */
const readingTime = computed(() => formatDuration(stats.readingSeconds))
</script>

<template>
  <div class="grid grid-cols-1 md:grid-cols-3 gap-4">
    <Card
      class="bg-surface-0 dark:bg-surface-900 border border-surface-200 dark:border-surface-700 shadow-sm"
    >
      <template #content>
        <div class="flex items-center gap-4">
          <div class="p-3 bg-blue-100 dark:bg-blue-900/40 rounded-lg">
            <i class="pi pi-book text-2xl text-blue-600 dark:text-blue-400"></i>
          </div>
          <div>
            <span class="block text-surface-500 dark:text-surface-400 text-sm font-medium mb-1"
              >Total Documents</span
            >
            <span class="text-2xl font-bold text-surface-900 dark:text-surface-0">{{
              totalDocuments
            }}</span>
          </div>
        </div>
      </template>
    </Card>

    <Card
      class="bg-surface-0 dark:bg-surface-900 border border-surface-200 dark:border-surface-700 shadow-sm"
    >
      <template #content>
        <div class="flex items-center gap-4">
          <div class="p-3 bg-green-100 dark:bg-green-900/40 rounded-lg">
            <i class="pi pi-percentage text-2xl text-green-600 dark:text-green-400"></i>
          </div>
          <div>
            <span class="block text-surface-500 dark:text-surface-400 text-sm font-medium mb-1"
              >Average Progress</span
            >
            <span class="text-2xl font-bold text-surface-900 dark:text-surface-0"
              >{{ averageProgress }}%</span
            >
          </div>
        </div>
      </template>
    </Card>

    <Card
      class="bg-surface-0 dark:bg-surface-900 border border-surface-200 dark:border-surface-700 shadow-sm"
    >
      <template #content>
        <div class="flex items-center gap-4">
          <div class="p-3 bg-orange-100 dark:bg-orange-900/40 rounded-lg">
            <i class="pi pi-clock text-2xl text-orange-600 dark:text-orange-400"></i>
          </div>
          <div>
            <span class="block text-surface-500 dark:text-surface-400 text-sm font-medium mb-1"
              >Recent Read Time</span
            >
            <span class="text-2xl font-bold text-surface-900 dark:text-surface-0">{{
              readingTime
            }}</span>
          </div>
        </div>
      </template>
    </Card>
  </div>
</template>
