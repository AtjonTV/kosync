<!--
  File:        webui/src/components/ReadStatisticsChart.vue
  Project:     https://git.obth.eu/atjontv/kosync
  Copyright:   © 2025-2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
-->
<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import Chart from 'primevue/chart'
import { useStatsStore } from '@/stores/stats'

const stats = useStatsStore()
const showDays = ref(14)

const chartData = computed(() => {
  const series = stats.series
  if (series.length === 0) return null

  return {
    labels: series.map((entry) => entry.date),
    datasets: [
      {
        label: 'Updates',
        data: series.map((entry) => entry.updates),
        fill: false,
        borderColor: '#10b981',
        tension: 0.4,
        yAxisID: 'y',
      },
      {
        label: 'Progress Increase (%)',
        data: series.map((entry) => entry.increase),
        fill: true,
        borderColor: '#3b82f6',
        backgroundColor: 'rgba(59, 130, 246, 0.2)',
        tension: 0.4,
        yAxisID: 'y1',
      },
      {
        label: 'Reading Time (min)',
        data: series.map((entry) => Math.round((entry.readingTime / 60) * 10) / 10),
        fill: false,
        borderColor: '#f59e0b',
        tension: 0.4,
        yAxisID: 'y2',
      },
    ],
  }
})

const chartOptions = ref()

const setChartOptions = () => {
  const documentStyle = getComputedStyle(document.documentElement)
  const textColor = documentStyle.getPropertyValue('--p-text-color').trim() || '#4b5563'
  const textColorSecondary =
    documentStyle.getPropertyValue('--p-text-muted-color').trim() || '#6b7280'
  const surfaceBorder =
    documentStyle.getPropertyValue('--p-content-border-color').trim() || '#e5e7eb'

  return {
    stacked: false,
    maintainAspectRatio: false,
    aspectRatio: 0.6,
    plugins: {
      legend: {
        labels: {
          color: textColor,
        },
      },
    },
    scales: {
      x: {
        ticks: { color: textColorSecondary },
        grid: { color: surfaceBorder },
      },
      y: {
        type: 'linear',
        display: true,
        position: 'left',
        ticks: { color: textColorSecondary },
        grid: { color: surfaceBorder },
        title: { display: true, text: 'Number of Updates', color: textColor },
      },
      y1: {
        type: 'linear',
        display: true,
        position: 'right',
        ticks: { color: textColorSecondary },
        grid: { drawOnChartArea: false, color: surfaceBorder },
        title: { display: true, text: 'Progress Increase (%)', color: textColor },
      },
      y2: {
        type: 'linear',
        display: true,
        position: 'right',
        ticks: { color: textColorSecondary },
        grid: { drawOnChartArea: false, color: surfaceBorder },
        title: { display: true, text: 'Reading Time (min)', color: textColor },
      },
    },
  }
}

watch(showDays, async (range) => {
  await stats.load(range)
})

onMounted(() => {
  chartOptions.value = setChartOptions()
})
</script>

<template>
  <Card class="bg-surface-0 dark:bg-surface-900 border border-surface-200 dark:border-surface-700">
    <template #title>
      <div class="flex justify-between items-center">
        <span class="text-xl font-semibold">Reading Statistics (Last {{ showDays }} Days)</span>
        <SelectButton v-model="showDays" :options="[7, 14, 30, 60]" :allow-empty="false" />
      </div>
    </template>
    <template #content>
      <div class="h-64">
        <Chart
          v-if="chartData"
          type="line"
          :data="chartData"
          :options="chartOptions"
          class="h-full w-full"
        />
        <div v-else class="flex items-center justify-center h-full">
          <span>Loading statistics...</span>
        </div>
      </div>
    </template>
  </Card>
</template>
