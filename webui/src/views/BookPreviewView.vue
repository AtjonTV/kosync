<!--
  File:        webui/src/views/BookPreviewView.vue
  Project:     https://git.obth.eu/atjontv/kosync
  Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
-->
<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { KosyncApi, errorMessage, pb } from '@/pb'
import type { PreviewChapter, PreviewEntry, PreviewOutline } from '@/models'

/**
 * A look inside a book, without opening it.
 *
 * This is a preview and not a reader: it records nothing, counts as no reading,
 * and forgets which chapter was open the moment the page is left. The question
 * it answers is "what is this one about", which is asked once and then done
 * with.
 *
 * The chapter arrives as markup the server rebuilt out of a small set of
 * elements, and it is rendered inside an <iframe sandbox> with no tokens at
 * all. The frame, not the rebuild, is the boundary: nothing in it can run
 * script, reach the session, or touch this document even if the rebuild has a
 * hole in it.
 */

const route = useRoute()
const router = useRouter()

const bookId = computed(() => String(route.params.id ?? ''))

const outline = ref<PreviewOutline | null>(null)
const chapter = ref<PreviewChapter | null>(null)
const current = ref(0)
const loading = ref(true)
const failure = ref('')
const listOpen = ref(false)
const dark = ref(false)

// The chapters already fetched, for the life of this page and no longer. Paging
// back is a page turn, and a page turn should not be a request.
const fetched = new Map<number, PreviewChapter>()

const chapters = computed<PreviewEntry[]>(() => outline.value?.chapters ?? [])

// The chapter list, broken where the book breaks. A trilogy in one file numbers
// its chapters from one three times over, so a flat list of them reads "1, 2,
// 3 … 1, 2, 3" and says nothing about which of the three you are picking.
const parts = computed(() => {
  const groups: { section: string; entries: PreviewEntry[] }[] = []

  for (const entry of chapters.value) {
    const section = entry.section ?? ''
    const last = groups[groups.length - 1]
    if (last && last.section === section) last.entries.push(entry)
    else groups.push({ section, entries: [entry] })
  }

  return groups
})

/** What the header calls the open chapter, with the part it is in. */
const heading = computed(() => {
  if (!chapter.value) return ''

  return [chapter.value.section, chapter.value.title].filter(Boolean).join(' · ')
})

// Where the open chapter stands in the list. The spine numbers it goes by are
// the file's own and can have gaps in them, so "the next one" is a step through
// the list rather than an addition.
const position = computed(() => chapters.value.findIndex((entry) => entry.index === current.value))
const hasPrevious = computed(() => position.value > 0)
const hasNext = computed(() => position.value >= 0 && position.value < chapters.value.length - 1)

/** Asks for the chapter list, and opens the first one. */
async function open(): Promise<void> {
  loading.value = true
  failure.value = ''
  try {
    const response = await pb.send<PreviewOutline>(KosyncApi.bookPreview(bookId.value), {
      method: 'GET',
    })
    outline.value = { title: response?.title ?? '', chapters: response?.chapters ?? [] }
  } catch (error) {
    failure.value = errorMessage(error, 'This book could not be opened for preview.')
    return
  } finally {
    loading.value = false
  }

  const first = chapters.value[0]
  if (!first) {
    failure.value = 'There is nothing readable in this book.'
    return
  }

  await show(first.index)
}

/** Puts one chapter on screen, fetching it unless it has been seen already. */
async function show(index: number): Promise<void> {
  listOpen.value = false
  current.value = index

  const held = fetched.get(index)
  if (held) {
    chapter.value = held
    failure.value = ''
    return
  }

  loading.value = true
  failure.value = ''
  try {
    const response = await pb.send<PreviewChapter>(
      KosyncApi.bookPreviewChapter(bookId.value, index),
      { method: 'GET' },
    )
    fetched.set(index, response)
    chapter.value = response
  } catch (error) {
    chapter.value = null
    failure.value = errorMessage(error, 'This chapter could not be read.')
  } finally {
    loading.value = false
  }
}

/** Moves one chapter along the list. */
function step(by: number): void {
  const next = chapters.value[position.value + by]
  if (next) void show(next.index)
}

function leave(): void {
  void router.push({ name: 'book', params: { id: bookId.value } })
}

// Arrow keys page, because a preview read at a desk is read with one hand on
// the keyboard. A key pressed inside the frame never arrives here — the frame
// is a separate document — which is what the footer buttons are for.
function onKey(event: KeyboardEvent): void {
  if (event.defaultPrevented || event.altKey || event.ctrlKey || event.metaKey) return

  if (event.key === 'ArrowLeft' && hasPrevious.value) step(-1)
  else if (event.key === 'ArrowRight' && hasNext.value) step(1)
}

// The interface's dark mode is a class the reader can set against the system
// preference, so the frame is told which one to draw rather than asking the
// system itself and disagreeing with the page around it.
let themeWatch: MutationObserver | null = null

function readTheme(): void {
  dark.value = document.documentElement.classList.contains('p-dark')
}

