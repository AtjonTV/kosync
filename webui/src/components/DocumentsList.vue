<!--
  File:        webui/src/components/DocumentsList.vue
  Project:     https://git.obth.eu/atjontv/kosync
  Copyright:   © 2025-2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
-->
<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useConfirm } from 'primevue/useconfirm'
import { useToast } from 'primevue/usetoast'
import { useDocumentsStore } from '@/stores/documents'
import { useDevicesStore } from '@/stores/devices'
import { useBooksStore } from '@/stores/books'
import HistoryList from '@/components/HistoryList.vue'
import MergeDialog from '@/components/MergeDialog.vue'
import type { DataTableCellEditCompleteEvent } from 'primevue/datatable'
import type { Book, DocumentWithHistory } from '@/models'
import { errorMessage, fileUrl } from '@/pb'

const props = withDefaults(
  defineProps<{
    /** The documents to show. The page decides which; this only renders them. */
    documents: DocumentWithHistory[]
    /** Shared with the page so one control switches every list on it. */
    viewMode?: string
    emptyMessage?: string
  }>(),
  { viewMode: 'Grid', emptyMessage: 'No documents found.' },
)

// Named for what it is rather than "documents", which is the prop: this
// component is handed the documents to render and only reaches for the store to
// change one.
const store = useDocumentsStore()
// A push carries a device identifier and a name, and the name it carries is
// whatever KOReader was told to call itself. This turns it into the name the
// owner chose.
const devices = useDevicesStore()
// The books are here for the covers and the links: a document that has one is a
// book you can open, and saying so is the difference between this page being a
// list of hashes and a list of things you read.
const books = useBooksStore()
const confirm = useConfirm()
const toast = useToast()

const showHistoryDialog = ref(false)
const showMergeDialog = ref(false)
const selectedDocument = ref<DocumentWithHistory | null>(null)

const booksById = computed(() => new Map(books.books.map((book) => [book.id, book])))

const bookOf = (doc: DocumentWithHistory): Book | undefined =>
  doc.book ? booksById.value.get(doc.book) : undefined

const coverOf = (doc: DocumentWithHistory) => {
  const book = bookOf(doc)

  return book?.cover ? fileUrl(book, book.cover, '100x150') : ''
}

const deviceOf = (doc: DocumentWithHistory) =>
  devices.nameOf(doc.last_device_id) || doc.last_device || 'unknown device'

const openHistory = (doc: DocumentWithHistory) => {
  selectedDocument.value = doc
  showHistoryDialog.value = true
}

// The document a merge is started from is the one that is kept, which is why
// this is a per-row action rather than a selection: picking the survivor is the
// only decision worth making, and clicking it is how it is made.
const openMerge = (doc: DocumentWithHistory) => {
  selectedDocument.value = doc
  showMergeDialog.value = true
}

const formatDate = (value: string) => (value ? new Date(value).toLocaleDateString() : '')
const formatDateTime = (value: string) => (value ? new Date(value).toLocaleString() : '')

const onEditComplete = async (event: DataTableCellEditCompleteEvent) => {
  const document = event.data as DocumentWithHistory
  const title = String(event.newValue ?? '').trim()

  // An empty title would hide which document a row is, so the edit is dropped
  // and the previous value stays.
  if (title.length === 0) {
    document.title = String(event.value ?? '')
    return
  }

  try {
    await store.updateTitle(document.id, title)
  } catch (e) {
    toast.add({
      severity: 'error',
      summary: 'Could not rename',
      detail: errorMessage(e),
      life: 5000,
    })
  }
}

const deleteDocument = (doc: DocumentWithHistory) => {
  confirm.require({
    message: `Are you sure you want to delete "${doc.title || doc.document}"?`,
    header: 'Confirmation',
    icon: 'pi pi-exclamation-triangle',
    rejectProps: { label: 'Cancel', severity: 'secondary', outlined: true },
    acceptProps: { label: 'Delete', severity: 'danger' },
    accept: async () => {
      try {
        await store.remove(doc.id)
      } catch (e) {
        toast.add({
          severity: 'error',
          summary: 'Could not delete',
          detail: errorMessage(e),
          life: 5000,
        })
      }
    },
  })
}

onMounted(() => {
  if (!devices.loaded) devices.load()
  if (!books.loaded) books.load()
})
</script>

