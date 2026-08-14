//
// File:        webui/src/tests/components/HomeView.test.ts
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

import { describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createTestingPinia } from '@pinia/testing'
import PrimeVue from 'primevue/config'
import HomeView from '@/views/HomeView.vue'
import DocumentsView from '@/views/DocumentsView.vue'
import { useAuthStore } from '@/stores/auth'
import { useBooksStore } from '@/stores/books'
import { useDocumentsStore } from '@/stores/documents'
import type { DocumentRecord } from '@/models'

vi.mock('@/pb', async () => {
  const mock = await import('../mocks/pb')
  const actual = await vi.importActual<typeof import('@/pb')>('@/pb')

  return {
    pb: mock.pbMock,
    Collections: actual.Collections,
    KosyncApi: actual.KosyncApi,
    errorMessage: actual.errorMessage,
    fileUrl: actual.fileUrl,
  }
})

function record(overrides: Partial<DocumentRecord> = {}): DocumentRecord {
  return {
    id: 'doc-a',
    collectionId: 'c',
    collectionName: 'documents',
    created: '',
    updated: '',
    owner: 'user-a',
    document: 'hash-a',
    title: 'Zeit des Sturms',
    current_location: '',
    progress: 0.4,
    last_device: 'go7',
    last_device_id: 'go7',
    last_read_at: '2026-03-01 10:00:00.000Z',
    source_account: '',
    book: 'book-a',
    ...overrides,
  }
}

// The dashboard only loads anything once the stored session has been confirmed,
// so the pinia has to exist before the component does.
async function mountHome() {
  const pinia = createTestingPinia({
    createSpy: vi.fn,
    initialState: { auth: { isValid: true } },
  })
  const auth = useAuthStore(pinia)
  vi.mocked(auth.refresh).mockResolvedValue(true)

  const wrapper = mount(HomeView, {
    global: {
      plugins: [pinia, PrimeVue],
      stubs: {
        DashboardMetrics: true,
        ReadStatisticsChart: true,
        BookLibrary: true,
        AuthPanel: true,
        SetupGuide: true,
      },
    },
  })

  await flushPromises()

  return wrapper
}

describe('HomeView', () => {
  // The library is the main component on the dashboard now, and it needs both:
  // the books for the covers and the documents for the progress on them.
  it('loads the books and the documents behind the covers', async () => {
    await mountHome()

    expect(useBooksStore().load).toHaveBeenCalled()
    expect(useDocumentsStore().load).toHaveBeenCalled()
  })

  it('releases the book subscription when it goes away', async () => {
    const wrapper = await mountHome()
    wrapper.unmount()

    expect(useBooksStore().unsubscribe).toHaveBeenCalled()
  })
})

describe('DocumentsView', () => {
  function mountDocuments(documents: DocumentRecord[]) {
    return mount(DocumentsView, {
      global: {
        plugins: [
          createTestingPinia({
            createSpy: vi.fn,
            initialState: { documents: { documents, loaded: true } },
          }),
          PrimeVue,
        ],
        stubs: { DocumentsList: true },
      },
    })
  }

  // A document with no book gets no cover, no page count and no statistics, and
  // the only way to change that is to upload the file. Saying how many there are
  // is what phase 9 left for this page.
  it('says how many documents have no book behind them', () => {
    const wrapper = mountDocuments([record(), record({ id: 'doc-b', book: '' })])

    expect(wrapper.text()).toContain('1 document has')
  })

  it('says nothing when every document is matched', () => {
    const wrapper = mountDocuments([record()])

    expect(wrapper.text()).not.toContain('no book on the server')
  })
})
