//
// File:        webui/src/models/index.ts
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

/** A record as PocketBase returns it. */
export interface BaseRecord {
  id: string
  collectionId: string
  collectionName: string
  created: string
  updated: string
}

/** The current reading position of a document. */
export interface DocumentRecord extends BaseRecord {
  owner: string
  document: string
  title: string
  current_location: string
  progress: number
  last_device: string
  last_device_id: string
  last_read_at: string
  source_account: string
  /** The uploaded book this is progress through, empty when unmatched. */
  book: string
}

/** A superseded reading position. */
export interface HistoryRecord extends BaseRecord {
  owner: string
  document_ref: string
  title: string
  current_location: string
  progress: number
  last_device: string
  last_device_id: string
  last_read_at: string
}

/** One precomputed day of reading. */
export interface ReadingDay extends BaseRecord {
  owner: string
  date: string
  update_count: number
  progress_increase: number
  reading_time: number
  documents_touched: number
  pages_read: number
  computed_at: string
}

/** A KOReader device credential. */
export interface KoreaderAccount extends BaseRecord {
  owner: string
  username: string
  label: string
  disabled: boolean
  last_used: string
}

/** An uploaded EPUB and the metadata read out of it. */
export interface Book extends BaseRecord {
  owner: string
  file: string
  cover: string
  title: string
  authors: string[]
  language: string
  identifiers: Record<string, string>
  page_count: number
  word_count: number
  content_hash: string
  hash_binary: string
  hash_filename: string
  /** The page count recovered from the progress a device pushed, 0 if none. */
  measured_pages: number
  /** Which device that measurement came from. */
  measured_device: string
  /** How far into the reading the measurement looked. */
  measured_through: string
}

/** One precomputed day of reading in one book. */
export interface ReadingBookDay extends BaseRecord {
  owner: string
  book: string
  date: string
  update_count: number
  progress_increase: number
  reading_time: number
  documents_touched: number
  pages_read: number
  computed_at: string
}

/** A document together with the states it went through. */
export interface DocumentWithHistory extends DocumentRecord {
  history: HistoryRecord[]
}
