<!--
  File:        webui/src/views/HomeView.vue
  Project:     https://git.obth.eu/atjontv/kosync
  Copyright:   © 2025-2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
-->
<script setup lang="ts">
import { onMounted, onUnmounted, watch } from 'vue'
import DashboardMetrics from '@/components/DashboardMetrics.vue'
import BookLibrary from '@/components/BookLibrary.vue'
import ReadStatisticsChart from '@/components/ReadStatisticsChart.vue'
import AuthPanel from '@/components/AuthPanel.vue'
import SetupGuide from '@/components/SetupGuide.vue'
import { useAuthStore } from '@/stores/auth'
import { useDocumentsStore } from '@/stores/documents'
import { useStatsStore } from '@/stores/stats'
import { useBooksStore } from '@/stores/books'

const auth = useAuthStore()
const documents = useDocumentsStore()
const stats = useStatsStore()
const books = useBooksStore()

// The books are loaded here as well as by the library itself, because the
// covers are the main thing on this page now and should not appear a beat after
// everything else. The documents come too: the progress shown on a cover lives
// on the document, not the book.
const start = async () => {
  await Promise.all([documents.load(), stats.load(), books.load()])
  await Promise.all([documents.subscribe(), stats.subscribe(), books.subscribe()])
}

const stop = () => {
  documents.unsubscribe()
  stats.stop()
  books.unsubscribe()
}

watch(
  () => auth.isValid,
  async (signedIn) => {
    if (signedIn) {
      await start()
    } else {
      stop()
    }
  },
)

onMounted(async () => {
  if (auth.isValid && (await auth.refresh())) {
    await start()
  }
})

onUnmounted(stop)
</script>

<template>
  <template v-if="auth.isValid">
    <DashboardMetrics />
    <ReadStatisticsChart />
    <BookLibrary :limit="6" />
  </template>

  <div v-else class="flex flex-col gap-6">
    <AuthPanel />
    <SetupGuide />
  </div>
</template>
