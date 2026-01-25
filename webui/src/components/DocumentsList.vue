<script setup lang="ts">

import {useSyncStore} from "@/stores/sync.ts";
import {ref} from "vue";
import {fetchApi} from "@/api.ts";

const {customTitle} = defineProps<{customTitle?: string}>()

const syncStore = useSyncStore();
syncStore.doSync();

const expandedRows = ref({});

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

    await syncStore.doSync(true);
}
</script>

<template>
  <div class="flex flex-col gap-4">
    <h1 class="text-3xl">{{ customTitle ?? 'Documents' }}</h1>
    <div>
      <DataTable
          v-model:expandedRows="expandedRows"
          dataKey="id"
          :value="syncStore.sync.documents"
          paginator :rows="15" :rowsPerPageOptions="[15, 25, 50, 100]"
          editMode="cell" @cellEditComplete="onEditComplete"
          resizableColumns columnResizeMode="fit" tableStyle="min-width: 100rem"
      >
        <Column expander style="width: 5rem" />
        <Column field="id" header="ID" :sortable="true" style="width: 25%"></Column>
        <Column field="pretty_name" header="Title" :sortable="true" style="width: 25%">
            <template #editor="{data, field}">
                <InputText v-model="data[field]" :defaultValue="data[field]" autofocus fluid />
            </template>
        </Column>
        <Column field="percentage" header="Reading progress" :sortable="true">
          <template #body="slotProps">
            {{ Number(slotProps.data.percentage*100).toFixed(2) }}%
          </template>
        </Column>
        <Column field="device" header="Device" :sortable="true"></Column>
        <Column field="timestamp" header="Last read" :sortable="true">
          <template #body="slotProps">
            {{ new Date(slotProps.data.timestamp*1000).toISOString() }}
          </template>
        </Column>

        <template #expansion="slotProps">
          <div class="p-4 flex flex-col gap-2">
            <h3 class="text-2xl">History</h3>
            <div v-if="slotProps.data.history !== null">
              <DataTable :value="slotProps.data.history">
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
