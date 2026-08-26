<!--
  File:        webui/src/components/BookLibrary.vue
  Project:     https://git.obth.eu/atjontv/kosync
  Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
-->
<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useConfirm } from 'primevue/useconfirm'
import { useToast } from 'primevue/usetoast'
import { useBooksStore } from '@/stores/books'
import { useDocumentsStore } from '@/stores/documents'
import { authorName, groupBooks, type Grouping } from '@/lib/grouping'
import { bookOrder, searchBooks, type Sorting } from '@/lib/browsing'
import type { Book } from '@/models'
import type { FileUploadUploaderEvent } from 'primevue/fileupload'
import { errorMessage, fileUrl } from '@/pb'

const props = withDefaults(
  defineProps<{
    /**
     * How many books to show. Unset means all of them, in whatever order and
     * grouping the page is being read with, which is what the library page
     * wants. Set, the dashboard gets the ones most recently read and a link to
     * the rest — a shelf, not a catalogue, and so not something to look through.
     */
    limit?: number
    /**
     * The card's own heading. Empty on a page that already has one, so the word
     * "Library" is not printed twice above the same grid.
     */
    heading?: string
    /**
     * The books to show, in the order they are to be shown in. Unset means the
     * whole library, which is what the library page and the dashboard want.
     *
     * A collection passes its own books and its own order, and gets a grid
     * without the upload button and without anything that would search, sort or
     * group them: none of it belongs on a page whose order is the point of it.
     */
    books?: Book[]
    /** What to say when there is nothing to show. */
    empty?: string
  }>(),
  { heading: 'Library', empty: 'No books yet. Add an EPUB to keep a copy here.' },
)

const library = useBooksStore()
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

/** The two things about the reading that the library can be ordered by. */
const reading = computed(() => ({
  lastRead: lastReadByBook.value,
  progress: progressByBook.value,
}))

/**
 * What is being looked for, if anything.
 *
 * Not remembered between visits, unlike the grouping and the sort: those are
 * ways of reading a library, and this is a question about one book. Coming back
 * to the page tomorrow and finding four of your books is a fault, not a favour.
 */
const query = ref('')

/**
 * The order the library comes out in, remembered between visits for the same
 * reason the grouping is: it is a preference about reading, not a place.
 */
const sortKey = 'library-sort'
const sortings: Sorting[] = ['title', 'added', 'last-read', 'progress']

const savedSorting = localStorage.getItem(sortKey) as Sorting | null
const sorting = ref<Sorting>(
  savedSorting && sortings.includes(savedSorting) ? savedSorting : 'title',
)

watch(sorting, (chosen) => localStorage.setItem(sortKey, chosen))

const sortOptions = [
  { label: 'Title', value: 'title' },
  { label: 'Recently added', value: 'added' },
  { label: 'Recently read', value: 'last-read' },
  { label: 'Progress', value: 'progress' },
]

/** The whole library, or as much of it as the search left. */
const matches = computed(() => searchBooks(library.books, query.value))

const byRecency = computed(() =>
  [...library.books].sort((a, b) => {
    const left = lastReadByBook.value.get(a.id) ?? ''
    const right = lastReadByBook.value.get(b.id) ?? ''
    if (left !== right) return right.localeCompare(left)

    return a.title.localeCompare(b.title)
  }),
)

const sorted = computed(() => {
  // A given list is already in the order it is meant to be read in — a shelf is
  // a sequence somebody decided on — so it is passed through untouched.
  if (props.books) return props.books
  if (props.limit) return byRecency.value.slice(0, props.limit)

  return [...matches.value].sort(bookOrder(sorting.value, reading.value))
})

/** How many books the limit is hiding. */
const hidden = computed(() => (props.limit ? Math.max(library.books.length - props.limit, 0) : 0))

/**
 * How the library is broken up, remembered between visits.
 *
 * Somebody who browses by series wants to browse by series tomorrow as well, and
 * the choice is worth less than the trouble of making it again. Stored rather than
 * put in the route because it is a preference, not a place: the library page has
 * one address whichever way its owner happens to be reading it.
 */
const groupingKey = 'library-grouping'
const groupings: Grouping[] = ['none', 'authors', 'series', 'languages']