/** The stylesheet the frame gets. The book's own CSS is never loaded. */
function sheet(): string {
  const ink = dark.value ? '#d4d4d8' : '#27272a'
  const paper = dark.value ? '#09090b' : '#ffffff'
  const quiet = dark.value ? '#71717a' : '#a1a1aa'

  return `
    html { -webkit-text-size-adjust: 100%; }
    body {
      margin: 0 auto; padding: 1.5rem 1.25rem 3rem; max-width: 38rem;
      font-family: Georgia, 'Times New Roman', serif; font-size: 1.05rem;
      line-height: 1.7; color: ${ink}; background: ${paper};
      overflow-wrap: break-word;
    }
    p { margin: 0 0 1em; }
    h1, h2, h3, h4, h5, h6 { line-height: 1.3; margin: 1.8em 0 0.6em; }
    img { display: block; max-width: 100%; height: auto; margin: 1.5em auto; }
    blockquote { margin: 1.5em 0; padding-left: 1em; border-left: 2px solid ${quiet}; }
    table { border-collapse: collapse; width: 100%; }
    td, th { border: 1px solid ${quiet}; padding: 0.25em 0.5em; }
    pre { overflow-x: auto; }
    hr { border: 0; border-top: 1px solid ${quiet}; margin: 2em 0; }
  `
}

const page = computed(
  () =>
    `<!doctype html><html><head><meta charset="utf-8">` +
    `<meta name="viewport" content="width=device-width, initial-scale=1">` +
    `<style>${sheet()}</style></head><body>${chapter.value?.html ?? ''}</body></html>`,
)

onMounted(() => {
  readTheme()
  themeWatch = new MutationObserver(readTheme)
  themeWatch.observe(document.documentElement, { attributes: true, attributeFilter: ['class'] })
  window.addEventListener('keydown', onKey)
  void open()
})

onUnmounted(() => {
  themeWatch?.disconnect()
  themeWatch = null
  window.removeEventListener('keydown', onKey)
})
</script>

<template>
  <section class="flex flex-col gap-3">
    <div class="flex items-center gap-2">
      <Button icon="pi pi-arrow-left" label="Back" variant="text" size="small" @click="leave" />
      <div class="min-w-0 flex-1 text-center">
        <p class="truncate font-medium">{{ outline?.title }}</p>
        <p class="truncate text-sm text-surface-500 dark:text-surface-400">
          {{ heading }}
        </p>
      </div>
      <Button
        icon="pi pi-bars"
        variant="text"
        size="small"
        aria-label="Chapters"
        aria-haspopup="true"
        aria-controls="preview-chapters"
        :disabled="!chapters.length"
        @click="listOpen = true"
      />
    </div>

    <Message v-if="failure" severity="error">
      {{ failure }}
    </Message>

    <!--
      A fixed height rather than one that grows to fit: with no script in it the
      frame cannot report how tall its document is, and a full window reader
      wants a scrolling pane anyway.
    -->
    <div
      class="relative h-[calc(100dvh-18rem)] min-h-80 overflow-hidden rounded-lg border border-surface-200 dark:border-surface-700 bg-white dark:bg-surface-950"
    >
      <iframe
        v-if="chapter"
        :srcdoc="page"
        sandbox=""
        referrerpolicy="no-referrer"
        :title="chapter.title"
        class="h-full w-full border-0"
      ></iframe>
      <div
        v-if="loading"
        class="absolute inset-0 flex items-center justify-center bg-white/70 dark:bg-surface-950/70"
      >
        <i class="pi pi-spin pi-spinner text-2xl text-surface-500" aria-label="Loading"></i>
      </div>
    </div>

    <Message v-if="chapter?.truncated" severity="info" variant="simple">
      This chapter is longer than the preview shows. Open the book on your reader for the rest.
    </Message>

    <div class="flex items-center justify-between gap-2">
      <Button
        icon="pi pi-chevron-left"
        label="Previous"
        variant="outlined"
        size="small"
        :disabled="!hasPrevious"
        @click="step(-1)"
      />
      <span v-if="chapters.length" class="text-sm text-surface-500 dark:text-surface-400">
        Chapter {{ position + 1 }} of {{ chapters.length }}
      </span>
      <Button
        icon="pi pi-chevron-right"
        icon-pos="right"
        label="Next"
        variant="outlined"
        size="small"
        :disabled="!hasNext"
        @click="step(1)"
      />
    </div>

    <Drawer id="preview-chapters" v-model:visible="listOpen" position="right" header="Chapters">
      <div v-for="(part, at) in parts" :key="at" class="flex flex-col gap-1">
        <p
          v-if="part.section"
          class="px-3 pt-3 text-xs uppercase tracking-wide text-surface-500 dark:text-surface-400"
        >
          {{ part.section }}
        </p>
        <ul class="flex flex-col gap-1">
          <li v-for="entry in part.entries" :key="entry.index">
            <button
              type="button"
              class="w-full rounded-lg px-3 py-2 text-left"
              :class="
                entry.index === current
                  ? 'bg-surface-100 dark:bg-surface-800 font-medium'
                  : 'hover:bg-surface-100 dark:hover:bg-surface-800'
              "
              @click="show(entry.index)"
            >
              {{ entry.title }}
            </button>
          </li>
        </ul>
      </div>
    </Drawer>
  </section>
</template>
