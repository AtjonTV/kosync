//
// File:        webui/src/tests/components/DocumentsView.test.ts
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { createTestingPinia } from '@pinia/testing'
import PrimeVue from 'primevue/config'
import ToastService from 'primevue/toastservice'
import ConfirmationService from 'primevue/confirmationservice'
import DocumentsView from '@/views/DocumentsView.vue'
import DocumentsList from '@/components/DocumentsList.vue'
import type { Book, DocumentWithHistory } from '@/models'

vi.mock('@/pb', async () => {
  const mock = await import('../mocks/pb')
  const actual = await vi.importActual<typeof import('@/pb')>('@/pb')

  return {
    pb: mock.pbMock,
    Collections: actual.Collections,
    KosyncApi: actual.KosyncApi,
    errorMessage: actual.errorMessage,
    fileUrl: () => 'blob:cover',
  }
})

function record(overrides: Partial<DocumentWithHistory> = {}): DocumentWithHistory {
  return {
    id: 'doc-a',
    collectionId: 'c',
    collectionName: 'documents',
    created: '',
    updated: '',
    owner: 'user-a',
    document: '043f11771ef9d191364ac0ba08198d36',
    title: 'Zeit des Sturms',
    current_location: '',
    progress: 0.4,
    last_device: 'go7',
    last_device_id: 'go7',
    last_read_at: '2026-03-01 10:00:00.000Z',
    source_account: '',
    book: 'book-a',
    history: [],
    ...overrides,
  }
}

function book(): Book {
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
    authors: [],
    language: 'de',
    identifiers: {},
    page_count: 700,
    word_count: 109288,
    content_hash: 'abc',
    hash_binary: 'bin',
    hash_filename: 'name',
    measured_pages: 700,
    measured_device: 'go7',
    measured_through: '',
  }
}

const plugins = () => [PrimeVue, ToastService, ConfirmationService]

function mountView(documents: DocumentWithHistory[]) {
  return mount(DocumentsView, {
    global: {
      plugins: [
        createTestingPinia({
          createSpy: vi.fn,
          initialState: {
            documents: { documents, loaded: true },
            books: { books: [book()], loaded: true },
          },
        }),
        ...plugins(),
      ],
      stubs: { DocumentsList: true, RouterLink: { template: '<a><slot /></a>' } },
    },
  })
}

describe('DocumentsView', () => {
  // The page exists to answer one question: what have I read that is not on the
  // server? So the answer leads, with the count in the heading.
  it('leads with the documents that have no book', () => {
    const wrapper = mountView([record(), record({ id: 'doc-b', book: '' })])
    const text = wrapper.text()

    expect(text).toContain('Not in your library')
    expect(text.indexOf('Not in your library')).toBeLessThan(text.indexOf('In your library'))
  })

  it('counts each group separately', () => {
    const wrapper = mountView([
      record(),
      record({ id: 'doc-b', book: '' }),
      record({ id: 'doc-c', book: '' }),
    ])

    expect(wrapper.text()).toContain('Not in your library (2)')
    expect(wrapper.text()).toContain('In your library (1)')
  })

  // Nothing missing is worth saying out loud, rather than leaving an absence to
  // be interpreted.
  it('says so when nothing is missing', () => {
    const wrapper = mountView([record()])

    expect(wrapper.text()).toContain('Every document has its book in your library')
    expect(wrapper.text()).not.toContain('Not in your library')
  })

  // Uploading is the only fix, so it is offered where the problem is stated
  // rather than only on the library page.
  it('offers an upload beside the missing ones', () => {
    const wrapper = mountView([record({ book: '' })])

    expect(wrapper.findComponent({ name: 'FileUpload' }).exists()).toBe(true)
  })
})

describe('DocumentsList', () => {
  function mountList(documents: DocumentWithHistory[], viewMode: string) {
    return mount(DocumentsList, {
      props: { documents, viewMode },
      global: {
        plugins: [
          createTestingPinia({
            createSpy: vi.fn,
            initialState: { books: { books: [book()], loaded: true }, devices: { devices: [] } },
          }),
          ...plugins(),
        ],
        // The destination is an object, so it is serialised into the markup for
        // the assertion to find the book it points at.
        stubs: {
          RouterLink: { template: '<a :href="JSON.stringify(to)"><slot /></a>', props: ['to'] },
        },
      },
    })
  }

  // The bug this replaced: the marker was added to the table and the grid is the
  // default, so the one view most people see said nothing at all. Both are
  // asserted from now on.
  for (const viewMode of ['Grid', 'List']) {
    it(`marks a document with no book in the ${viewMode.toLowerCase()} view`, () => {
      const wrapper = mountList([record({ book: '' })], viewMode)

      expect(wrapper.text()).toContain('Not in library')
    })

    it(`does not mark a matched document in the ${viewMode.toLowerCase()} view`, () => {
      const wrapper = mountList([record()], viewMode)

      expect(wrapper.text()).not.toContain('Not in library')
    })

    it(`links a matched document to its book in the ${viewMode.toLowerCase()} view`, () => {
      const wrapper = mountList([record()], viewMode)

      expect(wrapper.html()).toContain('book-a')
    })
  }
})
