//
// File:        webui/src/tests/components/BookPreviewView.test.ts
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import PrimeVue from 'primevue/config'
import BookPreviewView from '@/views/BookPreviewView.vue'

vi.mock('@/pb', async () => {
  const mock = await import('../mocks/pb')
  const actual = await vi.importActual<typeof import('@/pb')>('@/pb')

  return {
    pb: mock.pbMock,
    Collections: actual.Collections,
    KosyncApi: actual.KosyncApi,
    errorMessage: actual.errorMessage,
  }
})

const push = vi.fn()

vi.mock('vue-router', () => ({
  useRoute: () => ({ params: { id: 'book-a' } }),
  useRouter: () => ({ push }),
}))

const { reset, send } = await import('../mocks/pb')

const outline = {
  title: 'Zeit des Sturms',
  chapters: [
    { index: 0, title: 'Kapitel 1' },
    { index: 1, title: 'Kapitel 2' },
    { index: 3, title: 'Kapitel 4' },
  ],
}

const chapters: Record<number, { index: number; title: string; html: string; truncated: boolean }> =
  {
    0: { index: 0, title: 'Kapitel 1', html: '<p>Ein Sturm zieht auf.</p>', truncated: false },
    1: { index: 1, title: 'Kapitel 2', html: '<p>Der Hexer wartet.</p>', truncated: false },
    3: { index: 3, title: 'Kapitel 4', html: '<p>Es endet hier.</p>', truncated: true },
  }

/** Answers the two preview endpoints out of the fixtures above. */
function serve(): void {
  send.mockImplementation(async (url: string) => {
    if (url === '/api/kosync/books/book-a/preview') return outline

    const asked = /\/preview\/(\d+)$/.exec(url)
    if (asked) return chapters[Number(asked[1])]

    throw new Error(`nothing serves ${url}`)
  })
}

function mountPreview() {
  return mount(BookPreviewView, { global: { plugins: [PrimeVue] } })
}

/** What the frame was handed to draw. */
function drawn(wrapper: ReturnType<typeof mountPreview>): string {
  return wrapper.find('iframe').attributes('srcdoc') ?? ''
}

/** How many times a chapter, any chapter, was asked for. */
function chapterRequests(): number {
  return send.mock.calls.filter((call) => /\/preview\/\d+$/.test(String(call[0]))).length
}