const savedGrouping = localStorage.getItem(groupingKey) as Grouping | null
const grouping = ref<Grouping>(
  savedGrouping && groupings.includes(savedGrouping) ? savedGrouping : 'none',
)

watch(grouping, (chosen) => localStorage.setItem(groupingKey, chosen))

const groupingOptions = [
  { label: 'Nothing', value: 'none' },
  { label: 'Author', value: 'authors' },
  { label: 'Series', value: 'series' },
  { label: 'Language', value: 'languages' },
]

/**
 * How the books actually come out, which is not always what was chosen.
 *
 * The dashboard is a shelf and not a catalogue, so it is never grouped: breaking
 * six recently read books into headed sections is noise, and the page it links to
 * is where the grouping belongs.
 *
 * A given list is never grouped either, for the stronger reason that grouping it
 * would sort it: the order is the whole content of a hand-made collection.
 */
const groupedBy = computed<Grouping>(() => (props.limit || props.books ? 'none' : grouping.value))

/**
 * The order to impose inside each shelf, when one has been asked for.
 *
 * Nothing for the default: by title is what the shelves already do, and a series
 * shelf's own reading order is worth more than saying that again.
 */
const shelfOrder = computed(() =>
  sorting.value === 'title' ? undefined : bookOrder(sorting.value, reading.value),
)

const shelves = computed(() => groupBooks(sorted.value, groupedBy.value, shelfOrder.value))

/** Whether this grid is the library itself, and so something to look through. */
const browsable = computed(() => !props.limit && !props.books)

/** Whether a search is narrowing what is on the screen. */
const searching = computed(() => browsable.value && query.value.trim().length > 0)

/**
 * What to say when there is nothing to show.
 *
 * A library a search has emptied is not an empty library, and telling somebody
 * to upload an EPUB when they have just mistyped an author's name answers a
 * question they did not ask.
 */
const nothing = computed(() =>
  searching.value ? `No books match "${query.value.trim()}".` : props.empty,
)

const progressOf = (book: Book) => progressByBook.value.get(book.id)
const percentOf = (book: Book) => Math.round((progressOf(book) ?? 0) * 100)

const coverUrl = (book: Book) => fileUrl(book, book.cover, '200x300')
const downloadUrl = (book: Book) => fileUrl(book, book.file)

// Tidied the way the shelf headings and the catalog tidy them, so that a book
// filed under "Lee Child" does not say "Child, Lee" underneath its own cover.
const authorsOf = (book: Book) => (book.authors ?? []).map(authorName).join(', ')

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
        await library.upload(file)
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
    await library.rename(renameTarget.value.id, newTitle.value.trim())
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
        await library.remove(book.id)
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
  library.load()
  // The reading progress comes from the documents, which the dashboard loads
  // too; asking again here keeps the library usable on its own.
  if (!documents.loaded) documents.load()
})
</script>

