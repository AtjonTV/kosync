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
  await syncStore.updateDocument(event.newData);

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
            syncStore.deleteDocument(data.id)
        }
    });
};

const viewMode = ref('Grid');
const viewOptions = ref(['Grid', 'List']);
</script>

<template>
  <div class="flex flex-col gap-4">
    <div class="flex justify-between items-center">
      <h1 class="text-3xl">{{ customTitle ?? 'Documents' }}</h1>
      <SelectButton v-model="viewMode" :options="viewOptions" :allowEmpty="false" />
    </div>

    <div v-if="viewMode === 'List'">
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
                <Button icon="pi pi-history" variant="text" rounded @click="openHistory(slotProps.data)" title="View History" />
                <Button icon="pi pi-trash" severity="danger" variant="text" rounded @click="deleteDocument(slotProps.data)" title="Delete" />
            </template>
        </Column>
      </DataTable>
    </div>

    <div v-else class="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 lg:grid-cols-4 gap-6">
      <Card v-for="doc in syncStore.sync.documents" :key="doc.id" class="flex flex-col h-full bg-surface-0 dark:bg-surface-900 border border-surface-200 dark:border-surface-700 shadow-sm overflow-hidden">
        <template #content>
          <div class="flex flex-col h-full p-2">
            <h3 class="text-xl font-semibold mb-4 text-surface-900 dark:text-surface-0 line-clamp-2" :title="doc.title || doc.id">{{ doc.title || doc.id }}</h3>

            <div class="mt-auto flex flex-col gap-3">
              <div>
                <div class="flex justify-between text-sm mb-1 text-surface-600 dark:text-surface-400">
                  <span>Progress</span>
                  <span>{{ Number((doc.progress || 0) * 100).toFixed(1) }}%</span>
                </div>
                <ProgressBar :value="Number((doc.progress || 0) * 100)" :showValue="false" style="height: 6px"></ProgressBar>
              </div>

              <div class="flex justify-between items-center text-sm text-surface-500 dark:text-surface-400">
                <span class="truncate max-w-[50%]" :title="doc.last_read_on_device"><i class="pi pi-tablet mr-1 text-xs"></i>{{ doc.last_read_on_device }}</span>
                <span>{{ new Date((doc.last_read_at || 0) / 10).toLocaleDateString() }}</span>
              </div>
            </div>

            <div class="flex justify-end gap-2 mt-4 pt-4 border-t border-surface-200 dark:border-surface-700">
              <Button icon="pi pi-history" variant="text" rounded @click="openHistory(doc)" title="View History" />
              <Button icon="pi pi-trash" severity="danger" variant="text" rounded @click="deleteDocument(doc)" title="Delete" />
            </div>
          </div>
        </template>
      </Card>

      <div v-if="syncStore.sync.documents.length === 0" class="col-span-full text-center p-8 text-surface-500 dark:text-surface-400">
        No documents found.
      </div>
    </div>

    <Dialog v-model:visible="showHistoryDialog" header="History" modal :breakpoints="{ '960px': '75vw', '640px': '90vw' }" :style="{ width: '80rem' }">
      <HistoryList :document="selectedDocument" />
    </Dialog>
  </div>
</template>
