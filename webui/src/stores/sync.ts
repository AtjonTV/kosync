import { ref } from 'vue'
import { defineStore } from 'pinia'
import type {SyncDoc} from "@/models/document.ts";
import {fetchApi} from "@/api.ts";

export const useSyncStore = defineStore('sync', () => {
  const syncStateEncoded = sessionStorage.getItem('syncState')
  const syncState = syncStateEncoded === null ? null : JSON.parse(atob(syncStateEncoded))

  const sync = ref(syncState ?? {
    lastSync: -1,
    documents: [] as SyncDoc[],
  })

  async function doSync(forceRefresh = false) {
    const now = Date.now();
    if (!forceRefresh && (now - sync.value.lastSync < 10_000)) return;

    const {data: documents} = await fetchApi<SyncDoc[]>("/api/documents.all", {
      method: "GET"
    });

    if (documents !== null) {
      sync.value = {lastSync: now, documents};
    }

    sessionStorage.setItem('syncState', btoa(JSON.stringify(sync.value)))
  }

  function clear() {
    sessionStorage.removeItem('syncState')
    sync.value = {lastSync: -1, documents: []}
  }

  return { sync, doSync, clear }
})
