//
// File:        src/components/ReadStatisticsChart.vue
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2025-2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//
<script setup lang="ts">
import { ref, onMounted } from 'vue';
import Chart from 'primevue/chart';
import { useSyncStore } from '@/stores/sync';

const syncStore = useSyncStore();
const chartData = ref();
const chartOptions = ref();

const showDays = 14;

const loadData = async () => {
  const stats = await syncStore.getReadStatistics(showDays);

  chartData.value = {
    labels: stats.map(s => s.date),
    datasets: [
      {
        label: 'Updates',
        data: stats.map(s => s.update_count),
        fill: false,
        borderColor: '#10b981',
        tension: 0.4,
        yAxisID: 'y'
      },
      {
        label: 'Progress Increase (%)',
        data: stats.map(s => s.progress_increase),
        fill: true,
        borderColor: '#3b82f6',
        backgroundColor: 'rgba(59, 130, 246, 0.2)',
        tension: 0.4,
        yAxisID: 'y1'
      }
    ]
  };

  chartOptions.value = setChartOptions();
};

const setChartOptions = () => {
  const documentStyle = getComputedStyle(document.documentElement);
  const textColor = documentStyle.getPropertyValue('--p-text-color').trim() || '#4b5563';
  const textColorSecondary = documentStyle.getPropertyValue('--p-text-muted-color').trim() || '#6b7280';
  const surfaceBorder = documentStyle.getPropertyValue('--p-content-border-color').trim() || '#e5e7eb';

  return {
    stacked: false,
    maintainAspectRatio: false,
    aspectRatio: 0.6,
    plugins: {
      legend: {
        labels: {
          color: textColor
        }
      }
    },
    scales: {
      x: {
        ticks: {
          color: textColorSecondary
        },
        grid: {
          color: surfaceBorder
        }
      },
      y: {
        type: 'linear',
        display: true,
        position: 'left',
        ticks: {
          color: textColorSecondary
        },
        grid: {
          color: surfaceBorder
        },
        title: {
          display: true,
          text: 'Number of Updates',
          color: textColor
        }
      },
      y1: {
        type: 'linear',
        display: true,
        position: 'right',
        ticks: {
          color: textColorSecondary
        },
        grid: {
          drawOnChartArea: false,
          color: surfaceBorder
        },
        title: {
          display: true,
          text: 'Progress Increase (%)',
          color: textColor
        }
      }
    }
  };
};

onMounted(loadData);
</script>

<template>
  <Card class="mb-8">
    <template #title>
      <span class="text-xl font-semibold">Reading Statistics (Last {{showDays}} Days)</span>
    </template>
    <template #content>
      <div class="h-64">
        <Chart type="line" :data="chartData" :options="chartOptions" class="h-full w-full" />
      </div>
    </template>
  </Card>
</template>
