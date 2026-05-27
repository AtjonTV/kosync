//
// File:        webui/src/App.vue
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//
<script setup lang="ts">
import { RouterView } from 'vue-router'
</script>

<template>
  <Suspense>
    <template #default>
      <div>
        <ConfirmDialog />
        <Toast />
        <RouterView />
      </div>
    </template>

    <template #fallback>
      <ProgressSpinner />
    </template>
  </Suspense>
</template>
