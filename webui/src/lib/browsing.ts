//
// File:        webui/src/lib/browsing.ts
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

import type { Book } from '@/models'
import { authorName } from '@/lib/grouping'

/**
 * The orders the library can be read in.
 *
 * Title is the one everything else in this interface uses, and stays the
 * default. The other three are questions about the reading rather than about
 * the books — what did I put here last, what was I in the middle of, how far
 * did I get — and each of them runs newest or furthest first, because that is
 * the end of the list somebody sorting by them is asking about.
 */
export type Sorting = 'title' | 'added' | 'last-read' | 'progress'

/**
 * What the reading says about each book, keyed by book id.
 *
 * Neither of these lives on a book: a book is a file and its metadata, and the
 * reading is on the documents that were matched to it. The component that has
 * both folds them into this, so that ordering by them needs nothing else.
 */
export interface Reading {
  /** When each book was last read. Absent for a book nothing has read. */
  lastRead: Map<string, string>
  /** How far the reading got, from 0 to 1. Absent for a book nothing has read. */
  progress: Map<string, number>
}

/** Titles, the way this interface sorts them everywhere else. */
const byTitle = (a: Book, b: Book) => a.title.localeCompare(b.title)

/**
 * The form of a word this searches in, on both sides of the comparison.
 *
 * Case and accents are dropped, so that "Schafer" finds "Schäfer" — a library
 * is typed into from a keyboard that may not have the letter the publisher
 * used. Dropping combining marks is cruder than it looks in a script that
 * builds its vowels out of them, but it is applied to the query as well as to
 * the book, and a fold applied to both sides can only find more than it should,
 * never less. That is the right way for a search box to be wrong.
 *
 * Deliberately unlike `authorKey`, which folds names to decide whether two of
 * them are one person. That answer has to be exact and has to hold in scripts
 * this would flatten; this one only has to put a book on the screen.
 */
function fold(text: string): string {
  return text.normalize('NFD').replace(/\p{M}/gu, '').toLowerCase()
}

/**
 * Everything about a book that a search looks at: its title, its series, and
 * its authors both as the file spells them and as the library shows them.
 *
 * Both spellings, because the shelves say "Lee Child" and the file may well say
 * "Child, Lee". Somebody searching for what is written on the screen must find
 * it, and so must somebody searching for what is written in the book.
 */
function haystack(book: Book): string {
  const authors = (book.authors ?? []).flatMap((written) => [written, authorName(written)])

  return fold([book.title, book.series ?? '', ...authors].join(' '))
}

/**
 * The books a query is asking for.
 *
 * Every word has to appear somewhere, but they need not appear together or in
 * order: "child killing" finds Lee Child's "Killing Floor", which no single
 * substring of the two would. Nothing is required to match in the same field
 * either, since the author and the title are one thing to the person typing.
 */
export function searchBooks(books: Book[], query: string): Book[] {
  const terms = fold(query).split(/\s+/).filter(Boolean)
  if (terms.length === 0) return books

  return books.filter((book) => {
    const searched = haystack(book)

    return terms.every((term) => searched.includes(term))
  })
}

/**
 * How to order two books.
 *
 * Every order falls back on the title, so that books a sort cannot tell apart —
 * two never read, two at the same percentage — come out in a stable, readable
 * order instead of whatever the server happened to answer with.
 */
export function bookOrder(by: Sorting, reading: Reading): (a: Book, b: Book) => number {
  switch (by) {
    // Newest first: the question this order answers is what was added last.
    case 'added':
      return (a, b) => b.created.localeCompare(a.created) || byTitle(a, b)

    // Most recently read first. A book nothing has read has no date at all,
    // and the empty string sorts it to the end, which is where it belongs.
    case 'last-read':
      return (a, b) => {
        const left = reading.lastRead.get(a.id) ?? ''
        const right = reading.lastRead.get(b.id) ?? ''

        return right.localeCompare(left) || byTitle(a, b)
      }

    // Furthest first, so the finished are at the top and the untouched at the
    // bottom, with whatever is in the middle in between.
    case 'progress':
      return (a, b) => {
        const left = reading.progress.get(a.id) ?? 0
        const right = reading.progress.get(b.id) ?? 0

        return right - left || byTitle(a, b)
      }

    default:
      return byTitle
  }
}
