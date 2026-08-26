<!--
  File:        webui/src/views/CollectionsView.vue
  Project:     https://git.obth.eu/atjontv/kosync
  Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
-->
<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useConfirm } from 'primevue/useconfirm'
import { useToast } from 'primevue/usetoast'
import { useCollectionsStore } from '@/stores/collections'
import { useBooksStore } from '@/stores/books'
import type { Book, BookCollection } from '@/models'
import { errorMessage, fileUrl } from '@/pb'

const collections = useCollectionsStore()
const library = useBooksStore()
const confirm = useConfirm()
const toast = useToast()

const showEdit = ref(false)
/** The shelf being renamed, null while a new one is being made. */
const editTarget = ref<BookCollection | null>(null)
const name = ref('')
const description = ref('')
const editError = ref('')
const busy = ref(false)

const booksById = computed(() => new Map(library.books.map((book) => [book.id, book])))

/**
 * The first few covers of a shelf, as its face.
 *
 * A book that is on a shelf but no longer in the library cannot happen — the
 * server takes a deleted book off every shelf — but the list is filtered anyway,
 * because it is also read before the library has finished loading.
 */
const coversOf = (collection: BookCollection): Book[] =>
  (collection.books ?? [])
    .map((id) => booksById.value.get(id))
    .filter((book): book is Book => book !== undefined)
    .slice(0, 4)

const countOf = (collection: BookCollection) => {
  const count = (collection.books ?? []).length

  return count === 1 ? '1 book' : `${count} books`
}

const coverUrl = (book: Book) => fileUrl(book, book.cover, '100x150')

const openNew = () => {
  editTarget.value = null
  name.value = ''
  description.value = ''
  editError.value = ''
  showEdit.value = true
}

const openRename = (collection: BookCollection) => {
  editTarget.value = collection
  name.value = collection.name
  description.value = collection.description
  editError.value = ''
  showEdit.value = true
}

const save = async () => {
  const chosen = name.value.trim()
  if (!chosen) {
    editError.value = 'A collection needs a name.'

    return
  }

  editError.value = ''
  busy.value = true
  try {
    if (editTarget.value) {
      await collections.update(editTarget.value.id, {
        name: chosen,
        description: description.value.trim(),
      })
    } else {
      await collections.create(chosen, description.value.trim())
    }
    showEdit.value = false
  } catch (error) {
    editError.value = errorMessage(error, 'Could not save the collection.')
  } finally {
    busy.value = false
  }
}

// The books are not mentioned in the question because they are not at stake:
// deleting a shelf puts nothing in the bin but the shelf.
const remove = (collection: BookCollection) => {
  confirm.require({
    header: 'Delete this collection?',
    message: `"${collection.name}" will be gone. The books on it stay in your library.`,
    icon: 'pi pi-exclamation-triangle',
    acceptProps: { label: 'Delete', severity: 'danger' },
    rejectProps: { label: 'Cancel', severity: 'secondary', variant: 'outlined' },
    accept: async () => {
      try {
        await collections.remove(collection.id)
        toast.add({ severity: 'success', summary: 'Collection deleted', life: 3000 })
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

onMounted(async () => {
  await Promise.all([collections.load(), library.loaded ? Promise.resolve() : library.load()])
  await collections.subscribe()
})

onUnmounted(() => collections.unsubscribe())
</script>

<template>
  <div class="flex flex-col gap-6">
    <div class="flex flex-wrap justify-between items-center gap-4">
      <h1 class="text-3xl">Collections</h1>
      <Button label="New collection" icon="pi pi-plus" @click="openNew" />
    </div>

    <p class="text-surface-600 dark:text-surface-400">
      A collection is a shelf you put together yourself, in the order you choose. Everything else
      here is read out of a file or reported by a device; this is the one part of the library that
      is your own opinion. Each collection also shows up in the OPDS catalog, so you can browse it
      from KOReader.
    </p>

    <div v-if="collections.loading && !collections.loaded" class="p-8 text-center">
      <ProgressSpinner style="width: 2.5rem; height: 2.5rem" />
    </div>

    <div
      v-else-if="collections.collections.length"
      class="grid gap-6 md:grid-cols-2 xl:grid-cols-3"
    >
      <Card
        v-for="collection in collections.collections"
        :key="collection.id"
        class="bg-surface-0 dark:bg-surface-900 border border-surface-200 dark:border-surface-700"
      >
        <template #title>
          <div class="flex justify-between items-baseline gap-2">
            <RouterLink
              :to="{ name: 'collection', params: { id: collection.id } }"
              class="text-lg font-semibold hover:underline"
            >
              {{ collection.name }}
            </RouterLink>
            <span class="text-sm font-normal text-surface-500 dark:text-surface-400 tabular-nums">
              {{ countOf(collection) }}
            </span>
          </div>
        </template>
        <template #content>
          <div class="flex flex-col gap-4">
            <p
              v-if="collection.description"
              class="text-sm text-surface-600 dark:text-surface-400 line-clamp-2"
            >
              {{ collection.description }}
            </p>

            <RouterLink
              :to="{ name: 'collection', params: { id: collection.id } }"
              class="flex gap-2"
            >
              <div
                v-for="book in coversOf(collection)"
                :key="book.id"
                class="w-16 aspect-[2/3] rounded overflow-hidden bg-surface-100 dark:bg-surface-800 border border-surface-200 dark:border-surface-700 flex items-center justify-center"
                :title="book.title"
              >
                <img
                  v-if="book.cover"
                  :src="coverUrl(book)"
                  :alt="`Cover of ${book.title}`"
                  class="w-full h-full object-cover"
                  loading="lazy"
                />
                <i v-else class="pi pi-book text-surface-400 dark:text-surface-500"></i>
              </div>
              <div
                v-if="!coversOf(collection).length"
                class="text-sm text-surface-500 dark:text-surface-400"
              >
                Nothing on this shelf yet.
              </div>
            </RouterLink>

            <div class="flex gap-1">
              <Button
                icon="pi pi-pencil"
                variant="text"
                rounded
                title="Rename"
                @click="openRename(collection)"
              />
              <Button
                icon="pi pi-trash"
                severity="danger"
                variant="text"
                rounded
                title="Delete"
                @click="remove(collection)"
              />
            </div>
          </div>
        </template>
      </Card>
    </div>

    <div v-else class="p-8 text-center text-surface-500 dark:text-surface-400">
      No collections yet. Make one to keep a reading list, a series in the order you mean to read
      it, or anything else the metadata cannot tell you.
    </div>
  </div>

  <Dialog
    v-model:visible="showEdit"
    :header="editTarget ? 'Rename this collection' : 'New collection'"
    modal
    :style="{ width: '30rem' }"
  >
    <form class="flex flex-col gap-4" @submit.prevent="save">
      <div class="flex flex-col gap-2">
        <label for="collection-name">Name</label>
        <InputText id="collection-name" v-model="name" autofocus fluid />
      </div>
      <div class="flex flex-col gap-2">
        <label for="collection-description">Description</label>
        <Textarea id="collection-description" v-model="description" rows="3" fluid />
        <small class="text-surface-500 dark:text-surface-400">
          Optional, and only shown here — the catalog lists collections by name.
        </small>
      </div>
      <Message v-if="editError" severity="error" variant="simple">{{ editError }}</Message>
      <div class="flex justify-end gap-2">
        <Button type="button" label="Cancel" severity="secondary" @click="showEdit = false" />
        <Button type="submit" :label="editTarget ? 'Save' : 'Create'" :loading="busy" />
      </div>
    </form>
  </Dialog>
</template>
