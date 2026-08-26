//
// File:        webui/src/tests/components/CollectionsView.test.ts
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

import { beforeEach, describe, expect, it, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createTestingPinia } from '@pinia/testing'
import PrimeVue from 'primevue/config'
import ToastService from 'primevue/toastservice'
import ConfirmationService from 'primevue/confirmationservice'
import CollectionsView from '@/views/CollectionsView.vue'
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
    description: '',
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
    measured_source: '',
  }
}

function shelf(id: string, name: string, books: string[] = []): BookCollection {
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

const winter = shelf('shelf-a', 'Winter reading', ['book-a', 'book-b'])
const someday = shelf('shelf-b', 'One day')

function mountView(collections: BookCollection[] = [winter, someday]) {
  const wrapper = mount(CollectionsView, {
    attachTo: document.body,
    global: {
      plugins: [
        createTestingPinia({
          createSpy: vi.fn,
          initialState: {
            collections: { collections, loaded: true },
            books: { books: [book('book-a', 'Ambush'), book('book-b', 'Betrayal')], loaded: true },
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

describe('CollectionsView', () => {
  beforeEach(() => {
    document.body.innerHTML = ''
  })

  it('lists the shelves with how much is on each', () => {
    const { wrapper } = mountView()
    const text = wrapper.text()

    expect(text).toContain('Winter reading')
    expect(text).toContain('2 books')
    expect(text).toContain('One day')
    expect(text).toContain('Nothing on this shelf yet')
  })

  // An empty shelf is a plan rather than a mistake, so it is listed like any
  // other; a library with no shelves at all is what needs explaining.
  it('says what a collection is for when there are none', () => {
    const { wrapper } = mountView([])

    expect(wrapper.text()).toContain('No collections yet')
  })

  it('makes a shelf from the name that was typed', async () => {
    const { store } = mountView()

    button('New collection')?.click()
    await flushPromises()

    const name = document.body.querySelector<HTMLInputElement>('#collection-name')
    if (!name) throw new Error('the name field is not there')
    name.value = '  Winter reading  '
    name.dispatchEvent(new Event('input'))
    await flushPromises()

    button('Create')?.click()
    await flushPromises()

    expect(store.create).toHaveBeenCalledWith('Winter reading', '')
  })

  // Saving an unnamed shelf would only be refused by the server, and a message
  // from three layers away is a worse answer than the one it can give here.
  it('refuses to make a shelf with no name', async () => {
    const { store } = mountView()

    button('New collection')?.click()
    await flushPromises()

    button('Create')?.click()
    await flushPromises()

    expect(store.create).not.toHaveBeenCalled()
    expect(document.body.textContent).toContain('A collection needs a name')
  })

  it('renames a shelf without touching what is on it', async () => {
    const { store } = mountView()

    document.body.querySelector<HTMLButtonElement>('[title="Rename"]')?.click()
    await flushPromises()

    const name = document.body.querySelector<HTMLInputElement>('#collection-name')
    if (!name) throw new Error('the name field is not there')
    expect(name.value).toBe('Winter reading')

    name.value = 'Winter'
    name.dispatchEvent(new Event('input'))
    await flushPromises()

    button('Save')?.click()
    await flushPromises()

    expect(store.update).toHaveBeenCalledWith('shelf-a', { name: 'Winter', description: '' })
  })

  it('follows the shelves as they change and lets go on the way out', async () => {
    const { wrapper, store } = mountView()
    await flushPromises()

    expect(store.subscribe).toHaveBeenCalled()

    wrapper.unmount()
    expect(store.unsubscribe).toHaveBeenCalled()
  })
})
