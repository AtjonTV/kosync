<script setup lang="ts">

import {useSyncStore} from "@/stores/sync.ts";
import {ref} from "vue";
import {fetchApi} from "@/api.ts";
import {useUserStore} from "@/stores/user.ts";
import { useConfirm } from "primevue/useconfirm";
import HistoryList from "@/components/HistoryList.vue";

const {customTitle} = defineProps<{customTitle?: string}>()

const userStore = useUserStore();
const confirm = useConfirm();

const syncStore = useSyncStore();
if (await userStore.isLoggedIn()) {
  syncStore.doSync();
  syncStore.doPubSubSync();
}

const showHistoryDialog = ref(false);
const selectedDocument = ref<any>(null);

const openHistory = (doc: any) => {
    selectedDocument.value = doc;
    showHistoryDialog.value = true;
};

const onEditComplete = async (event: any) => {
    const result = await fetchApi("/api/documents.update", {
        method: "PUT",
        headers: {"Content-Type": "application/json"},
        body: JSON.stringify(event.newData)
    });
    if (result.error !== null) alert("Failed to update document: " + result.error)

    let {data, newValue, field} = event;
    if (newValue.trim().length > 0) {
        data[field] = newValue;
    } else {
        event.preventDefault();
    }
}

const deleteDocument = (data: any) => {
    confirm.require({
        message: `Are you sure you want to delete "${data.title || data.id}"?`,
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
            const result = await fetchApi(`/api/documents.delete?id=${data.id}`, {
                method: "DELETE"
            });
            if (result.error !== null) {
                alert("Failed to delete document: " + result.error)
            } else {
                await syncStore.doSync(true);
            }
        }
    });
};
</script>

<template>
  <div class="flex flex-col gap-4">
    <h1 class="text-3xl">{{ customTitle ?? 'Documents' }}</h1>
    <div>
      <DataTable
          dataKey="id"
          :value="syncStore.sync.documents"
          paginator :rows="15" :rowsPerPageOptions="[15, 25, 50, 100]"
          editMode="cell" @cellEditComplete="onEditComplete"
          resizableColumns columnResizeMode="fit" tableStyle="min-width: 100rem"
          sortField="last_read_at" :sortOrder="-1"
      >
        <Column field="id" header="ID" :sortable="true" style="width: 25%"></Column>
        <Column field="title" header="Title" :sortable="true" style="width: 25%">
            <template #editor="{data, field}">
                <InputText v-model="data[field]" :defaultValue="data[field]" autofocus fluid />
            </template>
        </Column>
        <Column field="progress" header="Reading progress" :sortable="true">
          <template #body="slotProps">
            {{ Number(slotProps.data.progress*100).toFixed(2) }}%
          </template>
        </Column>
        <Column field="last_read_on_device" header="Device" :sortable="true"></Column>
        <Column field="last_read_at" header="Last read" :sortable="true">
          <template #body="slotProps">
            {{ new Date(slotProps.data.last_read_at/10).toISOString() }}
          </template>
        </Column>
        <Column header="Actions" style="width: 5rem">
            <template #body="slotProps">
                <Button icon="pi pi-history" variant="text" rounded @click="openHistory(slotProps.data)" />
                <Button icon="pi pi-trash" severity="danger" variant="text" rounded @click="deleteDocument(slotProps.data)" />
            </template>
        </Column>
      </DataTable>
    </div>
    <Dialog v-model:visible="showHistoryDialog" header="History" modal :breakpoints="{ '960px': '75vw', '640px': '90vw' }" :style="{ width: '80rem' }">
      <HistoryList :document="selectedDocument" />
    </Dialog>
  </div>
</template>
