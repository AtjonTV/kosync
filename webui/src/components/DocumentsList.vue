<!--
  File:        webui/src/components/DocumentsList.vue
  Project:     https://git.obth.eu/atjontv/kosync
  Copyright:   © 2025-2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
-->
<script setup lang="ts">
import { ref } from 'vue'
import { useConfirm } from 'primevue/useconfirm'
import { useToast } from 'primevue/usetoast'
import { useDocumentsStore } from '@/stores/documents'
import HistoryList from '@/components/HistoryList.vue'
import type { DataTableCellEditCompleteEvent } from 'primevue/datatable'
import type { DocumentWithHistory } from '@/models'
import { errorMessage } from '@/pb'

defineProps<{ customTitle?: string }>()

const documents = useDocumentsStore()
const confirm = useConfirm()
const toast = useToast()

const showHistoryDialog = ref(false)
const selectedDocument = ref<DocumentWithHistory | null>(null)

const viewMode = ref('Grid')
const viewOptions = ref(['Grid', 'List'])

const openHistory = (doc: DocumentWithHistory) => {
  selectedDocument.value = doc
  showHistoryDialog.value = true
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
    await documents.updateTitle(document.id, title)
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
        await documents.remove(doc.id)
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
</script>

<template>
  <div class="flex flex-col gap-4">
    <div class="flex justify-between items-center">
      <h1 class="text-3xl">{{ customTitle ?? 'Documents' }}</h1>
      <SelectButton v-model="viewMode" :options="viewOptions" :allow-empty="false" />
    </div>

    <div v-if="viewMode === 'List'">
      <DataTable
        data-key="id"
        :value="documents.documents"
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
        <Column field="document" header="Document" :sortable="true" style="width: 25%"></Column>
        <Column field="title" header="Title" :sortable="true" style="width: 25%">
          <template #editor="{ data, field }">
            <InputText v-model="data[field]" autofocus fluid />
          </template>
        </Column>
        <Column field="progress" header="Reading progress" :sortable="true">
          <template #body="{ data }"> {{ Number(data.progress * 100).toFixed(2) }}% </template>
        </Column>
        <Column field="last_device" header="Device" :sortable="true"></Column>
        <Column field="last_read_at" header="Last read" :sortable="true">
          <template #body="{ data }">{{ formatDateTime(data.last_read_at) }}</template>
        </Column>
        <Column header="Actions" style="width: 8rem">
          <template #body="{ data }">
            <Button
              icon="pi pi-history"
              variant="text"
              rounded
              title="View History"
              @click="openHistory(data)"
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

    <div v-else class="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 lg:grid-cols-4 gap-6">
      <Card
        v-for="doc in documents.documents"
        :key="doc.id"
        class="flex flex-col h-full bg-surface-0 dark:bg-surface-900 border border-surface-200 dark:border-surface-700 shadow-sm overflow-hidden"
      >
        <template #content>
          <div class="flex flex-col h-full p-2">
            <h3
              class="text-xl font-semibold mb-4 text-surface-900 dark:text-surface-0 line-clamp-2"
              :title="doc.title || doc.document"
            >
              {{ doc.title || doc.document }}
            </h3>

            <div class="mt-auto flex flex-col gap-3">
              <div>
                <div
                  class="flex justify-between text-sm mb-1 text-surface-600 dark:text-surface-400"
                >
                  <span>Progress</span>
                  <span>{{ Number((doc.progress || 0) * 100).toFixed(1) }}%</span>
                </div>
                <ProgressBar
                  :value="Number((doc.progress || 0) * 100)"
                  :show-value="false"
                  style="height: 6px"
                ></ProgressBar>
              </div>

              <div
                class="flex justify-between items-center text-sm text-surface-500 dark:text-surface-400"
              >
                <span class="truncate max-w-[50%]" :title="doc.last_device">
                  <i class="pi pi-tablet mr-1 text-xs"></i>{{ doc.last_device }}
                </span>
                <span>{{ formatDate(doc.last_read_at) }}</span>
              </div>
            </div>

            <div
              class="flex justify-end gap-2 mt-4 pt-4 border-t border-surface-200 dark:border-surface-700"
            >
              <Button
                icon="pi pi-history"
                variant="text"
                rounded
                title="View History"
                @click="openHistory(doc)"
              />
              <Button
                icon="pi pi-trash"
                severity="danger"
                variant="text"
                rounded
                title="Delete"
                @click="deleteDocument(doc)"
              />
            </div>
          </div>
        </template>
      </Card>

      <div
        v-if="documents.documents.length === 0"
        class="col-span-full text-center p-8 text-surface-500 dark:text-surface-400"
      >
        No documents found.
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
  </div>
</template>
