<!--
  File:        webui/src/views/HomeView.vue
  Project:     https://git.obth.eu/atjontv/kosync
  Copyright:   © 2025-2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
-->
<script setup lang="ts">
import { onMounted, onUnmounted, watch } from 'vue'
import DashboardMetrics from '@/components/DashboardMetrics.vue'
import DocumentsList from '@/components/DocumentsList.vue'
import ReadStatisticsChart from '@/components/ReadStatisticsChart.vue'
import AuthPanel from '@/components/AuthPanel.vue'
import SetupGuide from '@/components/SetupGuide.vue'
import { useAuthStore } from '@/stores/auth'
import { useDocumentsStore } from '@/stores/documents'
import { useStatsStore } from '@/stores/stats'

const auth = useAuthStore()
const documents = useDocumentsStore()
const stats = useStatsStore()

const start = async () => {
  await Promise.all([documents.load(), stats.load()])
  await Promise.all([documents.subscribe(), stats.subscribe()])
}

const stop = () => {
  documents.unsubscribe()
  stats.stop()
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
    <DocumentsList custom-title="My documents" />
  </template>

  <div v-else class="flex flex-col gap-6">
    <AuthPanel />
    <SetupGuide />
  </div>
</template>
