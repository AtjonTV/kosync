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
const topBarRef = ref<InstanceType<typeof TopBar> | null>(null);
const currentSite = window.location.origin;

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
    <TopBar ref="topBarRef" @login-success="onLoginSuccess" @logout="onLogout" />
    <main class="max-w-7xl mx-auto p-4 md:p-6 lg:p-8 flex flex-col gap-6">
      <template v-if="isLoggedIn">
        <DashboardMetrics />
        <ReadStatisticsChart />
        <DocumentsList customTitle="My documents" />
      </template>
      <div v-else class="flex flex-col gap-6 mt-8">
        <div class="text-center p-12 bg-surface-0 dark:bg-surface-900 rounded-xl border border-surface-200 dark:border-surface-700 shadow-sm">
           <i class="pi pi-lock text-4xl text-surface-400 dark:text-surface-500 mb-4"></i>
           <p class="text-xl text-surface-600 dark:text-surface-400">Please login to see your documents.</p>
        </div>

        <div class="p-8 bg-surface-0 dark:bg-surface-900 rounded-xl border border-surface-200 dark:border-surface-700 shadow-sm">
          <h2 class="text-2xl font-bold mb-4">How to Setup KOReader Sync</h2>
          <ol class="list-decimal list-inside space-y-4 text-surface-700 dark:text-surface-300">
            <li>Set <strong>Custom sync server</strong> to <code>{{ currentSite }}</code></li>
            <li><strong>Register</strong> a new account.</li>
            <li>Enable <strong>automatically keep documents in sync</strong>.</li>
            <li>Set <strong>periodically sync every # pages</strong> to 2.</li>
            <li>Set <strong>Document matching method</strong> to "Binary".</li>
            <li>Login on all devices repeating steps 3-5.</li>
            <li>Login to KOSync <Button label="Login" size="small" @click="topBarRef?.openLogin()" class="ml-2" /></li>
            <li>Read books.</li>
          </ol>
        </div>
      </div>
    </main>
  </div>
</template>
