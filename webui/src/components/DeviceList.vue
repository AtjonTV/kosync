<!--
  File:        webui/src/components/DeviceList.vue
  Project:     https://git.obth.eu/atjontv/kosync
  Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
-->
<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { errorMessage } from '@/pb'
import { useDevicesStore } from '@/stores/devices'
import type { Device } from '@/models'

const devices = useDevicesStore()

const showRename = ref(false)
const target = ref<Device | null>(null)
const newName = ref('')
const renameError = ref('')
const busy = ref(false)

const openRename = (device: Device) => {
  target.value = device
  newName.value = device.name || device.reported_name
  renameError.value = ''
  showRename.value = true
}

const rename = async () => {
  if (!target.value) return

  renameError.value = ''
  busy.value = true
  try {
    await devices.rename(target.value.id, newName.value.trim())
    showRename.value = false
  } catch (error) {
    renameError.value = errorMessage(error, 'Could not change the name.')
  } finally {
    busy.value = false
  }
}

const formatSeen = (value: string) => (value ? new Date(value).toLocaleString() : 'never')

onMounted(() => {
  if (!devices.loaded) devices.load()
})
</script>

<template>
  <Card class="bg-surface-0 dark:bg-surface-900 border border-surface-200 dark:border-surface-700">
    <template #title>
      <span class="text-xl font-semibold">Devices</span>
    </template>
    <template #content>
      <p class="mb-4 text-surface-600 dark:text-surface-400">
        Every device that has synced with KOsync appears here on its own. The name comes from
        KOReader, which usually calls a device something short rather than something recognisable —
        change it to whatever you call the thing, and that name is used wherever a device is named.
      </p>

      <DataTable v-if="devices.devices.length" :value="devices.devices" data-key="id">
        <Column header="Name">
          <template #body="{ data }">
            <span class="font-medium">{{ data.name || data.reported_name || data.device_id }}</span>
            <span
              v-if="data.name && data.reported_name && data.name !== data.reported_name"
              class="block text-xs text-surface-500 dark:text-surface-400"
            >
              reports itself as {{ data.reported_name }}
            </span>
          </template>
        </Column>
        <Column header="Last seen">
          <template #body="{ data }">
            <span class="text-surface-600 dark:text-surface-400">{{
              formatSeen(data.last_seen)
            }}</span>
          </template>
        </Column>
        <Column header="" style="width: 4rem">
          <template #body="{ data }">
            <Button
              icon="pi pi-pencil"
              variant="text"
              rounded
              title="Change name"
              @click="openRename(data)"
            />
          </template>
        </Column>
      </DataTable>

      <div v-else class="p-6 text-center text-surface-500 dark:text-surface-400">
        No device has synced yet. One appears here the first time KOReader pushes progress.
      </div>
    </template>
  </Card>

  <Dialog v-model:visible="showRename" header="Name this device" modal :style="{ width: '28rem' }">
    <form class="flex flex-col gap-4" @submit.prevent="rename">
      <p v-if="target" class="text-surface-600 dark:text-surface-400">
        KOReader reports this device as
        <span class="font-medium">{{ target.reported_name || target.device_id }}</span
        >. The name you give it here is used everywhere else.
      </p>
      <div class="flex flex-col gap-2">
        <label for="device-name">Name</label>
        <InputText id="device-name" v-model="newName" autofocus fluid />
      </div>
      <Message v-if="renameError" severity="error" variant="simple">{{ renameError }}</Message>
      <div class="flex justify-end gap-2">
        <Button type="button" label="Cancel" severity="secondary" @click="showRename = false" />
        <Button type="submit" label="Save" :loading="busy" />
      </div>
    </form>
  </Dialog>
</template>
