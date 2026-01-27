
export interface SyncDoc extends SyncDocData {
    id: string;
    pretty_name: string;
    history: SyncDocData[];
}

export interface SyncDocData {
  document: string;
  progress: string;
  percentage: number;
  device: string;
  device_id: string;
  timestamp: number;
}