describe('BookPreviewView', () => {
  beforeEach(() => {
    reset()
    push.mockClear()
    document.body.innerHTML = ''
    serve()
  })

  it('opens the book at its first chapter', async () => {
    const wrapper = mountPreview()
    await flushPromises()

    expect(wrapper.text()).toContain('Zeit des Sturms')
    expect(wrapper.text()).toContain('Kapitel 1')
    expect(wrapper.text()).toContain('Chapter 1 of 3')
    expect(drawn(wrapper)).toContain('<p>Ein Sturm zieht auf.</p>')
  })

  // The frame is the security boundary, not the server's rebuild of the markup:
  // an empty sandbox forbids everything, scripts and the parent document above
  // all. A token added here would be a hole in the feature.
  it('draws the chapter in a frame that may do nothing', async () => {
    const wrapper = mountPreview()
    await flushPromises()

    const frame = wrapper.find('iframe')
    expect(frame.attributes()).toHaveProperty('sandbox')
    // Empty, and it has to stay empty: every token is a permission granted.
    expect(frame.attributes('sandbox')).toBe('')
    expect(frame.attributes('referrerpolicy')).toBe('no-referrer')
  })

  // The book's own stylesheet is never loaded, so the frame is given one.
  it('sends its own stylesheet along with the chapter', async () => {
    const wrapper = mountPreview()
    await flushPromises()

    expect(drawn(wrapper)).toContain('<style>')
    expect(drawn(wrapper)).toContain('max-width: 38rem')
  })

  it('pages forward and back through the spine', async () => {
    const wrapper = mountPreview()
    await flushPromises()

    const next = wrapper.findAll('button').find((node) => node.text().includes('Next'))!
    await next.trigger('click')
    await flushPromises()

    expect(drawn(wrapper)).toContain('Der Hexer wartet.')
    expect(wrapper.text()).toContain('Chapter 2 of 3')

    const previous = wrapper.findAll('button').find((node) => node.text().includes('Previous'))!
    await previous.trigger('click')
    await flushPromises()

    expect(drawn(wrapper)).toContain('Ein Sturm zieht auf.')
  })

  // Paging back is a page turn, and a page turn should not be a request.
  it('keeps a chapter it has already read', async () => {
    const wrapper = mountPreview()
    await flushPromises()

    const next = wrapper.findAll('button').find((node) => node.text().includes('Next'))!
    await next.trigger('click')
    await flushPromises()

    const previous = wrapper.findAll('button').find((node) => node.text().includes('Previous'))!
    await previous.trigger('click')
    await flushPromises()

    expect(chapterRequests()).toBe(2)
  })

  // The spine numbers come from the file and can have gaps, so the last entry
  // here is chapter 4 and stepping to it must not ask for the missing 2.
  it('stops at the last chapter', async () => {
    const wrapper = mountPreview()
    await flushPromises()

    const next = () => wrapper.findAll('button').find((node) => node.text().includes('Next'))!
    await next().trigger('click')
    await flushPromises()
    await next().trigger('click')
    await flushPromises()

    expect(drawn(wrapper)).toContain('Es endet hier.')
    expect(wrapper.text()).toContain('Chapter 3 of 3')
    expect(next().attributes('disabled')).toBeDefined()
  })

  it('jumps to a chapter picked from the list', async () => {
    const wrapper = mountPreview()
    await flushPromises()

    await wrapper.find('[aria-controls="preview-chapters"]').trigger('click')
    await flushPromises()

    const entry = Array.from(document.body.querySelectorAll('button')).find(
      (node) => node.textContent?.trim() === 'Kapitel 4',
    )!
    entry.click()
    await flushPromises()

    expect(drawn(wrapper)).toContain('Es endet hier.')
  })

  // A chapter the server had to cut short says so, because the reader would
  // otherwise take the end of the page for the end of the chapter.
  it('says when a chapter was cut short', async () => {
    const wrapper = mountPreview()
    await flushPromises()

    expect(wrapper.text()).not.toContain('longer than the preview shows')

    const next = () => wrapper.findAll('button').find((node) => node.text().includes('Next'))!
    await next().trigger('click')
    await flushPromises()
    await next().trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('longer than the preview shows')
  })

  it('says so when the book cannot be previewed', async () => {
    send.mockRejectedValue({ response: { message: 'This book cannot be previewed.' } })

    const wrapper = mountPreview()
    await flushPromises()

    expect(wrapper.text()).toContain('This book cannot be previewed.')
    expect(wrapper.find('iframe').exists()).toBe(false)
  })

  it('says so when one chapter cannot be read', async () => {
    send.mockImplementation(async (url: string) => {
      if (url === '/api/kosync/books/book-a/preview') return outline
      throw { response: { message: 'This chapter could not be read.' } }
    })

    const wrapper = mountPreview()
    await flushPromises()

    expect(wrapper.text()).toContain('This chapter could not be read.')
  })

  // Read at a desk, a preview is read with one hand on the keyboard.
  it('pages with the arrow keys', async () => {
    const wrapper = mountPreview()
    await flushPromises()

    window.dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowRight' }))
    await flushPromises()
    expect(drawn(wrapper)).toContain('Der Hexer wartet.')

    window.dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowLeft' }))
    await flushPromises()
    expect(drawn(wrapper)).toContain('Ein Sturm zieht auf.')
  })

  it('stops listening for keys once it is gone', async () => {
    const wrapper = mountPreview()
    await flushPromises()
    wrapper.unmount()

    window.dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowRight' }))
    await flushPromises()

    expect(chapterRequests()).toBe(1)
  })

  // The interface's dark mode is a class the reader can set against the system
  // preference, so the frame is told which one to draw rather than asking the
  // system and disagreeing with the page around it.
  it('draws the chapter in the theme the interface is in', async () => {
    document.documentElement.classList.add('p-dark')
    try {
      const wrapper = mountPreview()
      await flushPromises()

      expect(drawn(wrapper)).toContain('#09090b')
      expect(drawn(wrapper)).not.toContain('#ffffff')
    } finally {
      document.documentElement.classList.remove('p-dark')
    }
  })

  // A trilogy in one file numbers its chapters from one three times over, so a
  // flat list reads "1, 2, 3 ... 1, 2, 3" and says nothing about which of the
  // three is being picked.
  describe('books in parts', () => {
    beforeEach(() => {
      send.mockImplementation(async (url: string) => {
        if (url === '/api/kosync/books/book-a/preview') {
          return {
            title: 'Metro - Die Trilogie',
            chapters: [
              { index: 0, title: 'Cover' },
              { index: 1, title: '1', section: 'METRO 2033' },
              { index: 2, title: '1', section: 'METRO 2034' },
            ],
          }
        }

        const asked = /\/preview\/(\d+)$/.exec(url)
        return {
          index: Number(asked![1]),
          title: '1',
          section: 'METRO 2033',
          html: '<p>Ein Sturm zieht auf.</p>',
          truncated: false,
        }
      })
    })

    it('names the part the open chapter is in', async () => {
      const wrapper = mountPreview()
      await flushPromises()

      expect(wrapper.text()).toContain('METRO 2033 · 1')
    })

    it('breaks the chapter list where the book breaks', async () => {
      const wrapper = mountPreview()
      await flushPromises()

      await wrapper.find('[aria-controls="preview-chapters"]').trigger('click')
      await flushPromises()

      const drawer = document.body.textContent ?? ''
      expect(drawer).toContain('METRO 2033')
      expect(drawer).toContain('METRO 2034')
    })
  })

  it('goes back to the book', async () => {
    const wrapper = mountPreview()
    await flushPromises()

    const back = wrapper.findAll('button').find((node) => node.text().includes('Back'))!
    await back.trigger('click')

    expect(push).toHaveBeenCalledWith({ name: 'book', params: { id: 'book-a' } })
  })
})
