//
// File:        src/components/ReadStatisticsChart.vue
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2025-2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//
<script setup lang="ts">
import { ref, onMounted, computed, watch } from 'vue';
import Chart from 'primevue/chart';
import { useSyncStore } from '@/stores/sync';
import { useI18nStore } from '@/stores/i18n';

const syncStore = useSyncStore();
const i18nStore = useI18nStore();
const showDays = ref(14);

const chartData = computed(() => {
  const stats = syncStore.sync.statistics.slice(-showDays.value);
  if (!stats || stats.length === 0) return null;

  return {
    labels: stats.map(s => s.date),
    datasets: [
      {
        label: i18nStore.t('chart_updates'),
        data: stats.map(s => s.update_count),
        fill: false,
        borderColor: '#10b981',
        tension: 0.4,
        yAxisID: 'y'
      },
      {
        label: i18nStore.t('chart_progress_increase'),
        data: stats.map(s => s.progress_increase),
        fill: true,
        borderColor: '#3b82f6',
        backgroundColor: 'rgba(59, 130, 246, 0.2)',
        tension: 0.4,
        yAxisID: 'y1'
      },
      {
        label: i18nStore.t('chart_reading_time'),
        data: stats.map(s => Math.round((s.reading_time || 0) / 60 * 10) / 10),
        fill: false,
        borderColor: '#f59e0b',
        tension: 0.4,
        yAxisID: 'y2'
      }
    ]
  };
});
const chartOptions = ref();

const loadData = async () => {
  if (syncStore.sync.statistics.length === 0) {
    await syncStore.doSync();
  }
  chartOptions.value = setChartOptions();
};

watch([showDays, () => i18nStore.locale], async ([newVal, _]) => {
  if (syncStore.sync.statistics.length < newVal) {
    await syncStore.doSync(true, newVal);
  }
  chartOptions.value = setChartOptions();
});

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
          text: i18nStore.t('chart_number_of_updates'),
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
          text: i18nStore.t('chart_progress_increase'),
          color: textColor
        }
      },
      y2: {
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
          text: i18nStore.t('chart_reading_time'),
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
      <div class="flex justify-between items-center">
        <span class="text-xl font-semibold">{{ $t('chart_title', showDays) }}</span>
        <SelectButton v-model="showDays" :options="[7, 14, 30, 60]" :unselectable="false" />
      </div>
    </template>
    <template #content>
      <div class="h-64">
        <Chart v-if="chartData" type="line" :data="chartData" :options="chartOptions" class="h-full w-full" />
        <div v-else class="flex items-center justify-center h-full">
          <span>{{ $t('chart_loading') }}</span>
        </div>
      </div>
    </template>
  </Card>
</template>
