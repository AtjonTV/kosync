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
import { formatBytes } from '@/pb'

const books = useBooksStore()
const documents = useDocumentsStore()

const summary = computed(() => {
  const count = books.books.length
  if (count === 0) return ''

  return count === 1 ? '1 book' : `${count} books`
})

// Only shown on an instance that has a limit. Without one there is nothing to
// be a fraction of, and a bar with no end would be decoration.
const used = computed(() => books.usage?.used ?? 0)
const quota = computed(() => books.usage?.quota ?? 0)
const percentUsed = computed(() =>
  quota.value > 0 ? Math.min(Math.round((used.value / quota.value) * 100), 100) : 0,
)
const storageLabel = computed(() => `${formatBytes(used.value)} of ${formatBytes(quota.value)}`)

// Amber before it is a problem, so that the first anybody hears of a full
// library is not an upload being refused. ProgressBar has no severity of its
// own, so the colour is set on the filled part through the pass-through class.
const severityClass = computed(() => {
  if (percentUsed.value >= 95) return '!bg-red-500'
  if (percentUsed.value >= 80) return '!bg-amber-500'

  return ''
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

    <div v-if="books.limited" class="flex flex-col gap-2">
      <div class="flex justify-between items-baseline text-sm">
        <span class="text-surface-600 dark:text-surface-400">Storage</span>
        <span class="text-surface-500 dark:text-surface-400 tabular-nums">{{ storageLabel }}</span>
      </div>
      <ProgressBar
        :value="percentUsed"
        :show-value="false"
        :pt="{ value: { class: severityClass } }"
        style="height: 0.5rem"
        :aria-label="`Storage: ${storageLabel}`"
      />
    </div>

    <BookLibrary heading="" />
  </div>
</template>
