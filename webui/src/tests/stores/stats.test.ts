//
// File:        webui/src/tests/stores/stats.test.ts
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
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
  }
})

import { useStatsStore, toDateKey } from '@/stores/stats'
import type { ReadingDay } from '@/models'

function day(date: string, overrides: Partial<ReadingDay> = {}): ReadingDay {
  return {
    id: 'day-' + date,
    collectionId: 'c',
    collectionName: 'reading_days',
    created: date + ' 00:00:00.000Z',
    updated: date + ' 00:00:00.000Z',
    owner: 'user-a',
    date,
    update_count: 4,
    progress_increase: 12.5,
    reading_time: 600,
    documents_touched: 1,
    pages_read: 0,
    computed_at: date + ' 00:00:00.000Z',
    ...overrides,
  }
}

describe('stats store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    pbMockModule.reset()
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-03-10T12:00:00Z'))
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('asks for the days inside the requested range', async () => {
    const store = useStatsStore()
    await store.load(7)

    const call = pbMockModule.collection('reading_days').getFullList.mock.calls[0]![0]
    // 7 days including today means back to the 4th.
    expect(call.filter).toBe("date >= '2026-03-04'")
    expect(call.sort).toBe('date')
  })

  it('fills the days without reading with zeroes', async () => {
    pbMockModule
      .collection('reading_days')
      .getFullList.mockResolvedValue([day('2026-03-09'), day('2026-03-10')])

    const store = useStatsStore()
    await store.load(3)

    expect(store.series.map((entry) => entry.date)).toEqual([
      '2026-03-08',
      '2026-03-09',
      '2026-03-10',
    ])
    expect(store.series[0]!.updates).toBe(0)
    expect(store.series[1]!.updates).toBe(4)
  })

  it('sums the reading time of the loaded range in seconds', async () => {
    pbMockModule
      .collection('reading_days')
      .getFullList.mockResolvedValue([
        day('2026-03-09', { reading_time: 600 }),
        day('2026-03-10', { reading_time: 300 }),
      ])

    const store = useStatsStore()
    await store.load(14)

    expect(store.readingSeconds).toBe(900)
  })

  it('applies a recomputed day that arrives while subscribed', async () => {
    pbMockModule.collection('reading_days').getFullList.mockResolvedValue([day('2026-03-10')])

    const store = useStatsStore()
    await store.load(3)
    await store.subscribe()

    pbMockModule.emit('reading_days', 'update', day('2026-03-10', { update_count: 42 }))

    expect(store.series.at(-1)!.updates).toBe(42)
  })

  it('inserts a day that appears while subscribed, in order', async () => {
    const store = useStatsStore()
    await store.load(3)
    await store.subscribe()

    pbMockModule.emit('reading_days', 'create', day('2026-03-09'))

    expect(store.days.map((entry) => entry.date)).toEqual(['2026-03-09'])
    expect(store.series[1]!.updates).toBe(4)
  })

  it('drops a day that was removed by the retention job', async () => {
    pbMockModule.collection('reading_days').getFullList.mockResolvedValue([day('2026-03-10')])

    const store = useStatsStore()
    await store.load(3)
    await store.subscribe()

    pbMockModule.emit('reading_days', 'delete', day('2026-03-10'))

    expect(store.days).toHaveLength(0)
    expect(store.series.at(-1)!.updates).toBe(0)
  })

  it('formats a date key in UTC', () => {
    expect(toDateKey(new Date('2026-03-10T23:30:00Z'))).toBe('2026-03-10')
  })
})
