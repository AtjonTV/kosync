import {type Ref, ref} from 'vue'
import { defineStore } from 'pinia'
import JMPClient from "jmp-client-js";
import type {Document, DocumentWithHistory} from "@/models/document.ts";
import {getWebSocketUrl} from "@/api.ts";

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

  const client = ref<JMPClient | null>(null);
  let connectingPromise: Promise<JMPClient> | null = null;

  async function getClient(): Promise<JMPClient> {
      if (client.value) return client.value as JMPClient;
      if (connectingPromise) return connectingPromise;

      connectingPromise = (async () => {
          try {
              const socketUri = getWebSocketUrl();
              if (!socketUri) throw new Error("No websocket URL");

              const newClient = new JMPClient(socketUri, true);
              await new Promise((resolve) => newClient.connect(resolve as any));
              client.value = newClient;
              return newClient;
          } finally {
              connectingPromise = null;
          }
      })();

      return connectingPromise;
  }

  async function doSync(forceRefresh = false) {
    const now = Date.now();
    if (!forceRefresh && (now - sync.value.lastSync < 10_000)) return;

    try {
      const c = await getClient();
      const documents = await c.rpc("documents.all", {});

      if (documents !== null) {
        sync.value = {lastSync: now, documents: documents as DocumentWithHistory[]};
      }

      sessionStorage.setItem('syncState', btoa(JSON.stringify(sync.value)))
    } catch (e) {
      console.error("Sync failed:", e);
    }
  }

  async function doPubSubSync() {
    const c = await getClient();
    c.subscribe("user.documents", (data, typeHint, errors) => {
      if (errors && errors.length > 0) {
        console.log(errors);
        return;
      }

      if (typeHint === "Document") {
        const doc = data as DocumentWithHistory;
        for (const docIndex in sync.value.documents) {
          const orig = sync.value.documents[docIndex];
          if (orig?.id === doc.id) {
            // Use JSON stringify and parse because structuredClone does not work on Proxy objects
            const origCopy = JSON.parse(JSON.stringify(orig));
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
    try {
      const c = await getClient();
      await c.rpc("documents.history.delete", {document_id: documentId, last_read_at: lastReadAt});
    } catch (e) {
      console.error("Failed to delete history item:", e);
      return;
    }
  }

  async function restoreHistoryItem(documentId: string, lastReadAt: number) {
    try {
      const c = await getClient();
      await c.rpc("documents.history.restore", {document_id: documentId, last_read_at: lastReadAt});
    } catch (e) {
      console.error("Failed to restore history item:", e);
      return;
    }
  }

  async function updateDocument(data: any) {
    try {
      const c = await getClient();
      await c.rpc("documents.update", {document: data});
    } catch (e) {
      console.error("Failed to update document:", e);
      return;
    }
  }

  async function deleteDocument(documentId: string) {
    try {
      const c = await getClient();
      await c.rpc("documents.delete", {document_id: documentId});
    } catch (e) {
      console.error("Failed to delete document:", e);
      return;
    }
  }

  return { sync, doSync, doPubSubSync, clear, deleteHistoryItem, restoreHistoryItem, updateDocument, deleteDocument }
})
