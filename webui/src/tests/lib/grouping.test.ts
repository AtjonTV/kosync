//
// File:        webui/src/tests/lib/grouping.test.ts
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

import { describe, expect, it } from 'vitest'
import { authorKey, authorName, groupBooks, languageName } from '@/lib/grouping'
import type { Book } from '@/models'

/**
 * The cases the server's own author folding is held to.
 *
 * The same file `server/internal/books/authors_test.go` reads, rather than a copy
 * of it: the rule is written twice, once in Go for the OPDS catalog and once here
 * for the library page, and a corpus each would let the two drift apart quietly.
 * Which is the failure this whole feature exists to prevent.
 */
import shared from '@testdata/author-names.json'

function book(id: string, overrides: Partial<Book> = {}): Book {
  return {
    id,
    collectionId: 'c',
    collectionName: 'books',
    created: '',
    updated: '',
    owner: 'user-a',
    file: 'book.epub',
    cover: '',
    title: 'A Book',
    authors: [],
    language: 'en',
    identifiers: {},
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
    ...overrides,
  }
}

const titlesOf = (books: Book[]) => books.map((one) => one.title)

describe('the author folding, against the shared cases', () => {
  it('shows every name the way the catalog shows it', () => {
    for (const [written, expected] of Object.entries(shared.display)) {
      expect(authorName(written), `AuthorName(${JSON.stringify(written)})`).toBe(expected)
    }
  })

  it('gives every spelling of one author the same key', () => {
    for (const spellings of shared.sameAuthor) {
      const key = authorKey(spellings[0]!)
      expect(key, `AuthorKey(${JSON.stringify(spellings[0])})`).not.toBe('')

      for (const spelling of spellings.slice(1)) {
        expect(authorKey(spelling), `AuthorKey(${JSON.stringify(spelling)})`).toBe(key)
      }
    }
  })

  it('keeps two authors apart', () => {
    for (const [one, other] of shared.differentAuthors) {
      expect(authorKey(one!), `against ${JSON.stringify(other)}`).not.toBe(authorKey(other!))
    }
  })

  it('gives a name with no letters in it no key at all', () => {
    for (const name of shared.noKey) {
      expect(authorKey(name), JSON.stringify(name)).toBe('')
    }
  })
})

