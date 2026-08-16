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
  /** The series this volume belongs to, empty if it belongs to none. */
  series: string
  /** Where in that series it sits. Not an integer: half-numbered volumes exist. */
  series_index: number
  /**
   * What the file says the book is about, as the publisher wrote it. Null for
   * a book that declares none, and of very mixed quality when it does.
   */
  subjects: string[] | null
  page_count: number
  word_count: number
  /** How many bytes the uploaded file takes. Covers are not counted. */
  file_size: number
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

/**
 * A shelf its owner put together by hand.
 *
 * `books` is a list of book ids in the order they were put there, and that order
 * is kept: a reading list is a sequence, not a set. It is the one thing in the
 * library nobody derived from a file — everything else here is what an EPUB said
 * or what a device reported.
 */
export interface BookCollection extends BaseRecord {
  owner: string
  name: string
  description: string
  books: string[]
}

/**
 * A device that has pushed progress.
 *
 * `device_id` is KOReader's own identifier and is what everything groups by,
 * because it survives a rename. `reported_name` is what the device calls itself,
 * and `name` is what its owner calls it.
 */
export interface Device extends BaseRecord {
  owner: string
  device_id: string
  reported_name: string
  name: string
  last_seen: string
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

/** One tier of an achievement, as it was awarded. */
export interface EarnedTier {
  tier: number
  value: number
  at: string
}

/**
 * One achievement rule with the account's standing in it.
 *
 * The rule half — name, summary, tiers, which cat — comes from the server
 * rather than being defined here, so there is one place for it.
 */
export interface Achievement {
  rule: string
  name: string
  summary: string
  unit: string
  icon: string
  fur: string
  tiers: number[]
  /** The measure right now. */
  value: number
  /** The highest tier reached, zero for none. */
  tier: number
  /** The next threshold, zero when every tier is done. */
  next: number
  earned: EarnedTier[]
}

/**
 * How much room an account's library takes, and how much it may take.
 *
 * `quota` is zero when the operator has not set a limit, which is not the same
 * as a full library and has to be told apart from one.
 */
export interface StorageUsage {
  books: number
  used: number
  quota: number
}
