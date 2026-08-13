<!--
  File:        webui/src/views/AccountView.vue
  Project:     https://git.obth.eu/atjontv/kosync
  Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
-->
<script setup lang="ts">
import KoreaderAccounts from '@/components/KoreaderAccounts.vue'
import SetupGuide from '@/components/SetupGuide.vue'
import { useAuthStore } from '@/stores/auth'

const auth = useAuthStore()
</script>

<template>
  <div class="flex flex-col gap-6">
    <div class="flex justify-between items-center">
      <h1 class="text-3xl">Account</h1>
      <span class="text-surface-500 dark:text-surface-400">{{ auth.email }}</span>
    </div>

    <KoreaderAccounts />
    <SetupGuide />
  </div>
</template>
