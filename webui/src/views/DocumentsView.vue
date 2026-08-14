<!--
  File:        webui/src/views/DocumentsView.vue
  Project:     https://git.obth.eu/atjontv/kosync
  Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
-->
<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useToast } from 'primevue/usetoast'
import DocumentsList from '@/components/DocumentsList.vue'
import { useDocumentsStore } from '@/stores/documents'
import { useBooksStore } from '@/stores/books'
import { errorMessage } from '@/pb'
import type { FileUploadUploaderEvent } from 'primevue/fileupload'

const documents = useDocumentsStore()
const books = useBooksStore()
const toast = useToast()

// One control for both lists on the page, so switching to a table does not have
// to be done twice.
const viewMode = ref('Grid')
const viewOptions = ref(['Grid', 'List'])

const uploading = ref(false)
const failures = ref<string[]>([])

/**
 * The documents with no uploaded book behind them.
 *
 * This is what the page is for. A document here gets no cover, no measured page
 * count and no book statistics, and the only way to change that is to upload the
 * EPUB it was read from — so it leads, and the matched ones follow.
 */
const missing = computed(() => documents.documents.filter((document) => !document.book))

const matched = computed(() => documents.documents.filter((document) => document.book))

/**
 * Uploads EPUBs from this page.
 *
 * The same upload as the library, offered here because this is where a person
 * finds out that something is missing from it.
 */
const uploadFiles = async (event: FileUploadUploaderEvent) => {
  const chosen = Array.isArray(event.files) ? event.files : [event.files]

  uploading.value = true
  failures.value = []
  let added = 0

  try {
    for (const file of chosen) {
      try {
        await books.upload(file)
        added += 1
      } catch (error) {
        failures.value.push(`${file.name}: ${errorMessage(error, 'could not be uploaded.')}`)
      }
    }
  } finally {
    uploading.value = false
  }

  if (added > 0) {
    toast.add({
      severity: 'success',
      summary: added === 1 ? 'Book added' : `${added} books added`,
      detail: 'Anything it matches moves into your library on its own.',
      life: 5000,
    })
  }
}

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
    <div class="flex justify-between items-center gap-4">
      <div>
        <h1 class="text-3xl">Documents</h1>
        <p class="text-surface-500 dark:text-surface-400">
          Everything your devices have reported reading, whether or not the book is on the server.
          Two entries that are the same book read from two different copies of the file can be
          merged into one.
        </p>
      </div>
      <SelectButton v-model="viewMode" :options="viewOptions" :allow-empty="false" />
    </div>

    <Message v-if="uploading" severity="info">Uploading…</Message>

    <Message v-if="failures.length" severity="error" closable @close="failures = []">
      <div class="flex flex-col gap-1">
        <span v-for="failure in failures" :key="failure">{{ failure }}</span>
      </div>
    </Message>

    <Card
      v-if="missing.length"
      class="bg-surface-0 dark:bg-surface-900 border border-surface-200 dark:border-surface-700"
    >
      <template #title>
        <div class="flex justify-between items-center gap-4">
          <span class="text-xl font-semibold">
            Not in your library
            <span class="text-surface-500 dark:text-surface-400">({{ missing.length }})</span>
          </span>
          <FileUpload
            mode="basic"
            name="file"
            accept=".epub,application/epub+zip"
            :multiple="true"
            :auto="true"
            :custom-upload="true"
            :disabled="uploading"
            choose-label="Add EPUB"
            choose-icon="pi pi-upload"
            @uploader="uploadFiles"
          />
        </div>
      </template>
      <template #content>
        <p class="mb-4 text-surface-600 dark:text-surface-400">
          These have been read on a device, but no EPUB here matches them, so there is no cover, no
          page count and no per-book statistics for them. Upload the very file you read: the match
          is made on the file's contents, so another copy of the same title will not do. Anything
          that matches moves into your library on its own, keeping the reading it already has.
        </p>

        <DocumentsList :documents="missing" :view-mode="viewMode" />
      </template>
    </Card>

    <Message v-else-if="documents.documents.length" severity="success">
      Every document has its book in your library.
    </Message>

    <Card
      class="bg-surface-0 dark:bg-surface-900 border border-surface-200 dark:border-surface-700"
    >
      <template #title>
        <div class="flex justify-between items-center gap-4">
          <span class="text-xl font-semibold">
            In your library
            <span class="text-surface-500 dark:text-surface-400">({{ matched.length }})</span>
          </span>
          <RouterLink :to="{ name: 'library' }" class="text-sm hover:underline">
            Go to the library
          </RouterLink>
        </div>
      </template>
      <template #content>
        <p class="mb-4 text-surface-600 dark:text-surface-400">
          Matched to an uploaded book. The library is the better place to read these — this list is
          here for the things underneath, like the position a device last pushed and the history
          behind it.
        </p>

        <DocumentsList
          :documents="matched"
          :view-mode="viewMode"
          empty-message="Nothing has been matched to a book yet."
        />
      </template>
    </Card>
  </div>
</template>
