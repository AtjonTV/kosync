//
// File:        webui/src/tests/lib/browsing.test.ts
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

import { describe, expect, it } from 'vitest'
import { bookOrder, searchBooks, type Reading } from '@/lib/browsing'
import type { Book } from '@/models'

function book(id: string, overrides: Partial<Book> = {}): Book {
  return {
    id,
    collectionId: 'c',
    collectionName: 'books',
    created: '2026-01-01 00:00:00.000Z',
    updated: '',
    owner: 'user-a',
    file: 'book.epub',
    cover: '',
    title: 'A Book',
    authors: [],
    language: 'en',
    identifiers: {},
    description: '',
    series: '',
    series_index: 0,
    subjects: null,
    page_count: 100,
    word_count: 10000,
    file_size: 1000,
    content_hash: id,
    hash_binary: 'bin' + id,
    hash_filename: '',
    measured_pages: 0,
    measured_device: '',
    measured_through: '',
    measured_source: '',
    ...overrides,
  }
}

/** The ids a search or a sort came back with, in order. */
const ids = (books: Book[]) => books.map((one) => one.id)

const nothingRead: Reading = { lastRead: new Map(), progress: new Map() }

describe('searchBooks', () => {
  const library = [
    book('a', { title: 'Killing Floor', authors: ['Child, Lee'], series: 'Jack Reacher' }),
    book('b', {
      title: 'Die Zeit der Verachtung',
      authors: ['Andrzej Sapkowski'],
      series: 'Hexer',
    }),
    book('c', { title: 'Der Schwalbenturm', authors: ['Andrzej Sapkowski'], series: 'Hexer' }),
    book('d', { title: 'Die Ethik', authors: ['Rainer Schäfer'] }),
  ]

  it('gives back the whole library when nothing is being looked for', () => {
    expect(searchBooks(library, '')).toHaveLength(4)
    expect(searchBooks(library, '   ')).toHaveLength(4)
  })

  it('finds a book by its title', () => {
    expect(ids(searchBooks(library, 'schwalbenturm'))).toEqual(['c'])
  })

  it('finds a book by its series', () => {
    expect(ids(searchBooks(library, 'hexer'))).toEqual(['b', 'c'])
  })

  it('finds a book by its author', () => {
    expect(ids(searchBooks(library, 'sapkowski'))).toEqual(['b', 'c'])
  })

  it('pays no attention to case', () => {
    expect(ids(searchBooks(library, 'SAPKOWSKI'))).toEqual(['b', 'c'])
  })

  // The shelves say "Lee Child" and the file says "Child, Lee". Somebody
  // searching for what is on the screen has to find it, and so does somebody
  // searching for what is in the book.
  it('finds an author under either spelling of their name', () => {
    expect(ids(searchBooks(library, 'lee child'))).toEqual(['a'])
    expect(ids(searchBooks(library, 'child, lee'))).toEqual(['a'])
  })

  // A library is typed into from a keyboard that may not have the letter the
  // publisher used.
  it('finds an accented name typed without the accent', () => {
    expect(ids(searchBooks(library, 'schafer'))).toEqual(['d'])
    expect(ids(searchBooks(library, 'schäfer'))).toEqual(['d'])
  })

  // No single substring of "child killing" occurs in the book, but both words do.
  it('takes the words separately, and in any order', () => {
    expect(ids(searchBooks(library, 'child killing'))).toEqual(['a'])
    expect(ids(searchBooks(library, 'killing child'))).toEqual(['a'])
  })

  it('wants every word, not just one of them', () => {
    expect(searchBooks(library, 'sapkowski reacher')).toHaveLength(0)
  })

  it('comes back with nothing when nothing matches', () => {
    expect(searchBooks(library, 'tolkien')).toHaveLength(0)
  })

  it('does not trip over a book whose metadata names no author', () => {
    expect(searchBooks([book('e', { authors: [] })], 'anything')).toHaveLength(0)
  })
})

describe('bookOrder', () => {
  it('sorts by title when that is what was asked for', () => {
    const shelf = [book('a', { title: 'Bravo' }), book('b', { title: 'Alpha' })]

    expect(ids([...shelf].sort(bookOrder('title', nothingRead)))).toEqual(['b', 'a'])
  })

  it('puts the most recently added first', () => {
    const shelf = [
      book('old', { created: '2026-01-01 00:00:00.000Z' }),
      book('new', { created: '2026-06-01 00:00:00.000Z' }),
    ]

    expect(ids([...shelf].sort(bookOrder('added', nothingRead)))).toEqual(['new', 'old'])
  })

  it('puts the most recently read first', () => {
    const shelf = [book('a'), book('b'), book('c')]
    const reading: Reading = {
      lastRead: new Map([
        ['a', '2026-03-01 10:00:00.000Z'],
        ['b', '2026-05-01 10:00:00.000Z'],
        ['c', '2026-04-01 10:00:00.000Z'],
      ]),
      progress: new Map(),
    }

    expect(ids([...shelf].sort(bookOrder('last-read', reading)))).toEqual(['b', 'c', 'a'])
  })

  // A book nothing has read has no date to sort on, and belongs at the end of a
  // list about when things were read rather than at the top of it.
  it('leaves the unread at the end of the reading orders', () => {
    const shelf = [book('never', { title: 'Alpha' }), book('read', { title: 'Bravo' })]
    const reading: Reading = {
      lastRead: new Map([['read', '2026-05-01 10:00:00.000Z']]),
      progress: new Map([['read', 0.4]]),
    }

    expect(ids([...shelf].sort(bookOrder('last-read', reading)))).toEqual(['read', 'never'])
    expect(ids([...shelf].sort(bookOrder('progress', reading)))).toEqual(['read', 'never'])
  })

  it('puts the furthest read first', () => {
    const shelf = [book('a'), book('b'), book('c')]
    const reading: Reading = {
      lastRead: new Map(),
      progress: new Map([
        ['a', 0.2],
        ['b', 1],
        ['c', 0.6],
      ]),
    }

    expect(ids([...shelf].sort(bookOrder('progress', reading)))).toEqual(['b', 'c', 'a'])
  })

  // Two books a sort cannot tell apart must not come out in whatever order the
  // server happened to answer with.
  it('settles every tie on the title', () => {
    const shelf = [book('a', { title: 'Bravo' }), book('b', { title: 'Alpha' })]

    for (const by of ['added', 'last-read', 'progress'] as const) {
      expect(ids([...shelf].sort(bookOrder(by, nothingRead)))).toEqual(['b', 'a'])
    }
  })
})
