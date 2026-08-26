//
// File:        webui/src/lib/grouping.ts
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

import type { Book } from '@/models'

/**
 * The ways the library can be broken into shelves.
 *
 * The same three the OPDS catalog offers, deliberately: somebody who browses
 * their library both ways should not find two different libraries. Subjects are
 * not among them for the reason given in `internal/opds/facets.go` — most of them
 * belong to exactly one book, so a shelf per subject is not navigation.
 */
export type Grouping = 'none' | 'authors' | 'series' | 'languages'

/** One shelf: what to call it, and what is on it. */
export interface BookGroup {
  /** Stable identity for the render, and what the shelf folded on. */
  key: string
  title: string
  books: Book[]
}

/**
 * maxGivenNames caps how much of a comma separated name may be the given part.
 *
 * "Child, Lee" is a name written backwards; "Corinna Mieth, Simon Weber, Rainer
 * Schäfer, Anna Schriefl" is four people crammed into one field, and turning that
 * inside out would produce nonsense.
 */
const maxGivenNames = 3

/**
 * The words that follow a comma without the name being written backwards.
 * Without them "Penguin Random House, LLC" becomes "LLC Penguin Random House".
 */
const nameSuffixes = new Set([
  'jr',
  'sr',
  'ii',
  'iii',
  'iv',
  'phd',
  'md',
  'esq',
  'llc',
  'inc',
  'ltd',
  'gmbh',
  'ag',
  'co',
])

/** A single letter or decimal digit, in any script. */
const letterOrDigit = /[\p{L}\p{Nd}]/u

/**
 * The form of an author's name to show.
 *
 * This is the browser's copy of `AuthorName` in `server/internal/books/authors.go`,
 * which the OPDS catalog uses. The rule has to hold in both places or the same
 * library reads differently depending on which client asked, and the shared cases
 * in `testdata/author-names.json` are what keeps them honest: change one side
 * and the other side's tests fail.
 */
export function authorName(raw: string): string {
  const name = raw.trim()

  const comma = name.indexOf(',')
  if (comma < 0) return name

  const family = name.slice(0, comma).trim()
  const given = name
    .slice(comma + 1)
    .replaceAll(',', ' ')
    .split(/\s+/)
    .filter(Boolean)

  if (!family || given.length === 0 || given.length > maxGivenNames) return name

  const last = given[given.length - 1]!.replace(/^\.+|\.+$/g, '').toLowerCase()
  if (nameSuffixes.has(last)) return name

  return `${given.join(' ')} ${family}`
}

/**
 * What every spelling of one author's name has in common.
 *
 * Two names belong to the same person when they are the same letters in the same
 * order: "George R. R. Martin", "George R.R. Martin" and "Martin, George, R. R."
 * differ only in the spaces and dots between the initials.
 *
 * Only punctuation and case are dropped, and nothing is transliterated. A name in
 * a script with no case and no spaces is its own key rather than an empty one,
 * which is what an ASCII-only key would make of it — and every such author would
 * then fold into a single nameless shelf.
 */
export function authorKey(name: string): string {
  return [...authorName(name).toLowerCase()].filter((letter) => letterOrDigit.test(letter)).join('')
}

/**
 * The language a stored tag means.
 *
 * A library stores "de", "de-DE" and "DE" for one language, and shelving those
 * apart reproduces exactly the splitting this exists to undo. The region goes
 * with the case.
 */
export function languageTag(raw: string): string {
  const tag = raw.trim().toLowerCase()
  const dash = tag.indexOf('-')

  return dash >= 0 ? tag.slice(0, dash) : tag
}

/**
 * The display names of the language tags a personal library is likely to hold.
 * Anything else is shown as the tag itself, uppercased, which is honest about the
 * fact that nothing here knows what it is.
 */
const languageNames: Record<string, string> = {
  cs: 'Czech',
  da: 'Danish',
  de: 'German',
  el: 'Greek',
  en: 'English',
  es: 'Spanish',
  fi: 'Finnish',
  fr: 'French',
  hu: 'Hungarian',
  it: 'Italian',
  ja: 'Japanese',
  la: 'Latin',
  nb: 'Norwegian',
  nl: 'Dutch',
  nn: 'Norwegian',
  pl: 'Polish',
  pt: 'Portuguese',
  ro: 'Romanian',
  ru: 'Russian',
  sv: 'Swedish',
  tr: 'Turkish',
  uk: 'Ukrainian',
  // Not a language: it is what an EPUB says when it will not say.
  und: 'Unknown',
  zh: 'Chinese',
}

/** What to call a language tag. */
export function languageName(tag: string): string {
  return languageNames[languageTag(tag)] ?? tag.toUpperCase()
}

/**
 * How two books on the same shelf are put in order.
 *
 * Passed in rather than decided here, because the order a library is read in is
 * a choice its owner makes and the shelves are not exempt from it: browsing by
 * author while sorting by what was read last has to give each author's shelf in
 * that order too, or the sort only appears to have been applied.
 */
export type Order = (a: Book, b: Book) => number

/** Titles, the way this interface sorts them everywhere else. */
const byTitle: Order = (a, b) => a.title.localeCompare(b.title)

