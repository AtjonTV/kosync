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
import { useDocumentsStore } from '@/stores/documents'
import type { Book } from '@/models'
import type { FileUploadUploaderEvent } from 'primevue/fileupload'
import { errorMessage, fileUrl } from '@/pb'

const props = defineProps<{
  /**
   * How many books to show. Unset means all of them, by title, which is what
   * the library page wants. Set, the dashboard gets the ones most recently read
   * and a link to the rest — a shelf, not a catalogue.
   */
  limit?: number
}>()

const books = useBooksStore()
const documents = useDocumentsStore()
const confirm = useConfirm()
const toast = useToast()

const uploading = ref(false)
const failures = ref<string[]>([])

const showRename = ref(false)
const renameTarget = ref<Book | null>(null)
const newTitle = ref('')
const renameError = ref('')
const busy = ref(false)

/** When a book was last read, keyed by book id. */
const lastReadByBook = computed(() => {
  const latest = new Map<string, string>()

  for (const document of documents.documents) {
    if (!document.book) continue

    const best = latest.get(document.book) ?? ''
    if (document.last_read_at > best) latest.set(document.book, document.last_read_at)
  }

  return latest
})

const byTitle = computed(() => [...books.books].sort((a, b) => a.title.localeCompare(b.title)))

const byRecency = computed(() =>
  [...books.books].sort((a, b) => {
    const left = lastReadByBook.value.get(a.id) ?? ''
    const right = lastReadByBook.value.get(b.id) ?? ''
    if (left !== right) return right.localeCompare(left)

    return a.title.localeCompare(b.title)
  }),
)

const sorted = computed(() => {
  if (!props.limit) return byTitle.value

  return byRecency.value.slice(0, props.limit)
})

/** How many books the limit is hiding. */
const hidden = computed(() => (props.limit ? Math.max(books.books.length - props.limit, 0) : 0))

/**
 * How far the reading has got in each book, keyed by book id.
 *
 * The link is made by the server: a device pushes a document hash, and a book
 * carrying that hash claims it. Nothing here needs to know how that is done.
 */
const progressByBook = computed(() => {
  const furthest = new Map<string, number>()

  for (const document of documents.documents) {
    if (!document.book) continue

    const best = furthest.get(document.book) ?? 0
    if (document.progress > best) furthest.set(document.book, document.progress)
  }

  return furthest
})

const progressOf = (book: Book) => progressByBook.value.get(book.id)
const percentOf = (book: Book) => Math.round((progressOf(book) ?? 0) * 100)

const coverUrl = (book: Book) => fileUrl(book, book.cover, '200x300')
const downloadUrl = (book: Book) => fileUrl(book, book.file)

const authorsOf = (book: Book) => (book.authors ?? []).join(', ')

/**
 * The page count worth showing: the one measured from the reading if there is
 * one, otherwise what the word count implies.
 */
const pagesOf = (book: Book) => (book.measured_pages > 0 ? book.measured_pages : book.page_count)

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
  // The reading progress comes from the documents, which the dashboard loads
  // too; asking again here keeps the library usable on its own.
  if (!documents.loaded) documents.load()
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
      <p v-if="!limit" class="mb-4 text-surface-600 dark:text-surface-400">
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
          <RouterLink
            :to="{ name: 'book', params: { id: book.id } }"
            class="relative aspect-[2/3] rounded-lg overflow-hidden bg-surface-100 dark:bg-surface-800 border border-surface-200 dark:border-surface-700 block hover:border-primary-400 transition-colors"
            :title="`Statistics for ${book.title}`"
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

            <div
              v-if="progressOf(book) !== undefined"
              class="absolute inset-x-0 bottom-0 bg-black/60 text-white text-xs px-2 py-1"
            >
              <div class="flex justify-between items-center gap-2">
                <span>{{ percentOf(book) === 100 ? 'Finished' : 'Reading' }}</span>
                <span class="tabular-nums">{{ percentOf(book) }}%</span>
              </div>
              <div class="mt-1 h-1 rounded-full bg-white/25 overflow-hidden">
                <div class="h-full bg-white" :style="{ width: `${percentOf(book)}%` }"></div>
              </div>
            </div>
          </RouterLink>

          <div class="flex flex-col gap-1">
            <RouterLink
              :to="{ name: 'book', params: { id: book.id } }"
              class="font-semibold leading-tight hover:underline"
              :title="book.title"
              >{{ book.title }}</RouterLink
            >
            <span v-if="authorsOf(book)" class="text-sm text-surface-600 dark:text-surface-400">
              {{ authorsOf(book) }}
            </span>
            <span class="text-xs text-surface-500 dark:text-surface-400">
              {{ formatCount(pagesOf(book)) }} pages · {{ formatCount(book.word_count) }} words
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

      <div v-if="hidden" class="mt-6 text-center">
        <RouterLink :to="{ name: 'library' }" class="hover:underline">
          See all {{ books.books.length }} books
        </RouterLink>
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
