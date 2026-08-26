//
// File:        webui/src/tests/mocks/pb.ts
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

import { vi } from 'vitest'

/**
 * A stand-in for the PocketBase SDK.
 *
 * The stores are the part worth testing here: what they ask the SDK for, and
 * how they fold realtime events into their state. The SDK itself is somebody
 * else's tested code.
 */
export interface CollectionMock {
  getFullList: ReturnType<typeof vi.fn>
  getOne: ReturnType<typeof vi.fn>
  create: ReturnType<typeof vi.fn>
  update: ReturnType<typeof vi.fn>
  delete: ReturnType<typeof vi.fn>
  subscribe: ReturnType<typeof vi.fn>
  authWithPassword: ReturnType<typeof vi.fn>
  authRefresh: ReturnType<typeof vi.fn>
  requestVerification: ReturnType<typeof vi.fn>
  requestPasswordReset: ReturnType<typeof vi.fn>
  requestEmailChange: ReturnType<typeof vi.fn>
}

export function createCollectionMock(): CollectionMock {
  return {
    getFullList: vi.fn().mockResolvedValue([]),
    getOne: vi.fn().mockResolvedValue({}),
    create: vi.fn().mockResolvedValue({}),
    update: vi.fn().mockResolvedValue({}),
    delete: vi.fn().mockResolvedValue(true),
    subscribe: vi.fn().mockResolvedValue(() => {}),
    authWithPassword: vi.fn().mockResolvedValue({}),
    authRefresh: vi.fn().mockResolvedValue({}),
    requestVerification: vi.fn().mockResolvedValue(true),
    requestPasswordReset: vi.fn().mockResolvedValue(true),
    requestEmailChange: vi.fn().mockResolvedValue(true),
  }
}

/** The collections a test touched, keyed by name. */
export const collections = new Map<string, CollectionMock>()

/** Handlers registered through subscribe(), keyed by collection name. */
export const subscriptions = new Map<string, (event: { action: string; record: unknown }) => void>()

export const send = vi.fn().mockResolvedValue({})

export const authStore = {
  record: null as Record<string, unknown> | null,
  isValid: false,
  onChange: vi.fn(),
  clear: vi.fn(function clear() {
    authStore.record = null
    authStore.isValid = false
  }),
}

export function collection(name: string): CollectionMock {
  let existing = collections.get(name)
  if (!existing) {
    existing = createCollectionMock()
    existing.subscribe.mockImplementation(
      async (_topic: string, handler: (event: { action: string; record: unknown }) => void) => {
        subscriptions.set(name, handler)
        return () => subscriptions.delete(name)
      },
    )
    collections.set(name, existing)
  }

  return existing
}

/** Emits a realtime event to whatever the store subscribed with. */
export function emit(collectionName: string, action: string, record: unknown): void {
  const handler = subscriptions.get(collectionName)
  if (!handler) throw new Error(`nothing subscribed to ${collectionName}`)
  handler({ action, record })
}

export function reset(): void {
  collections.clear()
  subscriptions.clear()
  send.mockClear()
  authStore.record = null
  authStore.isValid = false
}

export const pbMock = {
  collection: vi.fn(collection),
  send,
  authStore,
  autoCancellation: vi.fn(),
  filter: (expression: string, params: Record<string, unknown>) => {
    // Good enough for assertions: the real implementation escapes and inlines
    // the parameters the same way.
    return Object.entries(params).reduce(
      (result, [key, value]) => result.replace(`{:${key}}`, `'${String(value)}'`),
      expression,
    )
  },
}
