<script setup lang="ts">
import { RouterView } from 'vue-router'
</script>

<template>
  <Suspense>
    <template #default>
      <div>
        <ConfirmDialog />
        <RouterView />
      </div>
    </template>

    <template #fallback>
      <ProgressSpinner />
    </template>
  </Suspense>
</template>