<template>
  <Card class="bg-surface-0 dark:bg-surface-900 border border-surface-200 dark:border-surface-700">
    <template #title>
      <div class="flex flex-wrap justify-between items-center gap-4">
        <span v-if="props.heading" class="text-xl font-semibold">{{ props.heading }}</span>
        <span v-else></span>
        <div class="flex items-center gap-3">
          <slot name="header" />
          <FileUpload
            v-if="!props.books"
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
      </div>
    </template>
    <template #content>
      <p v-if="!limit && !props.books" class="mb-4 text-surface-600 dark:text-surface-400">
        Books you upload are kept here as a backup, and let KOsync recognise which book a device is
        reporting progress for. Upload the very file you read on the device: the match is made on
        the file's contents, so another copy of the same title will not do.
      </p>

      <!--
        Only where there is a library to look through. On the dashboard shelf and
        on a collection the order is the page's own, and offering to change it
        would be offering something the grid then ignores.
      -->
      <div v-if="browsable && library.books.length" class="flex flex-wrap items-end gap-3 mb-6">
        <div class="flex flex-col gap-2 grow min-w-56 max-w-96">
          <label for="library-search" class="text-sm text-surface-600 dark:text-surface-400">
            Search
          </label>
          <IconField>
            <InputIcon class="pi pi-search" />
            <!--
              A search input rather than a text one, so that clearing it is the
              browser's own control: an icon of ours inside the field would come
              out of InputIcon marked aria-hidden, and a focusable control nobody
              using a screen reader can see is worse than no control at all.
            -->
            <InputText
              id="library-search"
              v-model="query"
              type="search"
              placeholder="Title, author or series"
              size="small"
              fluid
            />
          </IconField>
        </div>

        <div class="flex flex-col gap-2">
          <label for="library-sort" class="text-sm text-surface-600 dark:text-surface-400">
            Sort by
          </label>
          <Select
            id="library-sort"
            v-model="sorting"
            :options="sortOptions"
            option-label="label"
            option-value="value"
            size="small"
            aria-label="Sort the library by"
          />
        </div>

        <div class="flex flex-col gap-2">
          <label for="library-grouping" class="text-sm text-surface-600 dark:text-surface-400">
            Group by
          </label>
          <Select
            id="library-grouping"
            v-model="grouping"
            :options="groupingOptions"
            option-label="label"
            option-value="value"
            size="small"
            aria-label="Group the library by"
          />
        </div>

        <!--
          Only while a search is on. Without one the count is the library's own,
          which the page above already prints.
        -->
        <span
          v-if="searching"
          class="text-sm text-surface-500 dark:text-surface-400 tabular-nums pb-2"
          aria-live="polite"
        >
          {{ matches.length }} of {{ library.books.length }}
        </span>
      </div>

      <Message v-if="uploading" severity="info" class="mb-4">Uploading…</Message>

      <Message v-if="failures.length" severity="error" class="mb-4" closable @close="failures = []">
        <div class="flex flex-col gap-1">
          <span v-for="failure in failures" :key="failure">{{ failure }}</span>
        </div>
      </Message>

      <div v-if="library.loading && !library.loaded" class="p-8 text-center">
        <ProgressSpinner style="width: 2.5rem; height: 2.5rem" />
      </div>

      <div v-else-if="shelves.length" class="flex flex-col gap-8">
        <section v-for="shelf in shelves" :key="shelf.key" class="flex flex-col gap-4">
          <h2
            v-if="shelf.title"
            class="flex items-baseline gap-2 pb-2 border-b border-surface-200 dark:border-surface-700"
          >
            <span class="text-lg font-semibold">{{ shelf.title }}</span>
            <span class="text-sm text-surface-500 dark:text-surface-400 tabular-nums">
              {{ shelf.books.length }}
            </span>
          </h2>

          <div class="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 xl:grid-cols-6 gap-6">
            <div v-for="book in shelf.books" :key="book.id" class="flex flex-col gap-2 h-full">
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
                  class="font-semibold leading-tight line-clamp-2 min-h-[2.5em] hover:underline"
                  :title="book.title"
                  >{{ book.title }}</RouterLink
                >
                <span
                  class="text-sm text-surface-600 dark:text-surface-400 leading-tight line-clamp-1 min-h-[1.25em]"
                  :title="authorsOf(book)"
                >
                  {{ authorsOf(book) }}
                </span>
                <span class="text-xs text-surface-500 dark:text-surface-400">
                  {{ formatCount(pagesOf(book)) }} pages · {{ formatCount(book.word_count) }} words
                </span>
              </div>

              <div class="flex gap-1 mt-auto">
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
                <!--
                  Left off a given list: a page about one shelf offers taking a
                  book off it, and two trash cans side by side, one of which
                  deletes the file, is a trap rather than a convenience.
                -->
                <Button
                  v-if="!props.books"
                  icon="pi pi-trash"
                  severity="danger"
                  variant="text"
                  rounded
                  title="Delete"
                  @click="remove(book)"
                />
                <slot name="actions" :book="book" />
              </div>
            </div>
          </div>
        </section>
      </div>

      <div v-else class="p-8 text-center text-surface-500 dark:text-surface-400">
        {{ nothing }}
      </div>

      <div v-if="hidden" class="mt-6 text-center">
        <RouterLink :to="{ name: 'library' }" class="hover:underline">
          See all {{ library.books.length }} books
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
