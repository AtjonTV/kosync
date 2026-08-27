//
// File:        webui/src/pb.ts
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

import { ref } from 'vue'
import PocketBase, { isTokenExpired } from 'pocketbase'

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
  // Not "collections": PocketBase calls its own tables that, and the API path
  // would then read /api/collections/collections/records.
  bookCollections: 'book_collections',
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
  bookPreview: (id: string) => `/api/kosync/books/${id}/preview`,
  bookPreviewChapter: (id: string, index: number) => `/api/kosync/books/${id}/preview/${index}`,
  storage: '/api/kosync/storage',
} as const

/**
 * The token that opens the library's files.
 *
 * A book and its cover are protected files: the server hands them over only to
 * a request carrying a short lived token, and checks the same rule against it
 * that decides who may see the record. An <img> cannot send an Authorization
 * header, so the token rides in the address instead.
 *
 * It is kept until it actually runs out rather than being renewed on a timer,
 * because every new token rewrites the address of every cover on the page and
 * the browser then fetches all of them again.
 */
const fileToken = ref('')

/** The account the held token belongs to, so a different one drops it. */
let fileTokenOwner = ''

/** The request in flight, so a page full of covers only asks once. */
let fileTokenRequest: Promise<void> | null = null

/** When another attempt may be made, after one failed. */
let fileTokenRetryAt = 0

/** How long to wait before asking again after a failure. */
const fileTokenRetryMs = 5000

/** How close to running out a token may be before it is replaced. */
const fileTokenMarginSeconds = 60

/**
 * Asks for a file token, unless one is already on its way.
 *
 * A failure leaves the held token alone — it may well still work — and holds
 * off the next attempt, so a server that is refusing does not collect one
 * request per rendered cover.
 */
function requestFileToken(): void {
  if (fileTokenRequest || Date.now() < fileTokenRetryAt) return

  fileTokenRequest = pb.files
    .getToken()
    .then((token) => {
      fileToken.value = token
    })
    .catch(() => {
      fileTokenRetryAt = Date.now() + fileTokenRetryMs
    })
    .finally(() => {
      fileTokenRequest = null
    })
}

// A token belongs to the account it was issued to, so signing out or signing in
// as somebody else drops it. A plain token refresh is not a change of account
// and must not, or every cover on screen would be fetched twice over.
pb.authStore.onChange(() => {
  const owner = pb.authStore.record?.id ?? ''
  if (owner === fileTokenOwner) return

  fileTokenOwner = owner
  fileToken.value = ''
  fileTokenRetryAt = 0
})

/**
 * The URL of a file stored on a record, optionally as a generated thumbnail.
 *
 * Returns an empty string when the field is unset and when no token has arrived
 * yet, so a book without a cover and a page that is still asking both render no
 * image rather than a broken one. The token is a reactive value: whatever read
 * it renders again, with a real address, as soon as it is there.
 */
export function fileUrl(
  record: { id: string; collectionId: string; collectionName: string },
  filename: string,
  thumb?: string,
): string {
  if (!filename) return ''

  const token = fileToken.value
  if (pb.authStore.isValid && (!token || isTokenExpired(token, fileTokenMarginSeconds))) {
    requestFileToken()
  }

  if (!token) return ''

  return pb.files.getURL(record, filename, thumb ? { thumb, token } : { token })
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

/**
 * A number of bytes, written the way a person would say it.
 *
 * Binary units under their decimal names, which is what every file manager and
 * every e-reader shows. This has to match the server's own formatting, because
 * the bar and the message refusing an upload are read together.
 */
export function formatBytes(bytes: number): string {
  if (!Number.isFinite(bytes) || bytes < 0) bytes = 0
  if (bytes < 1024) return `${Math.round(bytes)} B`

  const units = ['KB', 'MB', 'GB', 'TB']
  let value = bytes
  let name = units[0]

  for (const next of units) {
    name = next
    value /= 1024
    if (value < 1024) break
  }

  return value < 10 ? `${value.toFixed(1)} ${name}` : `${Math.round(value)} ${name}`
}
