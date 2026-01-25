<script setup lang="ts">

import {useSyncStore} from "@/stores/sync.ts";
import {ref} from "vue";

const {customTitle} = defineProps<{customTitle?: string}>()

const syncStore = useSyncStore();
syncStore.doSync();

const expandedRows = ref({});
</script>

<template>
  <div class="flex flex-col gap-4">
    <h1 class="text-3xl">{{ customTitle ?? 'Documents' }}</h1>
    <div>
      <DataTable v-model:expandedRows="expandedRows" dataKey="id" :value="syncStore.sync.documents" paginator :rows="15" :rowsPerPageOptions="[15, 25, 50, 100]">
        <Column expander style="width: 5rem" />
        <Column field="id" header="Document Hash" :sortable="true"></Column>
        <Column field="document.percentage" header="Reading progress" :sortable="true">
          <template #body="slotProps">
            {{ Number(slotProps.data.document.percentage*100).toFixed(2) }}%
          </template>
        </Column>
        <Column field="document.device" header="Device" :sortable="true"></Column>
        <Column field="document.timestamp" header="Last read" :sortable="true">
          <template #body="slotProps">
            {{ new Date(slotProps.data.document.timestamp*1000).toISOString() }}
          </template>
        </Column>

        <template #expansion="slotProps">
          <div class="p-4 flex flex-col gap-2">
            <h3 class="text-2xl">History</h3>
            <div v-if="slotProps.data.document_history !== null">
              <DataTable :value="slotProps.data.document_history">
                <Column field="percentage" header="Reading progress" :sortable="true">
                  <template #body="slotProps">
                    {{ Number(slotProps.data.percentage*100).toFixed(2) }}%
                  </template>
                </Column>
                <Column field="device" header="Device" :sortable="true"></Column>
                <Column field="timestamp" header="When" :sortable="true">
                  <template #body="slotProps">
                    {{ new Date(slotProps.data.timestamp*1000).toISOString() }}
                  </template>
                </Column>
              </DataTable>
            </div>
            <div v-else>
              <p>This document does not have a history.<br>You can try pushing your progress and you might want to check your automatic push setting.</p>
            </div>
          </div>
        </template>
      </DataTable>
    </div>
  </div>
</template>

<style scoped>

</style>
