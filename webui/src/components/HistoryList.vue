<!--
  File:        webui/src/components/HistoryList.vue
  Project:     https://git.obth.eu/atjontv/kosync
  Copyright:   © 2025-2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
-->
<script setup lang="ts">
import { useConfirm } from 'primevue/useconfirm'
import { useToast } from 'primevue/usetoast'
import { useDocumentsStore } from '@/stores/documents'
import type { DocumentWithHistory, HistoryRecord } from '@/models'
import { errorMessage } from '@/pb'

const props = defineProps<{ document: DocumentWithHistory | null }>()

const documents = useDocumentsStore()
const confirm = useConfirm()
const toast = useToast()

const formatDateTime = (value: string) => (value ? new Date(value).toLocaleString() : '')

const deleteHistoryItem = (entry: HistoryRecord) => {
  confirm.require({
    message: `Are you sure you want to delete the state from ${formatDateTime(entry.last_read_at)}?`,
    header: 'Confirmation',
    icon: 'pi pi-exclamation-triangle',
    rejectProps: { label: 'Cancel', severity: 'secondary', outlined: true },
    acceptProps: { label: 'Delete', severity: 'danger' },
    accept: async () => {
      try {
        await documents.removeHistoryEntry(entry.id)
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

const restoreHistoryItem = (entry: HistoryRecord) => {
  if (!props.document) return

  const documentId = props.document.id

  confirm.require({
    message: `Restore this document to the state from ${formatDateTime(entry.last_read_at)}? The current position is kept in the history.`,
    header: 'Confirmation',
    icon: 'pi pi-refresh',
    rejectProps: { label: 'Cancel', severity: 'secondary', outlined: true },
    acceptProps: { label: 'Restore' },
    accept: async () => {
      try {
        await documents.restoreHistoryEntry(documentId, entry.id)
        toast.add({ severity: 'success', summary: 'Document restored', life: 3000 })
      } catch (e) {
        toast.add({
          severity: 'error',
          summary: 'Could not restore',
          detail: errorMessage(e),
          life: 5000,
        })
      }
    },
  })
}
</script>

<template>
  <div v-if="document && document.history.length > 0">
    <DataTable
      :value="document.history"
      data-key="id"
      paginator
      :rows="10"
      :rows-per-page-options="[10, 25, 50, 100]"
      sort-field="last_read_at"
      :sort-order="-1"
      scrollable
      table-style="min-width: 50rem"
    >
      <Column field="progress" header="Reading progress" :sortable="true">
        <template #body="{ data }">{{ Number(data.progress * 100).toFixed(2) }}%</template>
      </Column>
      <Column field="title" header="Previous Title" :sortable="true"></Column>
      <Column field="last_device" header="Device" :sortable="true"></Column>
      <Column field="last_read_at" header="When" :sortable="true">
        <template #body="{ data }">{{ formatDateTime(data.last_read_at) }}</template>
      </Column>
      <Column header="Actions" style="width: 8rem">
        <template #body="{ data }">
          <div class="flex gap-2">
            <Button
              icon="pi pi-refresh"
              severity="info"
              variant="text"
              rounded
              title="Restore"
              @click="restoreHistoryItem(data)"
            />
            <Button
              icon="pi pi-trash"
              severity="danger"
              variant="text"
              rounded
              title="Delete"
              @click="deleteHistoryItem(data)"
            />
          </div>
        </template>
      </Column>
    </DataTable>
  </div>
  <div v-else>
    <p>
      This document does not have a history yet.<br />
      Push your progress from a device, and check that automatic sync is enabled in KOReader.
    </p>
  </div>
</template>
