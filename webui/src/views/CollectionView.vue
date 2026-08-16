<!--
  File:        webui/src/views/CollectionView.vue
  Project:     https://git.obth.eu/atjontv/kosync
  Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
-->
<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import { useToast } from 'primevue/usetoast'
import BookLibrary from '@/components/BookLibrary.vue'
import { useCollectionsStore } from '@/stores/collections'
import { useBooksStore } from '@/stores/books'
import { authorName } from '@/lib/grouping'
import type { Book } from '@/models'
import { errorMessage } from '@/pb'

const route = useRoute()
const collections = useCollectionsStore()
const library = useBooksStore()
const toast = useToast()

const id = computed(() => String(route.params.id ?? ''))
const collection = computed(() => collections.byId.get(id.value))

const booksById = computed(() => new Map(library.books.map((book) => [book.id, book])))

/**
 * What is on the shelf, in the order it was put there.
 *
 * The ids are the shelf; the books are looked up out of the library, so a book
 * renamed elsewhere is renamed here too.
 */
const shelved = computed<Book[]>(() =>
  (collection.value?.books ?? [])
    .map((bookId) => booksById.value.get(bookId))
    .filter((book): book is Book => book !== undefined),
)

/** Everything not on it yet, by title, which is how a book is looked for. */
const available = computed<Book[]>(() => {
  const already = new Set(collection.value?.books ?? [])

  return library.books
    .filter((book) => !already.has(book.id))
    .sort((a, b) => a.title.localeCompare(b.title))
})

const showAdd = ref(false)
const chosen = ref<Book[]>([])
const search = ref('')
const addError = ref('')
const busy = ref(false)

const matching = computed(() => {
  const needle = search.value.trim().toLowerCase()
  if (!needle) return available.value

  return available.value.filter(
    (book) =>
      book.title.toLowerCase().includes(needle) ||
      (book.authors ?? []).some((author) => author.toLowerCase().includes(needle)),
  )
})

const authorsOf = (book: Book) => (book.authors ?? []).map(authorName).join(', ')

const openAdd = () => {
  chosen.value = []
  search.value = ''
  addError.value = ''
  showAdd.value = true
}

/**
 * Adds the chosen books, one request each and in the order they were listed.
 *
 * One at a time because each is an append: sending them as a list would mean
 * sending the shelf as it was read, which is how one open tab quietly undoes
 * another's work.
 */
const add = async () => {
  if (!collection.value || !chosen.value.length) return

  addError.value = ''
  busy.value = true
  try {
    for (const book of chosen.value) {
      await collections.addBook(collection.value.id, book.id)
    }
    showAdd.value = false
    toast.add({
      severity: 'success',
      summary: chosen.value.length === 1 ? 'Book added' : `${chosen.value.length} books added`,
      life: 3000,
    })
  } catch (error) {
    addError.value = errorMessage(error, 'Could not add the books.')
  } finally {
    busy.value = false
  }
}

const takeOff = async (book: Book) => {
  if (!collection.value) return

  try {
    await collections.removeBook(collection.value.id, book.id)
  } catch (error) {
    toast.add({ severity: 'error', summary: 'Failed', detail: errorMessage(error), life: 5000 })
  }
}

/** Moves a book one place along the shelf. */
const move = async (book: Book, by: number) => {
  if (!collection.value) return

  const order = [...(collection.value.books ?? [])]
  const from = order.indexOf(book.id)
  const to = from + by
  if (from < 0 || to < 0 || to >= order.length) return

  const [moved] = order.splice(from, 1)
  if (moved === undefined) return
  order.splice(to, 0, moved)

  try {
    await collections.reorder(collection.value.id, order)
  } catch (error) {
    toast.add({ severity: 'error', summary: 'Failed', detail: errorMessage(error), life: 5000 })
  }
}

const positionOf = (book: Book) => shelved.value.findIndex((one) => one.id === book.id)

onMounted(async () => {
  await Promise.all([
    collections.loaded ? Promise.resolve() : collections.load(),
    library.loaded ? Promise.resolve() : library.load(),
  ])
  await collections.subscribe()
})

onUnmounted(() => collections.unsubscribe())
</script>

<template>
  <div class="flex flex-col gap-6">
    <RouterLink
      :to="{ name: 'collections' }"
      class="text-surface-600 dark:text-surface-400 hover:underline"
    >
      <i class="pi pi-arrow-left mr-2"></i>All collections
    </RouterLink>

    <div v-if="collections.loading && !collections.loaded" class="p-8 text-center">
      <ProgressSpinner style="width: 2.5rem; height: 2.5rem" />
    </div>

    <Message v-else-if="!collection" severity="warn">
      That collection is not there. It may have been deleted.
    </Message>

    <template v-else>
      <div class="flex flex-wrap justify-between items-center gap-4">
        <h1 class="text-3xl">{{ collection.name }}</h1>
        <Button label="Add books" icon="pi pi-plus" @click="openAdd" />
      </div>

      <p v-if="collection.description" class="text-surface-600 dark:text-surface-400">
        {{ collection.description }}
      </p>

      <BookLibrary
        heading=""
        :books="shelved"
        empty="Nothing on this shelf yet. Add books from your library to build it."
      >
        <template #actions="{ book }">
          <Button
            icon="pi pi-arrow-left"
            variant="text"
            rounded
            title="Move earlier"
            :disabled="positionOf(book) <= 0"
            @click="move(book, -1)"
          />
          <Button
            icon="pi pi-arrow-right"
            variant="text"
            rounded
            title="Move later"
            :disabled="positionOf(book) >= shelved.length - 1"
            @click="move(book, 1)"
          />
          <Button
            icon="pi pi-times"
            severity="danger"
            variant="text"
            rounded
            title="Take off this collection"
            @click="takeOff(book)"
          />
        </template>
      </BookLibrary>
    </template>
  </div>

  <Dialog v-model:visible="showAdd" header="Add books" modal :style="{ width: '34rem' }">
    <div class="flex flex-col gap-4">
      <p class="text-surface-600 dark:text-surface-400">
        Books are added at the end, in the order you pick them. You can move them afterwards.
      </p>

      <InputText v-model="search" placeholder="Search your library" fluid />

      <Listbox
        v-model="chosen"
        :options="matching"
        option-label="title"
        multiple
        checkmark
        list-style="max-height: 20rem"
        empty-message="Every book in your library is already on this collection."
      >
        <template #option="{ option }">
          <div class="flex flex-col">
            <span>{{ option.title }}</span>
            <span v-if="authorsOf(option)" class="text-sm text-surface-500 dark:text-surface-400">
              {{ authorsOf(option) }}
            </span>
          </div>
        </template>
      </Listbox>

      <Message v-if="addError" severity="error" variant="simple">{{ addError }}</Message>

      <div class="flex justify-end gap-2">
        <Button type="button" label="Cancel" severity="secondary" @click="showAdd = false" />
        <Button
          type="button"
          :label="chosen.length ? `Add ${chosen.length}` : 'Add'"
          :disabled="!chosen.length"
          :loading="busy"
          @click="add"
        />
      </div>
    </div>
  </Dialog>
</template>
