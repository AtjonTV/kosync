//
// File:        webui/src/tests/components/CollectionView.test.ts
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

import { beforeEach, describe, expect, it, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createTestingPinia } from '@pinia/testing'
import PrimeVue from 'primevue/config'
import ToastService from 'primevue/toastservice'
import ConfirmationService from 'primevue/confirmationservice'
import CollectionView from '@/views/CollectionView.vue'
import { useCollectionsStore } from '@/stores/collections'
import type { Book, BookCollection } from '@/models'

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
  useRoute: () => ({ params: { id: 'shelf-a' } }),
}))

function book(id: string, title: string): Book {
  return {
    id,
    collectionId: 'c',
    collectionName: 'books',
    created: '',
    updated: '',
    owner: 'user-a',
    file: 'book.epub',
    cover: 'cover.jpg',
    title,
    authors: ['Lee Child'],
    language: 'en',
    identifiers: {},
    series: '',
    series_index: 0,
    subjects: null,
    page_count: 300,
    word_count: 90000,
    file_size: 1_000_000,
    content_hash: id,
    hash_binary: 'bin' + id,
    hash_filename: '',
    measured_pages: 0,
    measured_device: '',
    measured_through: '',
  }
}

// Deliberately neither alphabetical nor the order the books were uploaded in:
// the order a shelf is in is the one thing about it no query could work out.
const library = [
  book('book-a', 'Ambush'),
  book('book-b', 'Betrayal'),
  book('book-c', 'Choice'),
  book('book-d', 'Unshelved'),
]

function shelf(overrides: Partial<BookCollection> = {}): BookCollection {
  return {
    id: 'shelf-a',
    collectionId: 'c',
    collectionName: 'book_collections',
    created: '',
    updated: '',
    owner: 'user-a',
    name: 'Winter reading',
    description: 'For the dark half of the year',
    books: ['book-c', 'book-a', 'book-b'],
    ...overrides,
  }
}

function mountShelf(collections: BookCollection[] = [shelf()]) {
  const wrapper = mount(CollectionView, {
    attachTo: document.body,
    global: {
      plugins: [
        createTestingPinia({
          createSpy: vi.fn,
          initialState: {
            collections: { collections, loaded: true },
            books: { books: library, loaded: true },
            documents: { documents: [], loaded: true },
          },
        }),
        PrimeVue,
        ToastService,
        ConfirmationService,
      ],
      stubs: { RouterLink: { template: '<a><slot /></a>' } },
    },
  })

  return { wrapper, store: useCollectionsStore() }
}

/** A button anywhere on the page, including inside a teleported dialog. */
function button(label: string): HTMLButtonElement | undefined {
  return Array.from(document.body.querySelectorAll('button')).find((candidate) =>
    candidate.textContent?.includes(label),
  )
}

describe('CollectionView', () => {
  beforeEach(() => {
    document.body.innerHTML = ''
  })

  it('shows the shelf in the order it was built', () => {
    const { wrapper } = mountShelf()
    const text = wrapper.text()

    expect(text).toContain('Winter reading')
    expect(text).toContain('For the dark half of the year')
    expect(text.indexOf('Choice')).toBeLessThan(text.indexOf('Ambush'))
    expect(text.indexOf('Ambush')).toBeLessThan(text.indexOf('Betrayal'))
    expect(text).not.toContain('Unshelved')
  })

  it('says so when the shelf is not there', () => {
    const { wrapper } = mountShelf([])

    expect(wrapper.text()).toContain('That collection is not there')
  })

  it('says what an empty shelf is missing', () => {
    const { wrapper } = mountShelf([shelf({ books: [] })])

    expect(wrapper.text()).toContain('Nothing on this shelf yet')
  })

  it('takes a book off the shelf without deleting it', async () => {
    const { wrapper, store } = mountShelf()

    await wrapper.find('[title="Take off this collection"]').trigger('click')

    expect(store.removeBook).toHaveBeenCalledWith('shelf-a', 'book-c')
  })

  // Moving a book is the one change that has to send the whole list, and the
  // list it sends is the stored one with a single book moved.
  it('moves a book one place along the shelf', async () => {
    const { wrapper, store } = mountShelf()

    const later = wrapper.findAll('[title="Move later"]')
    await later[0]?.trigger('click')

    expect(store.reorder).toHaveBeenCalledWith('shelf-a', ['book-a', 'book-c', 'book-b'])
  })

  it('cannot move the first book earlier or the last one later', () => {
    const { wrapper } = mountShelf()

    const earlier = wrapper.findAll('[title="Move earlier"]')
    const later = wrapper.findAll('[title="Move later"]')

    expect(earlier[0]?.attributes('disabled')).toBeDefined()
    expect(earlier[1]?.attributes('disabled')).toBeUndefined()
    expect(later[2]?.attributes('disabled')).toBeDefined()
  })

  it('offers only the books that are not on the shelf already', async () => {
    mountShelf()

    button('Add books')?.click()
    await flushPromises()

    const options = Array.from(document.body.querySelectorAll('[role="option"]')).map((option) =>
      option.textContent?.trim(),
    )

    expect(options).toHaveLength(1)
    expect(options[0]).toContain('Unshelved')
  })

  it('adds a picked book to the end of the shelf', async () => {
    const { store } = mountShelf()

    button('Add books')?.click()
    await flushPromises()

    document.body.querySelector<HTMLElement>('[role="option"]')?.click()
    await flushPromises()

    button('Add 1')?.click()
    await flushPromises()

    expect(store.addBook).toHaveBeenCalledWith('shelf-a', 'book-d')
  })
})
