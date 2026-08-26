//
// File:        webui/src/stores/devices.ts
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

import { computed, ref } from 'vue'
import { defineStore } from 'pinia'
import { pb, Collections } from '@/pb'
import type { Device } from '@/models'

/**
 * The devices that have pushed progress.
 *
 * The rows are made by the server as pushes arrive; the only thing editable
 * here is the name. Everything else in KOsync identifies a device by
 * `device_id`, which is a hex string — this store is what turns one of those
 * into something a person recognises.
 */
export const useDevicesStore = defineStore('devices', () => {
  const devices = ref<Device[]>([])
  const loading = ref(false)
  const loaded = ref(false)

  let unsubscribe: (() => void) | null = null

  async function load(): Promise<void> {
    loading.value = true
    try {
      devices.value = await pb.collection(Collections.devices).getFullList<Device>({
        sort: '-last_seen',
      })
      loaded.value = true
    } finally {
      loading.value = false
    }
  }

  /** Changes the name. The rest of a device row describes what it reported. */
  async function rename(id: string, name: string): Promise<void> {
    await pb.collection(Collections.devices).update(id, { name })
    await load()
  }

  async function subscribe(): Promise<void> {
    if (unsubscribe) return

    unsubscribe = await pb.collection(Collections.devices).subscribe<Device>('*', (event) => {
      const index = devices.value.findIndex((device) => device.id === event.record.id)

      if (event.action === 'delete') {
        if (index !== -1) devices.value.splice(index, 1)

        return
      }

      if (index === -1) {
        devices.value.push(event.record)
      } else {
        devices.value[index] = event.record
      }
    })
  }

  function stop(): void {
    unsubscribe?.()
    unsubscribe = null
  }

  /** The devices keyed by the identifier everything else stores. */
  const byDeviceId = computed(
    () => new Map(devices.value.map((device) => [device.device_id, device])),
  )

  /**
   * What to call a device, best first: the name its owner chose, then the one it
   * reports, then the identifier.
   *
   * The identifier is a last resort rather than a blank, because a device whose
   * name never arrived is still a device and the hex at least tells two of them
   * apart.
   *
   * A computed returning a function rather than a plain one, so that it stays a
   * getter: Pinia treats a bare function on a setup store as an action, and an
   * action is exactly the thing a test double replaces.
   */
  const nameOf = computed(() => (deviceId: string): string => {
    if (!deviceId) return ''

    const device = byDeviceId.value.get(deviceId)

    return device?.name || device?.reported_name || deviceId
  })

  function clear(): void {
    stop()
    devices.value = []
    loaded.value = false
  }

  return { devices, loading, loaded, byDeviceId, load, rename, subscribe, stop, nameOf, clear }
})
