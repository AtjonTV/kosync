//
// File:        webui/src/tests/stores/books.test.ts
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

import { beforeEach, describe, expect, it, vi } from 'vitest'
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
    fileUrl: actual.fileUrl,
  }
})

import { useBooksStore } from '@/stores/books'

/** The FormData the store handed to the collection create. */
const uploadedForm = (): FormData => {
  const [call] = pbMockModule.collection('books').create.mock.calls
  if (!call) throw new Error('create was never called')

  const form = call[0]
  if (!(form instanceof FormData)) throw new Error('create was not called with FormData')

  return form
}

/** A stand-in for a chosen EPUB. */
const epubFile = () =>
  new File([new Uint8Array([0x50, 0x4b, 0x03, 0x04])], 'Zeit des Sturms.epub', {
    type: 'application/epub+zip',
  })

describe('books store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    pbMockModule.reset()
    pbMockModule.authStore.record = { id: 'user-1' }
    pbMockModule.authStore.isValid = true
  })

  it('uploads the file as multipart with the owner attached', async () => {
    const store = useBooksStore()
    await store.upload(epubFile())

    expect(pbMockModule.collection('books').create).toHaveBeenCalled()

    const form = uploadedForm()
    // The create rule checks the owner against the caller, so it has to be sent.
    expect(form.get('owner')).toBe('user-1')
    expect(form.get('file')).toBeInstanceOf(File)
  })

  // Everything the server derives from the file would be untrustworthy coming
  // from a browser, so the upload must not send any of it.
  it('never sends derived metadata', async () => {
    const store = useBooksStore()
    await store.upload(epubFile())

    const form = uploadedForm()
    for (const field of [
      'title',
      'authors',
      'cover',
      'word_count',
      'page_count',
      'content_hash',
      'hash_binary',
      'hash_filename',
    ]) {
      expect(form.get(field)).toBeNull()
    }
  })

  it('refuses to upload while signed out', async () => {
    pbMockModule.authStore.record = null
    pbMockModule.authStore.isValid = false

    const store = useBooksStore()
    await expect(store.upload(epubFile())).rejects.toThrow('not signed in')
    expect(pbMockModule.collection('books').create).not.toHaveBeenCalled()
  })

  it('changes only the title when renaming', async () => {
    const store = useBooksStore()
    await store.rename('book-1', 'A better title')

    expect(pbMockModule.collection('books').update).toHaveBeenCalledWith('book-1', {
      title: 'A better title',
    })
  })

  it('reloads after deleting a book', async () => {
    const store = useBooksStore()
    await store.remove('book-1')

    expect(pbMockModule.collection('books').delete).toHaveBeenCalledWith('book-1')
    expect(pbMockModule.collection('books').getFullList).toHaveBeenCalled()
  })

  it('applies realtime events to the loaded library', async () => {
    const store = useBooksStore()
    await store.subscribe()

    pbMockModule.emit('books', 'create', { id: 'b2', title: 'Zeit des Sturms' })
    pbMockModule.emit('books', 'create', { id: 'b1', title: 'Der letzte Wunsch' })
    expect(store.books.map((book) => book.id)).toEqual(['b1', 'b2'])

    pbMockModule.emit('books', 'update', { id: 'b1', title: 'Zzz last' })
    expect(store.books.map((book) => book.title)).toEqual(['Zeit des Sturms', 'Zzz last'])

    pbMockModule.emit('books', 'delete', { id: 'b2' })
    expect(store.books.map((book) => book.id)).toEqual(['b1'])
  })

  it('asks the server how full the library is', async () => {
    pbMockModule.send.mockResolvedValueOnce({ books: 2, used: 4096, quota: 1048576 })

    const store = useBooksStore()
    await store.load()

    expect(pbMockModule.send).toHaveBeenCalledWith('/api/kosync/storage', { method: 'GET' })
    expect(store.usage?.used).toBe(4096)
    expect(store.limited).toBe(true)
  })

  // The quota is the operator's setting, and not having one is the default.
  it('is not limited when the server reports no quota', async () => {
    pbMockModule.send.mockResolvedValueOnce({ books: 2, used: 4096, quota: 0 })

    const store = useBooksStore()
    await store.load()

    expect(store.limited).toBe(false)
  })

  // A library that cannot say how full it is must still show its books.
  it('keeps the books when the usage cannot be fetched', async () => {
    pbMockModule.collection('books').getFullList.mockResolvedValueOnce([{ id: 'b1', title: 'One' }])
    pbMockModule.send.mockRejectedValueOnce(new Error('nope'))

    const store = useBooksStore()
    await store.load()

    expect(store.books).toHaveLength(1)
    expect(store.usage).toBeNull()
  })

  it('drops the library and its subscription on clear', async () => {
    const store = useBooksStore()
    await store.subscribe()
    pbMockModule.emit('books', 'create', { id: 'b1', title: 'Kreuzweg der Raben' })

    store.clear()

    expect(store.books).toHaveLength(0)
    expect(store.loaded).toBe(false)
  })
})
