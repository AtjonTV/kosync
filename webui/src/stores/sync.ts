import {type Ref, ref} from 'vue'
import { defineStore } from 'pinia'
import JMPClient from "jmp-client-js";
import type {Document, DocumentWithHistory} from "@/models/document.ts";
import {fetchApi, getWebSocketUrl} from "@/api.ts";

export type SyncState = {
  lastSync: number,
  documents: DocumentWithHistory[],
}

export const useSyncStore = defineStore('sync', () => {
  const syncStateEncoded = sessionStorage.getItem('syncState')
  const syncState = syncStateEncoded === null ? null : JSON.parse(atob(syncStateEncoded))

  const sync: Ref<SyncState> = ref(syncState ?? {
    lastSync: -1,
    documents: [] as DocumentWithHistory[],
  })

  async function doSync(forceRefresh = false) {
    const now = Date.now();
    if (!forceRefresh && (now - sync.value.lastSync < 10_000)) return;

    const {data: documents, error} = await fetchApi<DocumentWithHistory[]>("/api/documents.all", {
      method: "GET"
    });

    if (error && error === "Unauthorized") {
      throw error;
    }

    if (documents !== null) {
      sync.value = {lastSync: now, documents};
    }

    sessionStorage.setItem('syncState', btoa(JSON.stringify(sync.value)))
  }

  async function doPubSubSync() {
    const socketUri = getWebSocketUrl();
    if (socketUri === null) {
      console.log("PubSub is only possible if a user is logged in, skipping.");
      return;
    }

    const client = new JMPClient(socketUri);
    client.connect(() => {
      client.subscribe("user.documents");
    });

    client.registerPubSubCallback("user.documents", (data, typeHint, errors) => {
      if (errors && errors.length > 0) {
        console.log(errors);
        return;
      }

      if (typeHint === "Document") {
        const doc = data as DocumentWithHistory;
        for (const docIndex in sync.value.documents) {
          const orig = sync.value.documents[docIndex];
          if (orig?.id === doc.id) {
            const origCopy = structuredClone(orig);
            delete (origCopy as any).history;
            let newHistory: Document[] = [];
            if (orig.history) {
              newHistory = [
                ...orig.history, // take full previous history
                origCopy // add original doc to history
              ];
            }
            sync.value.documents[docIndex] = {
              ...doc,
              history: newHistory
            };
            break;
          }
        }
      } else if (typeHint === "DocumentDeletion") {
        const {document_id} = data as {document_id: string};
        for (const docIndex in sync.value.documents) {
          const doc = sync.value.documents[docIndex];
          if (doc && doc.id === document_id) {
            sync.value.documents.splice(Number(docIndex), 1);
            break;
          }
        }
      } else if (typeHint === "HistoryDeletion") {
        const {document_id, last_read_at} = data as {document_id: string, last_read_at: number};
        for (const docIndex in sync.value.documents) {
          const doc = sync.value.documents[docIndex];
          if (doc && doc.id === document_id) {
            doc.history = doc.history?.filter(item => item.last_read_at !== last_read_at);
            break;
          }
        }
      }

      // Persist updated state
      // Note: Do not set lastSync, so that the next page-refresh causes a full sync
      sessionStorage.setItem('syncState', btoa(JSON.stringify(sync.value)))
    });
  }

  function clear() {
    sessionStorage.removeItem('syncState')
    sync.value = {lastSync: -1, documents: []}
  }

  async function deleteHistoryItem(documentId: string, lastReadAt: number) {
    const {error} = await fetchApi(`/api/documents.history.delete?id=${documentId}&last_read_at=${lastReadAt}`, {
      method: "DELETE"
    });

    if (error) {
      console.error("Failed to delete history item:", error);
      return;
    }

    // Refresh data
    await doSync(true);
  }

  return { sync, doSync, doPubSubSync, clear, deleteHistoryItem }
})
