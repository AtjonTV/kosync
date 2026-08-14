<!--
  File:        webui/src/views/LibraryView.vue
  Project:     https://git.obth.eu/atjontv/kosync
  Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
-->
<script setup lang="ts">
import { computed, onMounted, onUnmounted } from 'vue'
import BookLibrary from '@/components/BookLibrary.vue'
import { useBooksStore } from '@/stores/books'
import { useDocumentsStore } from '@/stores/documents'

const books = useBooksStore()
const documents = useDocumentsStore()

const summary = computed(() => {
  const count = books.books.length
  if (count === 0) return ''

  return count === 1 ? '1 book' : `${count} books`
})

// The documents are subscribed to as well as the books, because the reading
// progress shown on a cover lives on the document, and the server moves it:
// uploading a book links the documents that were already recording progress
// through it. Without this subscription that link only becomes visible on a
// full page reload.
const start = async () => {
  await Promise.all([books.subscribe(), documents.subscribe()])
}

const stop = () => {
  books.unsubscribe()
  documents.unsubscribe()
}

onMounted(start)
onUnmounted(stop)
</script>

<template>
  <div class="flex flex-col gap-6">
    <div class="flex justify-between items-center">
      <h1 class="text-3xl">Library</h1>
      <span class="text-surface-500 dark:text-surface-400">{{ summary }}</span>
    </div>

    <BookLibrary heading="" />
  </div>
</template>
