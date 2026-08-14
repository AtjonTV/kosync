//
// File:        webui/src/tests/components/BookLibrary.test.ts
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { createTestingPinia } from '@pinia/testing'
import PrimeVue from 'primevue/config'
import ToastService from 'primevue/toastservice'
import ConfirmationService from 'primevue/confirmationservice'
import BookLibrary from '@/components/BookLibrary.vue'
import type { Book, DocumentRecord } from '@/models'

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

function book(id: string, overrides: Partial<Book> = {}): Book {
  return {
    id,
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
    page_count: 700,
    word_count: 109288,
    content_hash: id,
    hash_binary: 'bin' + id,
    hash_filename: '',
    measured_pages: 0,
    measured_device: '',
    measured_through: '',
    ...overrides,
  }
}

function read(bookId: string, at: string): DocumentRecord {
  return {
    id: 'doc-' + bookId,
    collectionId: 'c',
    collectionName: 'documents',
    created: '',
    updated: '',
    owner: 'user-a',
    document: 'hash-' + bookId,
    title: 'x',
    current_location: '',
    progress: 0.5,
    last_device: 'go7',
    last_device_id: 'go7',
    last_read_at: at,
    source_account: '',
    book: bookId,
  }
}

function mountLibrary(
  books: Book[],
  props: Record<string, unknown> = {},
  documents: DocumentRecord[] = [],
) {
  return mount(BookLibrary, {
    props,
    global: {
      plugins: [
        createTestingPinia({
          createSpy: vi.fn,
          initialState: {
            books: { books, loaded: true },
            documents: { documents, loaded: true },
          },
        }),
        PrimeVue,
        ToastService,
        ConfirmationService,
      ],
      stubs: { RouterLink: { template: '<a><slot /></a>' } },
    },
  })
}

describe('BookLibrary', () => {
  // The Witcher omnibus has a title six lines long, which stretched the whole
  // grid row it sat in and left the shorter cards floating above a gap. Every
  // card now reserves the same two lines whether it fills them or not.
  it('reserves the same title space for every book', () => {
    const wrapper = mountLibrary([
      book('a', { title: 'Der letzte Wunsch' }),
      book('b', {
        title:
          'Die Witcher-Saga - Das Erbe der Elfen Die Zeit der Verachtung Feuertaufe ' +
          'Der Schwalbenturm Die Dame vom See',
      }),
    ])

    const titles = wrapper.findAll('.line-clamp-2')
    expect(titles).toHaveLength(2)
    for (const title of titles) {
      expect(title.classes()).toContain('min-h-[2.5em]')
    }
  })

  // A book whose metadata names no author still has to occupy an author's worth
  // of space, or it comes out shorter than the one beside it.
  it('keeps the author line even when there is no author', () => {
    const wrapper = mountLibrary([book('a', { authors: [] })])

    expect(wrapper.find('.line-clamp-1').exists()).toBe(true)
  })

  it('shows every book by title when it is not limited', () => {
    const wrapper = mountLibrary([book('a', { title: 'Bravo' }), book('b', { title: 'Alpha' })])
    const text = wrapper.text()

    expect(text.indexOf('Alpha')).toBeLessThan(text.indexOf('Bravo'))
  })

  // The dashboard is a shelf, not a catalogue: the most recently read come
  // first, and the rest are one link away.
  it('shows the most recently read first when it is limited', () => {
    const wrapper = mountLibrary(
      [book('a', { title: 'Alpha' }), book('b', { title: 'Bravo' }), book('c', { title: 'Delta' })],
      { limit: 2 },
      [read('b', '2026-03-05 10:00:00.000Z'), read('c', '2026-03-01 10:00:00.000Z')],
    )
    const text = wrapper.text()

    expect(text).toContain('Bravo')
    expect(text).not.toContain('Alpha')
    expect(text).toContain('See all 3 books')
  })

  it('prints no heading of its own when the page already has one', () => {
    const wrapper = mountLibrary([book('a')], { heading: '' })

    expect(wrapper.text()).not.toContain('Library')
  })
})
