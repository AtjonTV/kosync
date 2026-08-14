<!--
  File:        webui/src/components/BookLibrary.vue
  Project:     https://git.obth.eu/atjontv/kosync
  Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
-->
<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useConfirm } from 'primevue/useconfirm'
import { useToast } from 'primevue/usetoast'
import { useBooksStore } from '@/stores/books'
import type { Book } from '@/models'
import type { FileUploadUploaderEvent } from 'primevue/fileupload'
import { errorMessage, fileUrl } from '@/pb'

const books = useBooksStore()
const confirm = useConfirm()
const toast = useToast()

const uploading = ref(false)
const failures = ref<string[]>([])

const showRename = ref(false)
const renameTarget = ref<Book | null>(null)
const newTitle = ref('')
const renameError = ref('')
const busy = ref(false)

const sorted = computed(() => [...books.books].sort((a, b) => a.title.localeCompare(b.title)))

const coverUrl = (book: Book) => fileUrl(book, book.cover, '200x300')
const downloadUrl = (book: Book) => fileUrl(book, book.file)

const authorsOf = (book: Book) => (book.authors ?? []).join(', ')

const numberFormat = new Intl.NumberFormat()
const formatCount = (value: number) => numberFormat.format(value ?? 0)

/**
 * Uploads the chosen files one at a time.
 *
 * Sequentially, because a browser will happily start a dozen multi-megabyte
 * uploads at once, and because a failure part way through should say which
 * book it was rather than which of several parallel requests.
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
      detail: 'Progress from a device holding the same file will be recognised automatically.',
      life: 5000,
    })
  }
}

const openRename = (book: Book) => {
  renameTarget.value = book
  newTitle.value = book.title
  renameError.value = ''
  showRename.value = true
}

const rename = async () => {
  if (!renameTarget.value) return

  renameError.value = ''
  busy.value = true
  try {
    await books.rename(renameTarget.value.id, newTitle.value.trim())
    showRename.value = false
  } catch (error) {
    renameError.value = errorMessage(error, 'Could not change the title.')
  } finally {
    busy.value = false
  }
}

const remove = (book: Book) => {
  confirm.require({
    message: `Delete "${book.title}"? The file is removed from the server. Reading progress pushed by your devices is kept.`,
    header: 'Confirmation',
    icon: 'pi pi-exclamation-triangle',
    rejectProps: { label: 'Cancel', severity: 'secondary', outlined: true },
    acceptProps: { label: 'Delete', severity: 'danger' },
    accept: async () => {
      try {
        await books.remove(book.id)
      } catch (error) {
        toast.add({
          severity: 'error',
          summary: 'Failed',
          detail: errorMessage(error),
          life: 5000,
        })
      }
    },
  })
}

onMounted(() => {
  books.load()
})
</script>

<template>
  <Card class="bg-surface-0 dark:bg-surface-900 border border-surface-200 dark:border-surface-700">
    <template #title>
      <div class="flex justify-between items-center gap-4">
        <span class="text-xl font-semibold">Library</span>
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
        Books you upload are kept here as a backup, and let KOsync recognise which book a device is
        reporting progress for. Upload the very file you read on the device: the match is made on
        the file's contents, so another copy of the same title will not do.
      </p>

      <Message v-if="uploading" severity="info" class="mb-4">Uploading…</Message>

      <Message v-if="failures.length" severity="error" class="mb-4" closable @close="failures = []">
        <div class="flex flex-col gap-1">
          <span v-for="failure in failures" :key="failure">{{ failure }}</span>
        </div>
      </Message>

      <div v-if="books.loading && !books.loaded" class="p-8 text-center">
        <ProgressSpinner style="width: 2.5rem; height: 2.5rem" />
      </div>

      <div
        v-else-if="sorted.length"
        class="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 xl:grid-cols-6 gap-6"
      >
        <div v-for="book in sorted" :key="book.id" class="flex flex-col gap-2">
          <div
            class="relative aspect-[2/3] rounded-lg overflow-hidden bg-surface-100 dark:bg-surface-800 border border-surface-200 dark:border-surface-700"
          >
            <img
              v-if="book.cover"
              :src="coverUrl(book)"
              :alt="`Cover of ${book.title}`"
              class="w-full h-full object-cover"
              loading="lazy"
            />
            <div
              v-else
              class="w-full h-full flex items-center justify-center text-surface-400 dark:text-surface-500"
            >
              <i class="pi pi-book text-4xl"></i>
            </div>
          </div>

          <div class="flex flex-col gap-1">
            <span class="font-semibold leading-tight" :title="book.title">{{ book.title }}</span>
            <span v-if="authorsOf(book)" class="text-sm text-surface-600 dark:text-surface-400">
              {{ authorsOf(book) }}
            </span>
            <span class="text-xs text-surface-500 dark:text-surface-400">
              {{ formatCount(book.page_count) }} pages · {{ formatCount(book.word_count) }} words
            </span>
          </div>

          <div class="flex gap-1">
            <a :href="downloadUrl(book)" :download="`${book.title}.epub`">
              <Button icon="pi pi-download" variant="text" rounded title="Download" />
            </a>
            <Button
              icon="pi pi-pencil"
              variant="text"
              rounded
              title="Change title"
              @click="openRename(book)"
            />
            <Button
              icon="pi pi-trash"
              severity="danger"
              variant="text"
              rounded
              title="Delete"
              @click="remove(book)"
            />
          </div>
        </div>
      </div>

      <div v-else class="p-8 text-center text-surface-500 dark:text-surface-400">
        No books yet. Add an EPUB to keep a copy here.
      </div>
    </template>
  </Card>

  <Dialog v-model:visible="showRename" header="Change the title" modal :style="{ width: '28rem' }">
    <form class="flex flex-col gap-4" @submit.prevent="rename">
      <p class="text-surface-600 dark:text-surface-400">
        The title comes from the file's own metadata, which publishers do not always get right.
        Changing it here affects nothing else about the book.
      </p>
      <div class="flex flex-col gap-2">
        <label for="book-title">Title</label>
        <InputText id="book-title" v-model="newTitle" autofocus fluid />
      </div>
      <Message v-if="renameError" severity="error" variant="simple">{{ renameError }}</Message>
      <div class="flex justify-end gap-2">
        <Button type="button" label="Cancel" severity="secondary" @click="showRename = false" />
        <Button type="submit" label="Save" :loading="busy" />
      </div>
    </form>
  </Dialog>
</template>
