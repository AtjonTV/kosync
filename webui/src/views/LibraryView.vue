<!--
  File:        webui/src/views/LibraryView.vue
  Project:     https://git.obth.eu/atjontv/kosync
  Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
-->
<script setup lang="ts">
import { computed, onMounted, onUnmounted } from 'vue'
import BookLibrary from '@/components/BookLibrary.vue'
import { useBooksStore } from '@/stores/books'

const books = useBooksStore()

const summary = computed(() => {
  const count = books.books.length
  if (count === 0) return ''

  return count === 1 ? '1 book' : `${count} books`
})

onMounted(() => {
  books.subscribe()
})

onUnmounted(() => {
  books.unsubscribe()
})
</script>

<template>
  <div class="flex flex-col gap-6">
    <div class="flex justify-between items-center">
      <h1 class="text-3xl">Library</h1>
      <span class="text-surface-500 dark:text-surface-400">{{ summary }}</span>
    </div>

    <BookLibrary />
  </div>
</template>
