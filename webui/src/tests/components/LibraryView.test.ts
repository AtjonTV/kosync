//
// File:        webui/src/tests/components/LibraryView.test.ts
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { createTestingPinia } from '@pinia/testing'
import PrimeVue from 'primevue/config'
import LibraryView from '@/views/LibraryView.vue'
import { useBooksStore } from '@/stores/books'
import { useDocumentsStore } from '@/stores/documents'

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

function mountLibrary(stubLibrary = true) {
  return mount(LibraryView, {
    global: {
      plugins: [createTestingPinia({ createSpy: vi.fn }), PrimeVue],
      stubs: stubLibrary ? { BookLibrary: true } : {},
    },
  })
}

describe('LibraryView', () => {
  // The progress shown on a cover lives on the document, not the book, and the
  // server moves it: uploading a book links the documents that were already
  // recording progress through it. Subscribing to books alone means that link
  // only shows up after a full page reload.
  it('subscribes to documents as well as books', () => {
    mountLibrary()

    expect(useBooksStore().subscribe).toHaveBeenCalled()
    expect(useDocumentsStore().subscribe).toHaveBeenCalled()
  })

  it('releases both subscriptions when it goes away', () => {
    const wrapper = mountLibrary()
    wrapper.unmount()

    expect(useBooksStore().unsubscribe).toHaveBeenCalled()
    expect(useDocumentsStore().unsubscribe).toHaveBeenCalled()
  })

  // The page has its own heading and book count, so the card underneath must not
  // print the word "Library" a second time above the same grid.
  it('does not repeat its heading on the card', () => {
    const wrapper = mountLibrary()

    expect(wrapper.findComponent({ name: 'BookLibrary' }).attributes('heading')).toBe('')
  })
})
