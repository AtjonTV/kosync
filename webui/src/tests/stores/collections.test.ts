//
// File:        webui/src/tests/stores/collections.test.ts
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

import { useCollectionsStore } from '@/stores/collections'
import type { BookCollection } from '@/models'

function shelf(overrides: Partial<BookCollection> = {}): BookCollection {
  return {
    id: 'shelf-a',
    collectionId: 'c',
    collectionName: 'book_collections',
    created: '',
    updated: '',
    owner: 'user-a',
    name: 'Winter reading',
    description: '',
    books: [],
    ...overrides,
  }
}

describe('collections store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    pbMockModule.reset()
  })

  it('asks for the shelves by name', async () => {
    const store = useCollectionsStore()
    await store.load()

    expect(pbMockModule.collections.get('book_collections')?.getFullList).toHaveBeenCalledWith({
      sort: 'name',
    })
  })

  // The create rule compares the owner to the caller, so the server does not
  // fill it in: a shelf made without one would simply be refused.
  it('names the owner when making a shelf', async () => {
    pbMockModule.authStore.record = { id: 'user-a' }

    const store = useCollectionsStore()
    await store.create('Winter reading', 'For the dark half of the year')

    expect(pbMockModule.collections.get('book_collections')?.create).toHaveBeenCalledWith({
      owner: 'user-a',
      name: 'Winter reading',
      description: 'For the dark half of the year',
    })
  })

  it('refuses to make a shelf with nobody signed in', async () => {
    const store = useCollectionsStore()

    await expect(store.create('Winter reading')).rejects.toThrow('not signed in')
    expect(pbMockModule.collections.get('book_collections')).toBeUndefined()
  })

  /**
   * The reason for the modifiers rather than the whole list: two tabs open on
   * the same shelf both add a book, and both additions survive. Sending the
   * list as it was read would make the second write undo the first.
   */
  it('adds and removes a single book rather than rewriting the shelf', async () => {
    const store = useCollectionsStore()

    await store.addBook('shelf-a', 'book-1')
    await store.removeBook('shelf-a', 'book-1')

    const collection = pbMockModule.collections.get('book_collections')
    expect(collection?.update).toHaveBeenNthCalledWith(1, 'shelf-a', { 'books+': 'book-1' })
    expect(collection?.update).toHaveBeenNthCalledWith(2, 'shelf-a', { 'books-': 'book-1' })
  })

  // Rearranging is the one thing that has to send the list: there is no
  // modifier for "third, not fifth".
  it('sends the whole list when the shelf is rearranged', async () => {
    const store = useCollectionsStore()
    await store.reorder('shelf-a', ['book-2', 'book-1'])

    expect(pbMockModule.collections.get('book_collections')?.update).toHaveBeenCalledWith(
      'shelf-a',
      { books: ['book-2', 'book-1'] },
    )
  })

  it('says which shelves a book stands on', async () => {
    const collection = pbMockModule.collection('book_collections')
    collection.getFullList.mockResolvedValue([
      shelf({ id: 'shelf-a', books: ['book-1', 'book-2'] }),
      shelf({ id: 'shelf-b', name: 'One day', books: ['book-2'] }),
    ])

    const store = useCollectionsStore()
    await store.load()

    expect(store.byBook.get('book-1')?.map((one) => one.id)).toEqual(['shelf-a'])
    expect(store.byBook.get('book-2')?.map((one) => one.id)).toEqual(['shelf-a', 'shelf-b'])
    expect(store.byBook.get('book-3')).toBeUndefined()
    expect(store.byId.get('shelf-b')?.name).toBe('One day')
  })

  it('folds live changes into the loaded shelves, in name order', async () => {
    const collection = pbMockModule.collection('book_collections')
    collection.getFullList.mockResolvedValue([shelf()])

    const store = useCollectionsStore()
    await store.load()
    await store.subscribe()

    pbMockModule.emit('book_collections', 'update', shelf({ books: ['book-1'] }))
    expect(store.byId.get('shelf-a')?.books).toEqual(['book-1'])

    pbMockModule.emit('book_collections', 'create', shelf({ id: 'shelf-b', name: 'Autumn' }))
    expect(store.collections.map((one) => one.name)).toEqual(['Autumn', 'Winter reading'])

    pbMockModule.emit('book_collections', 'delete', shelf())
    expect(store.collections.map((one) => one.id)).toEqual(['shelf-b'])
  })

  it('drops everything when it is cleared', async () => {
    const collection = pbMockModule.collection('book_collections')
    collection.getFullList.mockResolvedValue([shelf()])

    const store = useCollectionsStore()
    await store.load()
    store.clear()

    expect(store.collections).toHaveLength(0)
    expect(store.loaded).toBe(false)
  })
})
