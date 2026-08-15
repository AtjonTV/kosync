<!--
  File:        webui/src/App.vue
  Project:     https://git.obth.eu/atjontv/kosync
  Copyright:   © 2025-2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
-->
<script setup lang="ts">
import { RouterView } from 'vue-router'
import TopBar from '@/components/TopBar.vue'
import AchievementSprite from '@/components/AchievementSprite.vue'
</script>

<template>
  <div class="min-h-screen bg-surface-50 dark:bg-surface-950">
    <!-- Mounted once, so every badge on every page is a <use> of one drawing. -->
    <AchievementSprite />
    <ConfirmDialog />
    <Toast />
    <TopBar />
    <main class="max-w-7xl mx-auto p-4 md:p-6 lg:p-8 flex flex-col gap-6">
      <RouterView />
    </main>
  </div>
</template>
