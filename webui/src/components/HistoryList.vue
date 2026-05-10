//
// File:        webui/src/components/HistoryList.vue
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2025-2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

<script setup lang="ts">
import {useSyncStore} from "@/stores/sync.ts";
import { useConfirm } from "primevue/useconfirm";

const props = defineProps<{document: any}>();

const syncStore = useSyncStore();
const confirm = useConfirm();

const deleteHistoryItem = (doc: any, historyItem: any) => {
    confirm.require({
        message: `Are you sure you want to delete this history item from "${historyItem.last_read_at}"?`,
        header: 'Confirmation',
        icon: 'pi pi-exclamation-triangle',
        rejectProps: {
            label: 'Cancel',
            severity: 'secondary',
            outlined: true
        },
        acceptProps: {
            label: 'Delete',
            severity: 'danger'
        },
        accept: async () => {
            await syncStore.deleteHistoryItem(doc.id, historyItem.last_read_at);
        }
    });
};
</script>

<template>
  <div v-if="document && document.history !== null">
    <DataTable
      :value="document.history"
      paginator :rows="10" :rowsPerPageOptions="[10, 25, 50, 100]"
      sortField="last_read_at" :sortOrder="-1"
      scrollable tableStyle="min-width: 50rem"
    >
      <Column field="progress" header="Reading progress" :sortable="true">
        <template #body="slotProps">
          {{ Number(slotProps.data.progress*100).toFixed(2) }}%
        </template>
      </Column>
      <Column field="title" header="Previous Title" :sortable="true"></Column>
      <Column field="last_read_on_device" header="Device" :sortable="true"></Column>
      <Column field="last_read_at" header="When" :sortable="true">
        <template #body="slotProps">
          {{ new Date(slotProps.data.last_read_at/10).toISOString() }}
        </template>
      </Column>
      <Column header="Actions" style="width: 3rem">
        <template #body="historySlotProps">
          <Button icon="pi pi-trash" severity="danger" variant="text" rounded @click="deleteHistoryItem(document, historySlotProps.data)" />
        </template>
      </Column>
    </DataTable>
  </div>
  <div v-else>
    <p>This document does not have a history.<br>You can try pushing your progress and you might want to check your automatic push setting.</p>
  </div>
</template>
