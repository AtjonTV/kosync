//
// File:        webui/src/tests/components/MergeDialog.test.ts
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

import { beforeEach, describe, expect, it, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createTestingPinia } from '@pinia/testing'
import PrimeVue from 'primevue/config'
import ToastService from 'primevue/toastservice'
import MergeDialog from '@/components/MergeDialog.vue'
import { useDocumentsStore } from '@/stores/documents'
import type { DocumentWithHistory } from '@/models'

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

function record(id: string, overrides: Partial<DocumentWithHistory> = {}): DocumentWithHistory {
  return {
    id,
    collectionId: 'c',
    collectionName: 'documents',
    created: '',
    updated: '',
    owner: 'user-a',
    document: 'hash-' + id,
    title: '',
    current_location: '',
    progress: 0.1,
    last_device: 'els-n39',
    last_device_id: 'els',
    last_read_at: '2026-03-01 10:00:00.000Z',
    source_account: '',
    book: '',
    history: [],
    ...overrides,
  }
}

// The split this exists for: one half matched to an uploaded book, the other
// half not, and the same reading in both.
const kept = record('doc-kept', { title: 'Metro - Die Trilogie', book: 'book-a' })
const other = record('doc-other', {
  title: 'Metro Trilogie (2033,2034,2035)',
  last_read_at: '2026-02-20 10:00:00.000Z',
})

/**
 * Mounts the dialog and returns the store, so a test can assert what it asked
 * for.
 *
 * The dialog teleports into the body, which is where everything below looks for
 * it rather than in the wrapper.
 */
async function mountDialog(documents: DocumentWithHistory[] = [kept, other]) {
  mount(MergeDialog, {
    props: { document: kept, visible: true },
    attachTo: document.body,
    global: {
      plugins: [
        createTestingPinia({ createSpy: vi.fn, initialState: { documents: { documents } } }),
        PrimeVue,
        ToastService,
      ],
    },
  })
  await flushPromises()

  return useDocumentsStore()
}

/** The dialog's button with the given label. */
function button(label: string): HTMLButtonElement | undefined {
  return Array.from(document.body.querySelectorAll('button')).find((candidate) =>
    candidate.textContent?.includes(label),
  )
}

describe('MergeDialog', () => {
  beforeEach(() => {
    document.body.innerHTML = ''
  })

  it('offers every other document, matched or not', async () => {
    await mountDialog()

    expect(document.body.textContent).toContain('Metro Trilogie (2033,2034,2035)')
    expect(document.body.textContent).toContain('Not in library')
  })

  it('names the document that is kept and does not offer it', async () => {
    await mountDialog()

    expect(document.body.textContent).toContain('Merge into this document')
    expect(document.body.querySelectorAll('input[type="checkbox"]')).toHaveLength(1)
  })

  it('says so when there is nothing to merge with', async () => {
    await mountDialog([kept])

    expect(document.body.textContent).toContain('There is nothing else to merge')
  })

  it('cannot merge until something is picked', async () => {
    await mountDialog()

    expect(button('Merge')?.disabled).toBe(true)
  })

  it('folds the picked documents into the one it was opened from', async () => {
    const store = await mountDialog()

    document.body.querySelector<HTMLInputElement>('input[type="checkbox"]')?.click()
    await flushPromises()

    button('Merge')?.click()
    await flushPromises()

    expect(store.merge).toHaveBeenCalledWith('doc-kept', ['doc-other'])
  })
})
