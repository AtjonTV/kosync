//
// File:        webui/src/tests/components/BookLibrary.test.ts
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

import { beforeEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { createTestingPinia } from '@pinia/testing'
import PrimeVue from 'primevue/config'
import ToastService from 'primevue/toastservice'
import ConfirmationService from 'primevue/confirmationservice'
import BookLibrary from '@/components/BookLibrary.vue'
import type { Book, DocumentRecord } from '@/models'

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

function book(id: string, overrides: Partial<Book> = {}): Book {
  return {
    id,
    collectionId: 'c',
    collectionName: 'books',
    created: '',
    updated: '',
    owner: 'user-a',
    file: 'book.epub',
    cover: 'cover.jpg',
    title: 'Zeit des Sturms',
    authors: ['Andrzej Sapkowski'],
    language: 'de',
    identifiers: {},
    series: '',
    series_index: 0,
    subjects: null,
    page_count: 700,
    word_count: 109288,
    file_size: 1_200_000,
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

function read(bookId: string, at: string): DocumentRecord {
  return {
    id: 'doc-' + bookId,
    collectionId: 'c',
    collectionName: 'documents',
    created: '',
    updated: '',
    owner: 'user-a',
    document: 'hash-' + bookId,
    title: 'x',
    current_location: '',
    progress: 0.5,
    last_device: 'go7',
    last_device_id: 'go7',
    last_read_at: at,
    source_account: '',
    book: bookId,
  }
}

function mountLibrary(
  books: Book[],
  props: Record<string, unknown> = {},
  documents: DocumentRecord[] = [],
  slots: Record<string, string> = {},
) {
  return mount(BookLibrary, {
    props,
    slots,
    global: {
      plugins: [
        createTestingPinia({
          createSpy: vi.fn,
          initialState: {
            books: { books, loaded: true },
            documents: { documents, loaded: true },
          },
        }),
        PrimeVue,
        ToastService,
        ConfirmationService,
      ],
      stubs: { RouterLink: { template: '<a><slot /></a>' } },
    },
  })
}

describe('BookLibrary', () => {
  // The Witcher omnibus has a title six lines long, which stretched the whole
  // grid row it sat in and left the shorter cards floating above a gap. Every
  // card now reserves the same two lines whether it fills them or not.
  it('reserves the same title space for every book', () => {
    const wrapper = mountLibrary([
      book('a', { title: 'Der letzte Wunsch' }),
      book('b', {
        title:
          'Die Witcher-Saga - Das Erbe der Elfen Die Zeit der Verachtung Feuertaufe ' +
          'Der Schwalbenturm Die Dame vom See',
      }),
    ])

    const titles = wrapper.findAll('.line-clamp-2')
    expect(titles).toHaveLength(2)
    for (const title of titles) {
      expect(title.classes()).toContain('min-h-[2.5em]')
    }
  })

  // A book whose metadata names no author still has to occupy an author's worth
  // of space, or it comes out shorter than the one beside it.
  it('keeps the author line even when there is no author', () => {
    const wrapper = mountLibrary([book('a', { authors: [] })])

    expect(wrapper.find('.line-clamp-1').exists()).toBe(true)
  })

  it('shows every book by title when it is not limited', () => {
    const wrapper = mountLibrary([book('a', { title: 'Bravo' }), book('b', { title: 'Alpha' })])
    const text = wrapper.text()

    expect(text.indexOf('Alpha')).toBeLessThan(text.indexOf('Bravo'))
  })

  // The dashboard is a shelf, not a catalogue: the most recently read come
  // first, and the rest are one link away.
  it('shows the most recently read first when it is limited', () => {
    const wrapper = mountLibrary(
      [book('a', { title: 'Alpha' }), book('b', { title: 'Bravo' }), book('c', { title: 'Delta' })],
      { limit: 2 },
      [read('b', '2026-03-05 10:00:00.000Z'), read('c', '2026-03-01 10:00:00.000Z')],
    )
    const text = wrapper.text()

    expect(text).toContain('Bravo')
    expect(text).not.toContain('Alpha')
    expect(text).toContain('See all 3 books')
  })

  it('prints no heading of its own when the page already has one', () => {
    const wrapper = mountLibrary([book('a')], { heading: '' })

    expect(wrapper.text()).not.toContain('Library')
  })

  describe('grouping', () => {
    beforeEach(() => localStorage.clear())

    it('offers the groupings on the library page', () => {
      expect(mountLibrary([book('a')]).text()).toContain('Group by')
    })

    // Breaking six recently read books into headed sections is noise, and the
    // page the dashboard links to is where the grouping belongs.
    it('offers nothing to group on the dashboard shelf', () => {
      expect(mountLibrary([book('a')], { limit: 6 }).text()).not.toContain('Group by')
    })

    it('offers nothing to group in an empty library', () => {
      expect(mountLibrary([]).text()).not.toContain('Group by')
    })

    // Remembered because somebody who browses by series today wants to browse by
    // series tomorrow, and the choice is worth less than making it again.
    it('starts out the way it was last left', () => {
      localStorage.setItem('library-grouping', 'series')

      const wrapper = mountLibrary([
        book('a', { title: 'Ambush', series: 'Jack Reacher', series_index: 3 }),
        book('b', { title: 'Betrayal', series: 'Jack Reacher', series_index: 1 }),
      ])
      const text = wrapper.text()

      expect(text).toContain('Jack Reacher')
      expect(text.indexOf('Betrayal')).toBeLessThan(text.indexOf('Ambush'))
    })

    it('ignores a stored grouping it cannot make sense of', () => {
      localStorage.setItem('library-grouping', 'by-colour-of-the-cover')

      const wrapper = mountLibrary([book('a', { title: 'Ambush', series: 'Jack Reacher' })])

      expect(wrapper.text()).not.toContain('Jack Reacher')
    })

    // The grid must never hide a book: whatever is chosen, everything in the
    // library is still somewhere on the page.
    it('keeps the books the grouping has no shelf for', () => {
      localStorage.setItem('library-grouping', 'series')

      const wrapper = mountLibrary([
        book('a', { title: 'Alone' }),
        book('b', { title: 'Betrayal', series: 'Jack Reacher', series_index: 1 }),
      ])
      const text = wrapper.text()

      expect(text).toContain('Without a series')
      expect(text).toContain('Alone')
    })

    // The whole reason the fold exists, seen from the page it now serves.
    it('heads one shelf with the name the library settled on', () => {
      localStorage.setItem('library-grouping', 'authors')

      const wrapper = mountLibrary([
        book('a', { title: 'Ambush', authors: ['Lee Child'] }),
        book('b', { title: 'Betrayal', authors: ['Lee Child'] }),
        book('c', { title: 'Choice', authors: ['Child, Lee'] }),
      ])
      const text = wrapper.text()

      expect(text).toContain('Lee Child')
      expect(text).not.toContain('Child, Lee')
    })

    it('never groups the dashboard shelf, whatever was last chosen', () => {
      localStorage.setItem('library-grouping', 'series')

      const wrapper = mountLibrary(
        [book('a', { title: 'Betrayal', series: 'Jack Reacher', series_index: 1 })],
        { limit: 6 },
      )

      expect(wrapper.find('h2').exists()).toBe(false)
    })
  })

  describe('searching', () => {
    beforeEach(() => localStorage.clear())

    const library = [
      book('a', { title: 'Killing Floor', authors: ['Child, Lee'], series: 'Jack Reacher' }),
      book('b', { title: 'Der Schwalbenturm', authors: ['Andrzej Sapkowski'], series: 'Hexer' }),
    ]

    it('offers a search on the library page', () => {
      expect(mountLibrary(library).find('#library-search').exists()).toBe(true)
    })

    // The dashboard is a shelf of six, and a collection is a list somebody put
    // together by hand. Neither is something to search through.
    it('offers none on the dashboard shelf or on a given list', () => {
      expect(mountLibrary(library, { limit: 6 }).find('#library-search').exists()).toBe(false)
      expect(mountLibrary(library, { books: library }).find('#library-search').exists()).toBe(false)
    })

    it('offers none in an empty library', () => {
      expect(mountLibrary([]).find('#library-search').exists()).toBe(false)
    })

    it('shows only what was asked for', async () => {
      const wrapper = mountLibrary(library)
      await wrapper.find('#library-search').setValue('sapkowski')

      expect(wrapper.text()).toContain('Der Schwalbenturm')
      expect(wrapper.text()).not.toContain('Killing Floor')
    })

    it('says how much of the library is left', async () => {
      const wrapper = mountLibrary(library)
      await wrapper.find('#library-search').setValue('sapkowski')

      expect(wrapper.text()).toContain('1 of 2')
    })

    // A library a search has emptied is not an empty library, and telling
    // somebody to upload an EPUB answers a question they did not ask.
    it('does not call an emptied library empty', async () => {
      const wrapper = mountLibrary(library)
      await wrapper.find('#library-search').setValue('tolkien')

      expect(wrapper.text()).toContain('No books match "tolkien".')
      expect(wrapper.text()).not.toContain('Add an EPUB to keep a copy here')
    })

    // The search narrows the library; the grouping decides how what is left is
    // laid out. Both at once has to mean both.
    it('groups what the search left', async () => {
      localStorage.setItem('library-grouping', 'series')

      const wrapper = mountLibrary(library)
      await wrapper.find('#library-search').setValue('sapkowski')

      expect(wrapper.text()).toContain('Hexer')
      expect(wrapper.text()).not.toContain('Jack Reacher')
    })
  })

  describe('sorting', () => {
    beforeEach(() => localStorage.clear())

    it('offers the sorts on the library page, and nowhere else', () => {
      expect(
        mountLibrary([book('a')])
          .find('#library-sort')
          .exists(),
      ).toBe(true)
      expect(
        mountLibrary([book('a')], { limit: 6 })
          .find('#library-sort')
          .exists(),
      ).toBe(false)
    })

    it('sorts by title until it is told otherwise', () => {
      const wrapper = mountLibrary([book('a', { title: 'Bravo' }), book('b', { title: 'Alpha' })])
      const text = wrapper.text()

      expect(text.indexOf('Alpha')).toBeLessThan(text.indexOf('Bravo'))
    })

    // Remembered for the same reason the grouping is: it is a way of reading a
    // library, and making the choice again every visit is worth more than it.
    it('starts out the way it was last left', () => {
      localStorage.setItem('library-sort', 'added')

      const wrapper = mountLibrary([
        book('a', { title: 'Alpha', created: '2026-01-01 00:00:00.000Z' }),
        book('b', { title: 'Bravo', created: '2026-06-01 00:00:00.000Z' }),
      ])
      const text = wrapper.text()

      expect(text.indexOf('Bravo')).toBeLessThan(text.indexOf('Alpha'))
    })

    it('ignores a stored sort it cannot make sense of', () => {
      localStorage.setItem('library-sort', 'by-the-weight-of-the-paper')

      const wrapper = mountLibrary([
        book('a', { title: 'Bravo', created: '2026-06-01 00:00:00.000Z' }),
        book('b', { title: 'Alpha', created: '2026-01-01 00:00:00.000Z' }),
      ])
      const text = wrapper.text()

      expect(text.indexOf('Alpha')).toBeLessThan(text.indexOf('Bravo'))
    })

    it('puts the most recently read first when asked', () => {
      localStorage.setItem('library-sort', 'last-read')

      const wrapper = mountLibrary(
        [book('a', { title: 'Alpha' }), book('b', { title: 'Bravo' })],
        {},
        [read('a', '2026-03-01 10:00:00.000Z'), read('b', '2026-05-01 10:00:00.000Z')],
      )
      const text = wrapper.text()

      expect(text.indexOf('Bravo')).toBeLessThan(text.indexOf('Alpha'))
    })

    it('puts the furthest read first when asked', () => {
      localStorage.setItem('library-sort', 'progress')

      const wrapper = mountLibrary(
        [book('a', { title: 'Alpha' }), book('b', { title: 'Bravo' })],
        {},
        [
          { ...read('a', '2026-03-01 10:00:00.000Z'), progress: 0.1 },
          { ...read('b', '2026-05-01 10:00:00.000Z'), progress: 0.9 },
        ],
      )
      const text = wrapper.text()

      expect(text.indexOf('Bravo')).toBeLessThan(text.indexOf('Alpha'))
    })

    // The sort is about the library, not about the ungrouped view of it: the
    // shelves have to come out in it too, or it has only appeared to apply.
    it('carries into the shelves a grouping makes', () => {
      localStorage.setItem('library-sort', 'added')
      localStorage.setItem('library-grouping', 'series')

      const wrapper = mountLibrary([
        book('a', {
          title: 'Alpha',
          series: 'Jack Reacher',
          series_index: 1,
          created: '2026-01-01 00:00:00.000Z',
        }),
        book('b', {
          title: 'Bravo',
          series: 'Jack Reacher',
          series_index: 2,
          created: '2026-06-01 00:00:00.000Z',
        }),
      ])
      const text = wrapper.text()

      expect(text).toContain('Jack Reacher')
      expect(text.indexOf('Bravo')).toBeLessThan(text.indexOf('Alpha'))
    })

    // Which the reading order of a series otherwise wins, and must keep winning
    // for somebody who has not asked for anything else.
    it('leaves a series in reading order while sorting by title', () => {
      localStorage.setItem('library-grouping', 'series')

      const wrapper = mountLibrary([
        book('a', { title: 'Alpha', series: 'Jack Reacher', series_index: 2 }),
        book('b', { title: 'Bravo', series: 'Jack Reacher', series_index: 1 }),
      ])
      const text = wrapper.text()

      expect(text.indexOf('Bravo')).toBeLessThan(text.indexOf('Alpha'))
    })

    // The dashboard shelf is the recently read, whatever the library page was
    // last left sorted by.
    it('never reorders the dashboard shelf', () => {
      localStorage.setItem('library-sort', 'title')

      const wrapper = mountLibrary(
        [book('a', { title: 'Alpha' }), book('b', { title: 'Bravo' })],
        { limit: 6 },
        [read('a', '2026-03-01 10:00:00.000Z'), read('b', '2026-05-01 10:00:00.000Z')],
      )
      const text = wrapper.text()

      expect(text.indexOf('Bravo')).toBeLessThan(text.indexOf('Alpha'))
    })
  })

  // A collection hands the grid its own books in its own order. That order is
  // the entire content of a hand-made shelf, so nothing here may touch it.
  describe('a given list', () => {
    beforeEach(() => localStorage.clear())

    const shelf = [
      book('c', { title: 'Choice' }),
      book('a', { title: 'Ambush' }),
      book('b', { title: 'Betrayal' }),
    ]

    it('shows it in the order it was given', () => {
      const wrapper = mountLibrary(shelf, { books: shelf })
      const text = wrapper.text()

      expect(text.indexOf('Choice')).toBeLessThan(text.indexOf('Ambush'))
      expect(text.indexOf('Ambush')).toBeLessThan(text.indexOf('Betrayal'))
    })

    // Grouping would sort it, so the remembered choice is ignored here rather
    // than quietly rearranging somebody's reading list.
    it('ignores the remembered grouping', () => {
      localStorage.setItem('library-grouping', 'authors')

      const wrapper = mountLibrary(shelf, { books: shelf })

      expect(wrapper.find('h2').exists()).toBe(false)
      expect(wrapper.text()).not.toContain('Group by')
    })

    // Neither uploading nor deleting belongs on a page about one shelf: the
    // first has nothing to do with it, and the second would take the file away
    // rather than the book off the shelf.
    it('offers neither uploading nor deleting', () => {
      const wrapper = mountLibrary(shelf, { books: shelf })

      expect(wrapper.findComponent({ name: 'FileUpload' }).exists()).toBe(false)
      expect(wrapper.find('[title="Delete"]').exists()).toBe(false)
      expect(wrapper.find('[title="Change title"]').exists()).toBe(true)
    })

    it('says what the page it is on means by empty', () => {
      const wrapper = mountLibrary([], { books: [], empty: 'Nothing on this shelf yet.' })

      expect(wrapper.text()).toContain('Nothing on this shelf yet.')
      expect(wrapper.text()).not.toContain('Add an EPUB')
    })

    it('lets the page add its own action to every card', () => {
      const wrapper = mountLibrary(shelf, { books: shelf }, [], {
        actions: '<button class="take-off">Take off</button>',
      })

      expect(wrapper.findAll('.take-off')).toHaveLength(3)
    })
  })
})
