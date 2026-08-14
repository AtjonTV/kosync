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
