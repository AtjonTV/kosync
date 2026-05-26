//
// File:        src/components/DashboardMetrics.vue
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2025-2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//
<script setup lang="ts">
import { computed } from 'vue';
import { useSyncStore } from '@/stores/sync';

const syncStore = useSyncStore();

const totalDocuments = computed(() => {
    return syncStore.sync.documents.length;
});

const averageProgress = computed(() => {
    const docs = syncStore.sync.documents;
    if (docs.length === 0) return 0;
    const totalProgress = docs.reduce((sum, doc) => sum + (doc.progress || 0), 0);
    return ((totalProgress / docs.length) * 100).toFixed(1);
});

const totalReadingTime = computed(() => {
    const stats = syncStore.sync.statistics;
    if (!stats || stats.length === 0) return 0;
    const totalSeconds = stats.reduce((sum, s) => sum + (s.reading_time || 0), 0);
    return Math.round(totalSeconds / 60);
});
</script>

<template>
  <div class="grid grid-cols-1 md:grid-cols-3 gap-4 mb-8">
    <Card class="bg-surface-0 dark:bg-surface-900 border border-surface-200 dark:border-surface-700 shadow-sm">
      <template #content>
        <div class="flex items-center gap-4">
          <div class="p-3 bg-blue-100 dark:bg-blue-900/40 rounded-lg">
            <i class="pi pi-book text-2xl text-blue-600 dark:text-blue-400"></i>
          </div>
          <div>
            <span class="block text-surface-500 dark:text-surface-400 text-sm font-medium mb-1">{{ $t('total_documents') }}</span>
            <span class="text-2xl font-bold text-surface-900 dark:text-surface-0">{{ totalDocuments }}</span>
          </div>
        </div>
      </template>
    </Card>

    <Card class="bg-surface-0 dark:bg-surface-900 border border-surface-200 dark:border-surface-700 shadow-sm">
      <template #content>
        <div class="flex items-center gap-4">
          <div class="p-3 bg-green-100 dark:bg-green-900/40 rounded-lg">
            <i class="pi pi-percentage text-2xl text-green-600 dark:text-green-400"></i>
          </div>
          <div>
            <span class="block text-surface-500 dark:text-surface-400 text-sm font-medium mb-1">{{ $t('average_progress') }}</span>
            <span class="text-2xl font-bold text-surface-900 dark:text-surface-0">{{ averageProgress }}%</span>
          </div>
        </div>
      </template>
    </Card>

    <Card class="bg-surface-0 dark:bg-surface-900 border border-surface-200 dark:border-surface-700 shadow-sm">
      <template #content>
        <div class="flex items-center gap-4">
          <div class="p-3 bg-orange-100 dark:bg-orange-900/40 rounded-lg">
            <i class="pi pi-clock text-2xl text-orange-600 dark:text-orange-400"></i>
          </div>
          <div>
            <span class="block text-surface-500 dark:text-surface-400 text-sm font-medium mb-1">{{ $t('recent_read_time') }}</span>
            <span class="text-2xl font-bold text-surface-900 dark:text-surface-0">{{ totalReadingTime }} {{ $t('minutes_abbr') }}</span>
          </div>
        </div>
      </template>
    </Card>
  </div>
</template>