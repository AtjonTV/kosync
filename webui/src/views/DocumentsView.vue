<!--
  File:        webui/src/views/DocumentsView.vue
  Project:     https://git.obth.eu/atjontv/kosync
  Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
-->
<script setup lang="ts">
import { computed, onMounted, onUnmounted } from 'vue'
import DocumentsList from '@/components/DocumentsList.vue'
import { useDocumentsStore } from '@/stores/documents'
import { useBooksStore } from '@/stores/books'

const documents = useDocumentsStore()
const books = useBooksStore()

/**
 * Documents with no uploaded book behind them.
 *
 * These are the ones that get no cover, no page count and no book statistics,
 * and the only way to change that is to upload the file they were read from —
 * so the number is worth saying out loud rather than leaving to be noticed.
 */
const unmatched = computed(() => documents.documents.filter((document) => !document.book).length)

const start = async () => {
  await Promise.all([documents.load(), books.load()])
  await Promise.all([documents.subscribe(), books.subscribe()])
}

const stop = () => {
  documents.unsubscribe()
  books.unsubscribe()
}

onMounted(start)
onUnmounted(stop)
</script>

<template>
  <div class="flex flex-col gap-6">
    <div class="flex justify-between items-center">
      <h1 class="text-3xl">Documents</h1>
      <span class="text-surface-500 dark:text-surface-400">
        {{
          documents.documents.length === 1
            ? '1 document'
            : `${documents.documents.length} documents`
        }}
      </span>
    </div>

    <Message v-if="unmatched" severity="info">
      {{ unmatched === 1 ? '1 document has' : `${unmatched} documents have` }} no book on the
      server. Upload the EPUB you read on the device and KOsync will recognise it — the match is
      made on the file's contents, so it has to be that copy.
    </Message>

    <DocumentsList custom-title="My documents" />
  </div>
</template>
