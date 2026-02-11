
export interface DocumentWithHistory extends Document {
    history: Document[];
}

export interface Document {
  id: string;
  owner_id: string;
  title: string;
  current_location: string;
  progress: number;
  last_read_on_device: string;
  last_read_on_device_id: string;
  last_read_at: number;
}
