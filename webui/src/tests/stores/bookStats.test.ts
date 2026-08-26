//
// File:        webui/src/tests/stores/bookStats.test.ts
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

import { beforeEach, describe, expect, it, vi } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import * as pbMockModule from '../mocks/pb'

vi.mock('@/pb', async () => {
  const mock = await import('../mocks/pb')
  const actual = await vi.importActual<typeof import('@/pb')>('@/pb')

  return {
    pb: mock.pbMock,
    Collections: actual.Collections,
    KosyncApi: actual.KosyncApi,
    errorMessage: actual.errorMessage,
    browserTimezone: actual.browserTimezone,
    timezoneNames: actual.timezoneNames,
    fileUrl: actual.fileUrl,
  }
})

import { useBookStatsStore } from '@/stores/bookStats'
import type { ReadingBookDay } from '@/models'

function bookDay(date: string, overrides: Partial<ReadingBookDay> = {}): ReadingBookDay {
  return {
    id: 'row-' + date,
    collectionId: 'c',
    collectionName: 'reading_book_days',
    created: date + ' 00:00:00.000Z',
    updated: date + ' 00:00:00.000Z',
    owner: 'user-a',
    book: 'book-a',
    date,
    update_count: 6,
    progress_increase: 8,
    reading_time: 600,
    documents_touched: 1,
    pages_read: 40,
    computed_at: date + ' 00:00:00.000Z',
    ...overrides,
  }
}

describe('book statistics store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    pbMockModule.reset()
  })

  it('asks for one book, oldest day first', async () => {
    const store = useBookStatsStore()
    await store.load('book-a')

    const collection = pbMockModule.collections.get('reading_book_days')
    expect(collection?.getFullList).toHaveBeenCalledWith({
      filter: "book = 'book-a'",
      sort: 'date',
    })
  })

  it('adds up the days it loaded', async () => {
    const collection = pbMockModule.collection('reading_book_days')
    collection.getFullList.mockResolvedValue([
      bookDay('2026-03-01', { reading_time: 600, pages_read: 40 }),
      bookDay('2026-03-04', { reading_time: 1800, pages_read: 95 }),
    ])

    const store = useBookStatsStore()
    await store.load('book-a')

    expect(store.totals.days).toBe(2)
    expect(store.totals.readingTime).toBe(2400)
    expect(store.totals.pagesRead).toBe(135)
    expect(store.totals.first).toBe('2026-03-01')
    expect(store.totals.last).toBe('2026-03-04')
    expect(store.bestDay?.date).toBe('2026-03-04')
  })

  // The subscription covers the whole collection, because PocketBase filters a
  // realtime feed by the collection's list rule rather than by a client filter.
  it('ignores live rows belonging to another book', async () => {
    const store = useBookStatsStore()
    await store.load('book-a')
    await store.subscribe()

    pbMockModule.emit('reading_book_days', 'create', bookDay('2026-03-02', { book: 'book-b' }))
    expect(store.days).toHaveLength(0)

    pbMockModule.emit('reading_book_days', 'create', bookDay('2026-03-02'))
    expect(store.days).toHaveLength(1)
  })

  it('folds live updates and deletions into the loaded days', async () => {
    const collection = pbMockModule.collection('reading_book_days')
    collection.getFullList.mockResolvedValue([bookDay('2026-03-01')])

    const store = useBookStatsStore()
    await store.load('book-a')
    await store.subscribe()

    pbMockModule.emit('reading_book_days', 'update', bookDay('2026-03-01', { pages_read: 111 }))
    expect(store.days[0]?.pages_read).toBe(111)

    pbMockModule.emit('reading_book_days', 'delete', bookDay('2026-03-01'))
    expect(store.days).toHaveLength(0)
  })

  it('keeps the days in date order when a live row arrives out of order', async () => {
    const collection = pbMockModule.collection('reading_book_days')
    collection.getFullList.mockResolvedValue([bookDay('2026-03-04')])

    const store = useBookStatsStore()
    await store.load('book-a')
    await store.subscribe()

    pbMockModule.emit('reading_book_days', 'create', bookDay('2026-03-01'))

    expect(store.days.map((day) => day.date)).toEqual(['2026-03-01', '2026-03-04'])
  })

  it('drops everything when it is cleared', async () => {
    const collection = pbMockModule.collection('reading_book_days')
    collection.getFullList.mockResolvedValue([bookDay('2026-03-01')])

    const store = useBookStatsStore()
    await store.load('book-a')
    store.clear()

    expect(store.days).toHaveLength(0)
    expect(store.bookId).toBe('')
    expect(store.bestDay).toBeNull()
  })
})
