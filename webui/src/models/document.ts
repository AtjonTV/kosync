
export interface SyncDoc {
  id: string;
  document: SyncDocData;
  document_history: SyncDocData[];
}

export interface SyncDocData {
  document: string;
  progress: string;
  percentage: number;
  device: string;
  device_id: string;
  timestamp: number;
}