describe('groupBooks', () => {
  it('leaves the library in one nameless shelf when nothing is chosen', () => {
    const shelves = groupBooks([book('a'), book('b')], 'none')

    expect(shelves).toHaveLength(1)
    expect(shelves[0]!.title).toBe('')
    expect(shelves[0]!.books).toHaveLength(2)
  })

  it('has no shelves at all for an empty library', () => {
    expect(groupBooks([], 'none')).toEqual([])
    expect(groupBooks([], 'authors')).toEqual([])
  })

  // The whole point of the exercise: the same author spelled three ways is one
  // shelf, headed with the spelling the library uses most.
  it('puts every spelling of an author on one shelf', () => {
    const shelves = groupBooks(
      [
        book('a', { title: 'Ambush', authors: ['Lee Child'] }),
        book('b', { title: 'Betrayal', authors: ['Lee Child'] }),
        book('c', { title: 'Choice', authors: ['Child, Lee'] }),
      ],
      'authors',
    )

    expect(shelves).toHaveLength(1)
    expect(shelves[0]!.title).toBe('Lee Child')
    expect(titlesOf(shelves[0]!.books)).toEqual(['Ambush', 'Betrayal', 'Choice'])
  })

  // Lincoln Child is not Lee Child, and a fold that cannot tell them apart is
  // worse than no fold at all.
  it('does not fold two authors who merely share a name', () => {
    const shelves = groupBooks(
      [
        book('a', { title: 'Ambush', authors: ['Lee Child'] }),
        book('b', { title: 'FaceOff', authors: ['Lincoln Child'] }),
      ],
      'authors',
    )

    expect(shelves.map((shelf) => shelf.title)).toEqual(['Lee Child', 'Lincoln Child'])
  })

  it('stands a book with two authors on both their shelves', () => {
    const shelves = groupBooks(
      [book('a', { title: 'Der letzte Wunsch', authors: ['Andrzej Sapkowski', 'Erik Simon'] })],
      'authors',
    )

    expect(shelves.map((shelf) => shelf.title)).toEqual(['Andrzej Sapkowski', 'Erik Simon'])
    expect(shelves.every((shelf) => shelf.books.length === 1)).toBe(true)
  })

  // A book that names the same person twice, once each way round, is still one
  // book on their shelf.
  it('counts a book once however often it names the same author', () => {
    const shelves = groupBooks(
      [book('a', { title: 'Ambush', authors: ['Lee Child', 'Child, Lee'] })],
      'authors',
    )

    expect(shelves).toHaveLength(1)
    expect(shelves[0]!.books).toHaveLength(1)
  })

  it('collects the books nobody is named on', () => {
    const shelves = groupBooks(
      [
        book('a', { title: 'Ambush', authors: ['Lee Child'] }),
        book('b', { title: 'Anonymus', authors: [] }),
      ],
      'authors',
    )

    expect(shelves.map((shelf) => shelf.title)).toEqual(['Lee Child', 'Without an author'])
  })

  // The reason the shelf exists: read alphabetically, a series is read in the
  // wrong order.
  it('shelves a series in reading order', () => {
    const shelves = groupBooks(
      [
        book('a', { title: 'Ambush', series: 'Jack Reacher', series_index: 3 }),
        book('b', { title: 'Betrayal', series: 'Jack Reacher', series_index: 1 }),
        book('c', { title: 'Choice', series: 'Jack Reacher', series_index: 2 }),
      ],
      'series',
    )

    expect(titlesOf(shelves[0]!.books)).toEqual(['Betrayal', 'Choice', 'Ambush'])
  })

  it('puts a volume the publisher gave no number at the front', () => {
    const shelves = groupBooks(
      [
        book('a', { title: 'The First Novel', series: 'The Saga', series_index: 1 }),
        book('b', { title: 'A Short Story', series: 'The Saga', series_index: 0 }),
      ],
      'series',
    )

    expect(titlesOf(shelves[0]!.books)).toEqual(['A Short Story', 'The First Novel'])
  })

  // Grouping must never hide a book. Everything that belongs to no series still
  // has to be somewhere on the page.
  it('keeps the books that belong to no series, last', () => {
    const shelves = groupBooks(
      [
        book('a', { title: 'Alone' }),
        book('b', { title: 'Betrayal', series: 'Jack Reacher', series_index: 1 }),
      ],
      'series',
    )

    expect(shelves.map((shelf) => shelf.title)).toEqual(['Jack Reacher', 'Without a series'])
    expect(titlesOf(shelves[1]!.books)).toEqual(['Alone'])
  })

  it('folds every spelling of a language into one shelf, the biggest first', () => {
    const shelves = groupBooks(
      [
        book('a', { title: 'Eins', language: 'de' }),
        book('b', { title: 'Zwei', language: 'de-DE' }),
        book('c', { title: 'Drei', language: 'DE' }),
        book('d', { title: 'One', language: 'en' }),
      ],
      'languages',
    )

    expect(shelves.map((shelf) => shelf.title)).toEqual(['German', 'English'])
    expect(shelves[0]!.books).toHaveLength(3)
  })

  // "und" is what an EPUB says when it will not say, and a file that says nothing
  // at all means the same thing. One shelf of those beats two.
  it('shelves the books whose language is unknown together', () => {
    const shelves = groupBooks(
      [
        book('a', { title: 'Anonymus', language: 'und' }),
        book('b', { title: 'Nameless', language: '' }),
      ],
      'languages',
    )

    expect(shelves.map((shelf) => shelf.title)).toEqual(['Unknown'])
    expect(shelves[0]!.books).toHaveLength(2)
  })

  it('shows a language nothing here knows as the tag itself', () => {
    expect(languageName('sw')).toBe('SW')
  })
})
