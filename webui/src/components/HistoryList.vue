//
// File:        webui/src/components/HistoryList.vue
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2025-2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

<script setup lang="ts">
import {useSyncStore} from "@/stores/sync.ts";
import { useConfirm } from "primevue/useconfirm";
import { useToast } from "primevue/usetoast";
import { useI18nStore } from "@/stores/i18n.ts";

const props = defineProps<{document: any}>();

const syncStore = useSyncStore();
const confirm = useConfirm();
const toast = useToast();
const i18nStore = useI18nStore();

const deleteHistoryItem = (doc: any, historyItem: any) => {
    confirm.require({
        message: i18nStore.t('delete_history_confirm', new Date(historyItem.last_read_at/10).toISOString()),
        header: i18nStore.t('confirm_title'),
        icon: 'pi pi-exclamation-triangle',
        rejectProps: {
            label: i18nStore.t('cancel'),
            severity: 'secondary',
            outlined: true
        },
        acceptProps: {
            label: i18nStore.t('delete'),
            severity: 'danger'
        },
        accept: async () => {
            try {
                await syncStore.deleteHistoryItem(doc.id, historyItem.last_read_at);
            } catch (e: any) {
                toast.add({ severity: 'error', summary: i18nStore.t('error'), detail: e.message || e, life: 3000 });
            }
        }
    });
};

const restoreHistoryItem = (doc: any, historyItem: any) => {
    confirm.require({
        message: i18nStore.t('restore_history_confirm', new Date(historyItem.last_read_at/10).toISOString()),
        header: i18nStore.t('confirm_title'),
        icon: 'pi pi-refresh',
        acceptProps: {
            label: i18nStore.t('btn_restore'),
            severity: 'primary'
        },
        accept: async () => {
            try {
                await syncStore.restoreHistoryItem(doc.id, historyItem.last_read_at);
            } catch (e: any) {
                toast.add({ severity: 'error', summary: i18nStore.t('error'), detail: e.message || e, life: 3000 });
            }
        }
    });
};
</script>

<template>
  <div v-if="document && document.history !== null && document.history.length > 0">
    <DataTable
      :value="document.history"
      paginator :rows="10" :rowsPerPageOptions="[10, 25, 50, 100]"
      sortField="last_read_at" :sortOrder="-1"
      scrollable tableStyle="min-width: 50rem"
    >
      <Column field="progress" :header="$t('col_progress')" :sortable="true">
        <template #body="slotProps">
          {{ Number(slotProps.data.progress*100).toFixed(2) }}%
        </template>
      </Column>
      <Column field="title" :header="$t('col_previous_title')" :sortable="true"></Column>
      <Column field="last_read_on_device" :header="$t('col_device')" :sortable="true"></Column>
      <Column field="last_read_at" :header="$t('col_when')" :sortable="true">
        <template #body="slotProps">
          {{ new Date(slotProps.data.last_read_at/10).toISOString() }}
        </template>
      </Column>
      <Column :header="$t('col_actions')" style="width: 6rem">
        <template #body="historySlotProps">
          <div class="flex gap-2">
            <Button icon="pi pi-refresh" severity="info" variant="text" rounded @click="restoreHistoryItem(document, historySlotProps.data)" :title="$t('btn_restore')" />
            <Button icon="pi pi-trash" severity="danger" variant="text" rounded @click="deleteHistoryItem(document, historySlotProps.data)" :title="$t('delete')" />
          </div>
        </template>
      </Column>
    </DataTable>
  </div>
  <div v-else>
    <p v-html="$t('no_history_desc')"></p>
  </div>
</template>
