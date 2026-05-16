//
// File:        src/views/HomeView.vue
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2025-2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//
<script setup lang="ts">
import DashboardMetrics from "@/components/DashboardMetrics.vue";
import DocumentsList from "@/components/DocumentsList.vue";
import ReadStatisticsChart from "@/components/ReadStatisticsChart.vue";
import TopBar from "@/components/TopBar.vue";
import {useUserStore} from "@/stores/user.ts";
import {ref} from "vue";

const userStore = useUserStore();

const isLoggedIn = ref(false);
userStore.isLoggedIn().then(status => {
    isLoggedIn.value = status;
});

const onLoginSuccess = () => {
  isLoggedIn.value = true;
}

const onLogout = () => {
  isLoggedIn.value = false;
}
</script>

<template>
  <div class="min-h-screen bg-surface-50 dark:bg-surface-950">
    <TopBar @login-success="onLoginSuccess" @logout="onLogout" />
    <main class="max-w-7xl mx-auto p-4 md:p-6 lg:p-8 flex flex-col gap-6">
      <template v-if="isLoggedIn">
        <DashboardMetrics />
        <ReadStatisticsChart />
        <DocumentsList customTitle="My documents" />
      </template>
      <div v-else class="text-center p-12 bg-surface-0 dark:bg-surface-900 rounded-xl border border-surface-200 dark:border-surface-700 shadow-sm mt-8">
         <i class="pi pi-lock text-4xl text-surface-400 dark:text-surface-500 mb-4"></i>
         <p class="text-xl text-surface-600 dark:text-surface-400">Please login to see your documents.</p>
      </div>
    </main>
  </div>
</template>