<template>
  <div class="flex flex-col gap-4">
    <div v-if="props.viewMode === 'List'">
      <DataTable
        data-key="id"
        :value="props.documents"
        paginator
        :rows="15"
        :rows-per-page-options="[15, 25, 50, 100]"
        edit-mode="cell"
        resizable-columns
        column-resize-mode="fit"
        table-style="min-width: 60rem"
        sort-field="last_read_at"
        :sort-order="-1"
        @cell-edit-complete="onEditComplete"
      >
        <Column field="title" header="Title" :sortable="true" style="width: 30%">
          <template #body="{ data }">
            <div class="flex items-center gap-2">
              <RouterLink
                v-if="data.book"
                :to="{ name: 'book', params: { id: data.book } }"
                class="hover:underline"
                >{{ data.title || data.document }}</RouterLink
              >
              <span v-else>{{ data.title || data.document }}</span>
              <Tag
                v-if="!data.book"
                value="Not in library"
                severity="warn"
                class="shrink-0"
                title="No uploaded EPUB matches this document"
              />
            </div>
          </template>
          <template #editor="{ data, field }">
            <InputText v-model="data[field]" autofocus fluid />
          </template>
        </Column>
        <Column field="progress" header="Reading progress" :sortable="true">
          <template #body="{ data }"> {{ Number(data.progress * 100).toFixed(2) }}% </template>
        </Column>
        <Column field="last_device" header="Device" :sortable="true">
          <template #body="{ data }">{{ deviceOf(data) }}</template>
        </Column>
        <Column field="last_read_at" header="Last read" :sortable="true">
          <template #body="{ data }">{{ formatDateTime(data.last_read_at) }}</template>
        </Column>
        <Column field="document" header="Hash" :sortable="true">
          <template #body="{ data }">
            <span
              class="font-mono text-xs text-surface-500 dark:text-surface-400"
              :title="`KOReader identifies this file as ${data.document}`"
              >{{ data.document.slice(0, 12) }}…</span
            >
          </template>
        </Column>
        <Column header="Actions" style="width: 10rem">
          <template #body="{ data }">
            <Button
              icon="pi pi-history"
              variant="text"
              rounded
              title="View History"
              @click="openHistory(data)"
            />
            <Button
              icon="pi pi-object-group"
              variant="text"
              rounded
              title="Merge with another document"
              @click="openMerge(data)"
            />
            <Button
              icon="pi pi-trash"
              severity="danger"
              variant="text"
              rounded
              title="Delete"
              @click="deleteDocument(data)"
            />
          </template>
        </Column>
      </DataTable>
    </div>

    <div v-else class="grid grid-cols-1 sm:grid-cols-2 xl:grid-cols-3 gap-4">
      <Card
        v-for="doc in props.documents"
        :key="doc.id"
        class="bg-surface-0 dark:bg-surface-900 border border-surface-200 dark:border-surface-700 shadow-sm overflow-hidden"
      >
        <template #content>
          <div class="flex gap-4">
            <RouterLink
              v-if="doc.book"
              :to="{ name: 'book', params: { id: doc.book } }"
              class="w-16 shrink-0 aspect-[2/3] rounded overflow-hidden bg-surface-100 dark:bg-surface-800 block"
            >
              <img
                v-if="coverOf(doc)"
                :src="coverOf(doc)"
                :alt="`Cover of ${doc.title}`"
                class="w-full h-full object-cover"
                loading="lazy"
              />
              <span
                v-else
                class="w-full h-full flex items-center justify-center text-surface-400 dark:text-surface-500"
              >
                <i class="pi pi-book"></i>
              </span>
            </RouterLink>
            <!-- No cover to show, and the reason is the point of this page. -->
            <span
              v-else
              class="w-16 shrink-0 aspect-[2/3] rounded border border-dashed border-surface-300 dark:border-surface-600 flex items-center justify-center text-surface-400 dark:text-surface-500"
              title="No uploaded EPUB matches this document"
            >
              <i class="pi pi-question"></i>
            </span>

            <div class="flex flex-col gap-2 min-w-0 grow">
              <div class="flex items-start justify-between gap-2">
                <RouterLink
                  v-if="doc.book"
                  :to="{ name: 'book', params: { id: doc.book } }"
                  class="font-semibold leading-tight line-clamp-2 hover:underline"
                  :title="doc.title || doc.document"
                  >{{ doc.title || doc.document }}</RouterLink
                >
                <span
                  v-else
                  class="font-semibold leading-tight line-clamp-2"
                  :title="doc.title || doc.document"
                  >{{ doc.title || doc.document }}</span
                >
              </div>

              <Tag v-if="!doc.book" value="Not in library" severity="warn" class="self-start" />

              <div>
                <div
                  class="flex justify-between text-sm mb-1 text-surface-600 dark:text-surface-400"
                >
                  <span>{{ deviceOf(doc) }}</span>
                  <span class="tabular-nums"
                    >{{ Number((doc.progress || 0) * 100).toFixed(1) }}%</span
                  >
                </div>
                <ProgressBar
                  :value="Number((doc.progress || 0) * 100)"
                  :show-value="false"
                  style="height: 6px"
                ></ProgressBar>
              </div>

              <div
                class="flex justify-between items-center text-xs text-surface-500 dark:text-surface-400"
              >
                <span>{{ formatDate(doc.last_read_at) }}</span>
                <span class="flex gap-1">
                  <Button
                    icon="pi pi-history"
                    variant="text"
                    rounded
                    size="small"
                    title="View History"
                    @click="openHistory(doc)"
                  />
                  <Button
                    icon="pi pi-object-group"
                    variant="text"
                    rounded
                    size="small"
                    title="Merge with another document"
                    @click="openMerge(doc)"
                  />
                  <Button
                    icon="pi pi-trash"
                    severity="danger"
                    variant="text"
                    rounded
                    size="small"
                    title="Delete"
                    @click="deleteDocument(doc)"
                  />
                </span>
              </div>
            </div>
          </div>
        </template>
      </Card>

      <div
        v-if="props.documents.length === 0"
        class="col-span-full text-center p-8 text-surface-500 dark:text-surface-400"
      >
        {{ props.emptyMessage }}
      </div>
    </div>

    <Dialog
      v-model:visible="showHistoryDialog"
      header="History"
      modal
      :breakpoints="{ '960px': '75vw', '640px': '90vw' }"
      :style="{ width: '70rem' }"
    >
      <HistoryList :document="selectedDocument" />
    </Dialog>

    <MergeDialog v-model:visible="showMergeDialog" :document="selectedDocument" />
  </div>
</template>
