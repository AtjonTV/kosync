//
// File:        webui/src/tests/components/BookView.test.ts
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { createTestingPinia } from '@pinia/testing'
import PrimeVue from 'primevue/config'
import BookView from '@/views/BookView.vue'
import { useBookStatsStore } from '@/stores/bookStats'
import type { Book, Device, ReadingBookDay } from '@/models'

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
    fileUrl: () => 'blob:cover',
  }
})

vi.mock('vue-router', () => ({
  useRoute: () => ({ params: { id: 'book-a' } }),
}))

function book(overrides: Partial<Book> = {}): Book {
  return {
    id: 'book-a',
    collectionId: 'c',
    collectionName: 'books',
    created: '',
    updated: '',
    owner: 'user-a',
    file: 'book.epub',
    cover: 'cover.jpg',
    title: 'Zeit des Sturms',
    authors: ['Andrzej Sapkowski'],
    language: 'de',
    identifiers: {},
    page_count: 500,
    word_count: 109288,
    file_size: 1_200_000,
    content_hash: 'abc',
    hash_binary: 'bin',
    hash_filename: 'name',
    measured_pages: 0,
    measured_device: '',
    measured_through: '',
    ...overrides,
  }
}

function bookDay(date: string, overrides: Partial<ReadingBookDay> = {}): ReadingBookDay {
  return {
    id: 'row-' + date,
    collectionId: 'c',
    collectionName: 'reading_book_days',
    created: '',
    updated: '',
    owner: 'user-a',
    book: 'book-a',
    date,
    update_count: 6,
    progress_increase: 8,
    reading_time: 3600,
    documents_touched: 1,
    pages_read: 40,
    computed_at: '',
    ...overrides,
  }
}

function device(overrides: Partial<Device> = {}): Device {
  return {
    id: 'device-a',
    collectionId: 'c',
    collectionName: 'devices',
    created: '',
    updated: '',
    owner: 'user-a',
    device_id: 'go7',
    reported_name: 'go7',
    name: '',
    last_seen: '',
    ...overrides,
  }
}

function mountBook(entry: Book, days: ReadingBookDay[] = [], known: Device[] = [device()]) {
  return mount(BookView, {
    global: {
      plugins: [
        createTestingPinia({
          createSpy: vi.fn,
          initialState: {
            books: { books: [entry], loaded: true },
            bookStats: { days },
            documents: { documents: [], loaded: true },
            devices: { devices: known, loaded: true },
          },
        }),
        PrimeVue,
      ],
      stubs: { Chart: true, RouterLink: { template: '<a><slot /></a>' } },
    },
  })
}

describe('BookView', () => {
  it('loads the statistics of the book in the route', () => {
    mountBook(book())

    expect(useBookStatsStore().load).toHaveBeenCalledWith('book-a')
  })

  // A count recovered from the reading is the device's own pagination, and
  // saying which device it came from is what makes it checkable.
  it('names the device a measured page count came from', () => {
    const wrapper = mountBook(book({ measured_pages: 700, measured_device: 'go7' }))

    expect(wrapper.text()).toContain('700')
    expect(wrapper.text()).toContain('measured on go7')
  })

  // The book stores the identifier, which is a hex string on a real device. What
  // belongs on screen is whatever its owner decided to call the thing.
  it('uses the name the owner gave the device', () => {
    const wrapper = mountBook(
      book({ measured_pages: 700, measured_device: '865F46C0C0F4401D9A05768B6B0BF3AC' }),
      [],
      [device({ device_id: '865F46C0C0F4401D9A05768B6B0BF3AC', name: 'Boox Go 7' })],
    )

    expect(wrapper.text()).toContain('measured on Boox Go 7')
    expect(wrapper.text()).not.toContain('865F46C0C0F4401D9A05768B6B0BF3AC')
  })

  // An EPUB has no pages of its own, so an unmeasured count is a guess from the
  // word count and has to say so rather than pass itself off as the real one.
  it('marks a page count derived from the word count as an estimate', () => {
    const wrapper = mountBook(book())

    expect(wrapper.text()).toContain('estimated from the word count')
  })

  it('sums the days the book was read on', () => {
    const wrapper = mountBook(book(), [
      bookDay('2026-03-01', { reading_time: 3600, pages_read: 40 }),
      bookDay('2026-03-02', { reading_time: 1800, pages_read: 25 }),
    ])

    // 5400 seconds is an hour and a half, over two days, 65 pages.
    expect(wrapper.text()).toContain('1 h 30 min')
    expect(wrapper.text()).toContain('65')
  })

  it('says so when a book has no reading recorded yet', () => {
    const wrapper = mountBook(book())

    expect(wrapper.text()).toContain('No reading recorded for this book yet')
  })

  it('releases the statistics when it goes away', () => {
    const wrapper = mountBook(book())
    wrapper.unmount()

    expect(useBookStatsStore().clear).toHaveBeenCalled()
  })
})
