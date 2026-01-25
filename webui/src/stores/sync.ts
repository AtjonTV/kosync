import { ref, computed } from 'vue'
import { defineStore } from 'pinia'
import type {SyncDoc} from "@/models/document.ts";
import {fetchApi} from "@/api.ts";

export const useSyncStore = defineStore('sync', () => {
  const syncStateEncoded = sessionStorage.getItem('syncState')
  const syncState = syncStateEncoded !== null ? JSON.parse(atob(syncStateEncoded)) : null

  const sync = ref(syncState ?? {
    lastSync: -1,
    documents: [] as SyncDoc[],
  })

  async function doSync() {
    const now = Date.now();
    if (now - sync.value.lastSync < 60_000) return;

    const {data: documents, error} = await fetchApi<SyncDoc[]>("/api/documents.all", {
      method: "GET"
    });

    if (documents !== null) {
      sync.value = {documents};
    }

    sessionStorage.setItem('syncState', btoa(JSON.stringify(sync.value)))
  }

  function clear() {
    sessionStorage.removeItem('syncState')
    sync.value = {lastSync: -1, documents: []}
  }

  return { sync, doSync, clear }
})
