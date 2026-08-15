//
// File:        webui/src/pb.ts
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

import PocketBase from 'pocketbase'

// The WebUI is served by the KOsync server itself, so the API lives on the same
// origin. During "bun run dev" Vite proxies /api to a local server.
export const pb = new PocketBase(window.location.origin)

// Realtime subscriptions are the reason this is disabled: PocketBase would
// otherwise cancel a pending request whenever a new one with the same key
// starts, which breaks parallel loads on the dashboard.
pb.autoCancellation(false)

/**
 * Collection names, so a typo shows up here instead of in a failed request.
 */
export const Collections = {
  users: 'users',
  koreaderAccounts: 'koreader_accounts',
  documents: 'documents',
  documentHistory: 'document_history',
  documentAliases: 'document_aliases',
  readingDays: 'reading_days',
  readingMonths: 'reading_months',
  books: 'books',
  readingBookDays: 'reading_book_days',
  devices: 'devices',
  achievements: 'achievements',
} as const

/**
 * Endpoints of the KOsync specific API.
 */
export const KosyncApi = {
  koreaderAccounts: '/api/kosync/koreader-accounts',
  koreaderAccountPassword: (id: string) => `/api/kosync/koreader-accounts/${id}/password`,
  restoreHistory: (documentId: string, historyId: string) =>
    `/api/kosync/documents/${documentId}/restore/${historyId}`,
  mergeDocuments: '/api/kosync/documents/merge',
  achievements: '/api/kosync/achievements',
} as const

/**
 * The URL of a file stored on a record, optionally as a generated thumbnail.
 *
 * Returns an empty string when the field is unset, so a book without a cover
 * simply renders no image instead of a broken one.
 */
export function fileUrl(
  record: { id: string; collectionId: string; collectionName: string },
  filename: string,
  thumb?: string,
): string {
  if (!filename) return ''

  return pb.files.getURL(record, filename, thumb ? { thumb } : undefined)
}

/**
 * The IANA timezone name this browser is set to, such as "Europe/Vienna".
 *
 * This is the only place KOsync can learn it. The KOReader sync protocol
 * carries no clock at all — the push is a document, a position and a device,
 * and the headers are authentication — so a reading day would otherwise have to
 * begin at UTC midnight, which for most of the world falls in the middle of an
 * evening's reading.
 *
 * Falls back to UTC, which is what every stored timestamp already is, so a
 * browser that will not say leaves the numbers exactly as they were.
 */
export function browserTimezone(): string {
  try {
    return Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC'
  } catch {
    return 'UTC'
  }
}

/**
 * Every timezone name this browser knows, for choosing one by hand.
 *
 * `supportedValuesOf` is not in every engine, so a browser without it gets the
 * one name that matters — its own — rather than an empty list.
 */
export function timezoneNames(): string[] {
  try {
    const supported = (
      Intl as unknown as { supportedValuesOf?: (key: string) => string[] }
    ).supportedValuesOf?.('timeZone')

    if (supported?.length) return supported
  } catch {
    // falls through to the pair below
  }

  return Array.from(new Set(['UTC', browserTimezone()]))
}

/**
 * Turns a failed request into a message that can be shown to a person.
 */
export function errorMessage(error: unknown, fallback = 'Something went wrong.'): string {
  if (error && typeof error === 'object') {
    const response = (error as { response?: { message?: string } }).response
    if (response?.message) return response.message

    const message = (error as { message?: string }).message
    if (message) return message
  }

  return fallback
}