/** Reading order: the number first, the title only to break ties. */
const byReadingOrder: Order = (a, b) =>
  a.series_index !== b.series_index ? a.series_index - b.series_index : byTitle(a, b)

/** Shelf names, case insensitively, the way the catalog orders them. */
const byName = (a: BookGroup, b: BookGroup) =>
  a.title.localeCompare(b.title, undefined, { sensitivity: 'base' }) ||
  a.title.localeCompare(b.title)

/**
 * Picks the name to show an author under.
 *
 * The one the library uses most, because that is the one its owner will
 * recognise. Two spellings used equally often are settled by taking the longer,
 * which is how "George R. R. Martin" wins over "George R.R. Martin" — and then by
 * the text itself, so the answer never depends on iteration order.
 */
function commonestSpelling(spellings: Map<string, number>): string {
  let best = ''
  let bestCount = -1

  for (const [spelling, count] of spellings) {
    if (
      count > bestCount ||
      (count === bestCount &&
        (spelling.length > best.length || (spelling.length === best.length && spelling < best)))
    ) {
      best = spelling
      bestCount = count
    }
  }

  return best
}

/**
 * The authors' shelves, one per person however their name is spelled.
 *
 * A book with two authors stands on both their shelves, which is why the counts
 * add up to more than the library holds.
 */
function byAuthor(books: Book[], order?: Order): BookGroup[] {
  const folded = new Map<string, { books: Book[]; spellings: Map<string, number> }>()
  const anonymous: Book[] = []

  for (const book of books) {
    const keys = new Set<string>()

    for (const written of book.authors ?? []) {
      const key = authorKey(written)
      if (!key) continue

      let one = folded.get(key)
      if (!one) {
        one = { books: [], spellings: new Map() }
        folded.set(key, one)
      }

      // Counted by book: a book naming the same author twice, once each way
      // round, is still one book on their shelf.
      if (!keys.has(key)) {
        one.books.push(book)
        keys.add(key)
      }

      const spelling = written.trim()
      one.spellings.set(spelling, (one.spellings.get(spelling) ?? 0) + 1)
    }

    if (keys.size === 0) anonymous.push(book)
  }

  const groups: BookGroup[] = []
  for (const [key, one] of folded) {
    groups.push({
      key,
      title: authorName(commonestSpelling(one.spellings)),
      books: one.books.sort(order ?? byTitle),
    })
  }
  groups.sort(byName)

  if (anonymous.length) {
    groups.push({ key: '', title: 'Without an author', books: anonymous.sort(order ?? byTitle) })
  }

  return groups
}

/**
 * The series' shelves, each in reading order unless another was chosen.
 *
 * Reading order is the default here and nowhere else, because it is the only
 * thing the number on a volume is for. An explicit sort still wins: somebody who
 * asked for the furthest read first asked about the whole library.
 */
function bySeries(books: Book[], order?: Order): BookGroup[] {
  const folded = new Map<string, Book[]>()
  const loose: Book[] = []

  for (const book of books) {
    const series = (book.series ?? '').trim()
    if (!series) {
      loose.push(book)
      continue
    }

    const shelf = folded.get(series)
    if (shelf) shelf.push(book)
    else folded.set(series, [book])
  }

  const groups: BookGroup[] = []
  for (const [series, shelf] of folded) {
    groups.push({ key: series, title: series, books: shelf.sort(order ?? byReadingOrder) })
  }
  groups.sort(byName)

  if (loose.length) {
    groups.push({ key: '', title: 'Without a series', books: loose.sort(order ?? byTitle) })
  }

  return groups
}

/**
 * The language shelves, the biggest first.
 *
 * Not alphabetically, because there are usually two or three of them and the one
 * somebody wants is the one most of their library is in. A book whose file names
 * no language at all is shelved with the ones whose file said "und": both mean
 * nobody knows, and one shelf of those is easier to look through than two.
 */
function byLanguage(books: Book[], order?: Order): BookGroup[] {
  const folded = new Map<string, Book[]>()

  for (const book of books) {
    const tag = languageTag(book.language ?? '') || 'und'

    const shelf = folded.get(tag)
    if (shelf) shelf.push(book)
    else folded.set(tag, [book])
  }

  return [...folded]
    .map(([tag, shelf]) => ({
      key: tag,
      title: languageName(tag),
      books: shelf.sort(order ?? byTitle),
    }))
    .sort((a, b) => b.books.length - a.books.length || a.key.localeCompare(b.key))
}

/**
 * Breaks a library into shelves.
 *
 * Ungrouped is one nameless shelf rather than an empty answer, so that the grid
 * that draws the result does not need to know which of the two it is looking at.
 * That shelf is handed back untouched: an ungrouped list is already in the order
 * its caller put it in, and there is nothing here to add to that.
 */
export function groupBooks(books: Book[], by: Grouping, order?: Order): BookGroup[] {
  switch (by) {
    case 'authors':
      return byAuthor(books, order)
    case 'series':
      return bySeries(books, order)
    case 'languages':
      return byLanguage(books, order)
    default:
      return books.length ? [{ key: '', title: '', books }] : []
  }
}
