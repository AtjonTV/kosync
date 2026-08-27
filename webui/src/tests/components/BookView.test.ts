//
// File:        webui/src/tests/components/BookView.test.ts
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

import { beforeEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { createTestingPinia } from '@pinia/testing'
import PrimeVue from 'primevue/config'
import ToastService from 'primevue/toastservice'
import BookView from '@/views/BookView.vue'
import { useBookStatsStore } from '@/stores/bookStats'
import { useCollectionsStore } from '@/stores/collections'
import type { Book, BookCollection, Device, ReadingBookDay } from '@/models'

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

const push = vi.fn()

vi.mock('vue-router', () => ({
  useRoute: () => ({ params: { id: 'book-a' } }),
  useRouter: () => ({ push }),
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
    description: '',
    series: '',
    series_index: 0,
    subjects: null,
    page_count: 500,
    word_count: 109288,
    file_size: 1_200_000,
    content_hash: 'abc',
    hash_binary: 'bin',
    hash_filename: 'name',
    measured_pages: 0,
    measured_device: '',
    measured_through: '',
    measured_source: '',
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

function shelf(id: string, name: string, books: string[]): BookCollection {
  return {
    id,
    collectionId: 'c',
    collectionName: 'book_collections',
    created: '',
    updated: '',
    owner: 'user-a',
    name,
    description: '',
    books,
  }
}

function mountBook(
  entry: Book,
  days: ReadingBookDay[] = [],
  known: Device[] = [device()],
  shelves: BookCollection[] = [],
) {
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
            collections: { collections: shelves, loaded: true },
          },
        }),
        PrimeVue,
        ToastService,
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
    const wrapper = mountBook(
      book({ measured_pages: 700, measured_device: 'go7', measured_source: 'progress' }),
    )

    expect(wrapper.text()).toContain('700')
    expect(wrapper.text()).toContain('measured on go7')
  })

  // A count the reader stated in the statistics it synced is the better of the
  // two, and the only one a very long book can have — but the file does not say
  // which reader wrote it, so it is not attributed to one.
  it('says a stated page count was counted by the reader', () => {
    const wrapper = mountBook(book({ measured_pages: 3543, measured_source: 'device' }))

    expect(wrapper.text()).toContain('3,543')
    expect(wrapper.text()).toContain('counted by your reader')
  })

  // The book stores the identifier, which is a hex string on a real device. What
  // belongs on screen is whatever its owner decided to call the thing.
  it('uses the name the owner gave the device', () => {
    const wrapper = mountBook(
      book({
        measured_pages: 700,
        measured_device: '865F46C0C0F4401D9A05768B6B0BF3AC',
        measured_source: 'progress',
      }),
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

  // The one question a shelf cannot answer on its own: what is this one about.
  it('shows the blurb read out of the file', () => {
    const wrapper = mountBook(book({ description: 'Der Hexer Geralt.\n\nEin Vorspiel.' }))

    expect(wrapper.text()).toContain('About this book')
    // Two paragraphs, because the server marked them with a blank line.
    const paragraphs = wrapper.findAll('p').map((node) => node.text())
    expect(paragraphs).toContain('Der Hexer Geralt.')
    expect(paragraphs).toContain('Ein Vorspiel.')
  })

  // Most books declare none, and an empty heading over nothing is worse than no
  // heading at all.
  it('leaves the blurb out when the book has none', () => {
    const wrapper = mountBook(book())

    expect(wrapper.text()).not.toContain('About this book')
  })

  // The column holds plain text. A book whose description somehow contains
  // markup shows the markup, rather than running it.
  it('does not render markup that ended up in the blurb', () => {
    const wrapper = mountBook(book({ description: '<img src=x onerror=alert(1)> Der Hexer.' }))

    const paragraph = wrapper.findAll('p').find((node) => node.text().includes('Der Hexer.'))
    expect(paragraph).toBeDefined()
    expect(paragraph!.find('img').exists()).toBe(false)
    expect(paragraph!.html()).toContain('&lt;img')
  })

  // Answering "what is this one about" without opening the book on a reader,
  // where opening it would count as reading.
  it('offers a preview of the book', () => {
    const wrapper = mountBook(book())

    const preview = wrapper.findAll('button').find((node) => node.text().includes('Preview'))
    expect(preview).toBeDefined()
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

  // A book's own page is where somebody decides it belongs on a shelf, so it
  // has to say which shelves it is already on.
  describe('collections', () => {
    beforeEach(() => {
      document.body.innerHTML = ''
    })

    it('names the shelves the book stands on', () => {
      const wrapper = mountBook(
        book(),
        [],
        [device()],
        [shelf('shelf-a', 'Winter reading', ['book-a']), shelf('shelf-b', 'One day', ['book-b'])],
      )

      expect(wrapper.text()).toContain('Winter reading')
      expect(wrapper.text()).not.toContain('One day')
    })

    it('takes the book off a shelf it is on', async () => {
      const wrapper = mountBook(
        book(),
        [],
        [device()],
        [shelf('shelf-a', 'Winter reading', ['book-a'])],
      )

      await wrapper.find('[title="Take off Winter reading"]').trigger('click')

      expect(useCollectionsStore().removeBook).toHaveBeenCalledWith('shelf-a', 'book-a')
    })

    it('offers the shelves it is not on yet', async () => {
      const wrapper = mountBook(
        book(),
        [],
        [device()],
        [shelf('shelf-a', 'Winter reading', ['book-a']), shelf('shelf-b', 'One day', ['book-b'])],
      )

      await wrapper.find('[aria-controls="book-shelves"]').trigger('click')

      expect(document.body.textContent).toContain('One day')
    })
  })
})
