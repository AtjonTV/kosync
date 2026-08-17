<!--
  File:        webui/src/views/BookView.vue
  Project:     https://git.obth.eu/atjontv/kosync
  Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
-->
<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, useTemplateRef, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import Chart from 'primevue/chart'
import { useToast } from 'primevue/usetoast'
import { errorMessage, fileUrl } from '@/pb'
import { authorName } from '@/lib/grouping'
import { useBooksStore } from '@/stores/books'
import { useBookStatsStore } from '@/stores/bookStats'
import { useDocumentsStore } from '@/stores/documents'
import { useDevicesStore } from '@/stores/devices'
import { useCollectionsStore } from '@/stores/collections'
import type { MenuItem } from 'primevue/menuitem'

const route = useRoute()
const router = useRouter()
const books = useBooksStore()
const stats = useBookStatsStore()
const documents = useDocumentsStore()
const devices = useDevicesStore()
const collections = useCollectionsStore()
const toast = useToast()

const bookId = computed(() => String(route.params.id ?? ''))
const book = computed(() => books.books.find((entry) => entry.id === bookId.value) ?? null)

// Tidied the way the library grid and the catalog tidy them: one book should not
// name its author differently depending on which page is open.
const authors = computed(() => (book.value?.authors ?? []).map(authorName).join(', '))
const coverUrl = computed(() =>
  book.value ? fileUrl(book.value, book.value.cover, '200x300') : '',
)
const downloadUrl = computed(() => (book.value ? fileUrl(book.value, book.value.file) : ''))

/** How far the reading has got, taken from the furthest of the linked documents. */
const progress = computed(() => {
  let furthest = 0
  let found = false

  for (const document of documents.documents) {
    if (document.book !== bookId.value) continue
    found = true
    if (document.progress > furthest) furthest = document.progress
  }

  return found ? furthest : null
})

const percent = computed(() => Math.round((progress.value ?? 0) * 100))

/**
 * The page count, and where it came from.
 *
 * A measured count is the number of pages the device itself paginated the book
 * into: either stated in the statistics it synced here, or recovered from the
 * size of the steps its progress moved in. The fallback is what the word count
 * implies, which on the reference books was out by up to a third, so all of them
 * are labelled rather than blended.
 */
const pages = computed(() => {
  if (!book.value) return null
  if (book.value.measured_pages > 0) {
    // A count out of the statistics database is the reader's own number, and the
    // file does not say which reader wrote it — so it is not attributed to one.
    if (book.value.measured_source === 'device') {
      return { count: book.value.measured_pages, note: 'counted by your reader' }
    }

    // The book stores the device's identifier, because that is what survives a
    // rename; what belongs on screen is whatever the owner calls the thing.
    const device = devices.nameOf(book.value.measured_device)

    return {
      count: book.value.measured_pages,
      note: device ? `measured on ${device}` : 'measured from your reading',
    }
  }
  if (book.value.page_count > 0) {
    return { count: book.value.page_count, note: 'estimated from the word count' }
  }

  return null
})

const numberFormat = new Intl.NumberFormat()
const formatCount = (value: number) => numberFormat.format(value ?? 0)

/** Reading time as hours and minutes, because a book runs to hours. */
function formatDuration(seconds: number): string {
  const minutes = Math.round((seconds ?? 0) / 60)
  if (minutes < 60) return `${minutes} min`

  return `${Math.floor(minutes / 60)} h ${minutes % 60} min`
}

const chartOptions = ref()

const setChartOptions = () => {
  const style = getComputedStyle(document.documentElement)
  const textColor = style.getPropertyValue('--p-text-color').trim() || '#4b5563'
  const mutedColor = style.getPropertyValue('--p-text-muted-color').trim() || '#6b7280'
  const borderColor = style.getPropertyValue('--p-content-border-color').trim() || '#e5e7eb'

  return {
    maintainAspectRatio: false,
    plugins: { legend: { labels: { color: textColor } } },
    scales: {
      x: { ticks: { color: mutedColor }, grid: { color: borderColor } },
      y: {
        type: 'linear',
        position: 'left',
        ticks: { color: mutedColor },
        grid: { color: borderColor },
        title: { display: true, text: 'Pages', color: textColor },
      },
      y1: {
        type: 'linear',
        position: 'right',
        ticks: { color: mutedColor },
        grid: { drawOnChartArea: false, color: borderColor },
        title: { display: true, text: 'Minutes', color: textColor },
      },
    },
  }
}

// Only the days the book was actually read are shown. Filling the gaps would
// stretch a book read over three months into a mostly empty chart.
const chartData = computed(() => {
  if (stats.days.length === 0) return null

  return {
    labels: stats.days.map((day) => day.date),
    datasets: [
      {
        type: 'bar',
        label: 'Pages',
        data: stats.days.map((day) => day.pages_read),
        backgroundColor: 'rgba(59, 130, 246, 0.5)',
        borderColor: '#3b82f6',
        yAxisID: 'y',
      },
      {
        type: 'line',
        label: 'Reading time (min)',
        data: stats.days.map((day) => Math.round((day.reading_time / 60) * 10) / 10),
        borderColor: '#f59e0b',
        tension: 0.4,
        fill: false,
        yAxisID: 'y1',
      },
    ],
  }
})

/** The shelves this book stands on. */
const onShelves = computed(() => collections.byBook.get(bookId.value) ?? [])

/**
 * Where else it could go: the shelves it is not on yet, offered as a menu.
 *
 * An account with no shelves at all is sent to the page that makes them rather
 * than being shown an empty menu, which would only look broken.
 */
const shelfMenu = useTemplateRef<{ toggle: (event: Event) => void }>('shelfMenu')

const fileOn = async (id: string) => {
  try {
    await collections.addBook(id, bookId.value)
  } catch (error) {
    toast.add({ severity: 'error', summary: 'Failed', detail: errorMessage(error), life: 5000 })
  }
}

const takeOff = async (id: string) => {
  try {
    await collections.removeBook(id, bookId.value)
  } catch (error) {
    toast.add({ severity: 'error', summary: 'Failed', detail: errorMessage(error), life: 5000 })
  }
}

const shelfItems = computed<MenuItem[]>(() => {
  const already = new Set(onShelves.value.map((shelf) => shelf.id))
  const rest = collections.collections.filter((shelf) => !already.has(shelf.id))

  if (!rest.length) {
    return [
      {
        label: collections.collections.length
          ? 'On every collection already'
          : 'No collections yet',
        icon: 'pi pi-bookmark',
        command: () => {
          void router.push({ name: 'collections' })
        },
      },
    ]
  }

  return rest.map((shelf) => ({
    label: shelf.name,
    command: () => {
      void fileOn(shelf.id)
    },
  }))
})

const load = async (id: string) => {
  if (!id) return
  await Promise.all([stats.load(id), stats.subscribe()])
}

onMounted(async () => {
  chartOptions.value = setChartOptions()

  if (!books.loaded) await books.load()
  if (!documents.loaded) await documents.load()
  if (!devices.loaded) await devices.load()
  if (!collections.loaded) await collections.load()
  await load(bookId.value)
})

watch(bookId, load)

onUnmounted(() => stats.clear())
</script>

<template>
  <div class="flex flex-col gap-6">
    <div>
      <RouterLink
        to="/library"
        class="text-surface-500 dark:text-surface-400 hover:underline inline-flex items-center gap-2"
      >
        <i class="pi pi-arrow-left text-xs"></i>
        <span>Library</span>
      </RouterLink>
    </div>

    <Message v-if="books.loaded && !book" severity="warn">
      That book is not in your library.
    </Message>

    <template v-else-if="book">
      <Card
        class="bg-surface-0 dark:bg-surface-900 border border-surface-200 dark:border-surface-700"
      >
        <template #content>
          <div class="flex flex-col sm:flex-row gap-6">
            <div
              class="w-40 shrink-0 aspect-[2/3] rounded-lg overflow-hidden bg-surface-100 dark:bg-surface-800 border border-surface-200 dark:border-surface-700"
            >
              <img
                v-if="book.cover"
                :src="coverUrl"
                :alt="`Cover of ${book.title}`"
                class="w-full h-full object-cover"
              />
              <div
                v-else
                class="w-full h-full flex items-center justify-center text-surface-400 dark:text-surface-500"
              >
                <i class="pi pi-book text-4xl"></i>
              </div>
            </div>

            <div class="flex flex-col gap-3 min-w-0">
              <div>
                <h1 class="text-2xl font-semibold">{{ book.title }}</h1>
                <p v-if="authors" class="text-surface-600 dark:text-surface-400">{{ authors }}</p>
              </div>

              <div v-if="progress !== null" class="max-w-md">
                <div class="flex justify-between text-sm mb-1">
                  <span>{{ percent === 100 ? 'Finished' : 'Reading' }}</span>
                  <span class="tabular-nums">{{ percent }}%</span>
                </div>
                <ProgressBar :value="percent" :show-value="false" style="height: 0.5rem" />
              </div>

              <dl class="grid grid-cols-2 gap-x-6 gap-y-1 text-sm">
                <template v-if="pages">
                  <dt class="text-surface-500 dark:text-surface-400">Pages</dt>
                  <dd class="tabular-nums">
                    {{ formatCount(pages.count) }}
                    <span class="text-surface-500 dark:text-surface-400">({{ pages.note }})</span>
                  </dd>
                </template>
                <dt class="text-surface-500 dark:text-surface-400">Words</dt>
                <dd class="tabular-nums">{{ formatCount(book.word_count) }}</dd>
                <template v-if="book.language">
                  <dt class="text-surface-500 dark:text-surface-400">Language</dt>
                  <dd>{{ book.language }}</dd>
                </template>
              </dl>

              <!--
                Not PrimeVue's own removable chip: that one hides itself the
                moment it is clicked, so a request the server refuses would take
                the shelf off the screen and nowhere else.
              -->
              <div v-if="onShelves.length" class="flex flex-wrap items-center gap-2">
                <span
                  v-for="shelf in onShelves"
                  :key="shelf.id"
                  class="inline-flex items-center gap-1 pl-3 pr-1 py-1 rounded-full text-sm bg-surface-100 dark:bg-surface-800"
                >
                  <RouterLink
                    :to="{ name: 'collection', params: { id: shelf.id } }"
                    class="hover:underline"
                  >
                    {{ shelf.name }}
                  </RouterLink>
                  <Button
                    icon="pi pi-times"
                    variant="text"
                    rounded
                    size="small"
                    :title="`Take off ${shelf.name}`"
                    :aria-label="`Take off ${shelf.name}`"
                    @click="takeOff(shelf.id)"
                  />
                </span>
              </div>

              <div class="flex flex-wrap gap-2">
                <a :href="downloadUrl" :download="`${book.title}.epub`">
                  <Button icon="pi pi-download" label="Download" variant="outlined" size="small" />
                </a>
                <Button
                  icon="pi pi-bookmark"
                  label="Add to collection"
                  variant="outlined"
                  size="small"
                  aria-haspopup="true"
                  aria-controls="book-shelves"
                  @click="shelfMenu?.toggle($event)"
                />
                <Menu id="book-shelves" ref="shelfMenu" :model="shelfItems" :popup="true" />
              </div>
            </div>
          </div>
        </template>
      </Card>

      <div class="grid grid-cols-2 lg:grid-cols-4 gap-4">
        <Card
          class="bg-surface-0 dark:bg-surface-900 border border-surface-200 dark:border-surface-700"
        >
          <template #content>
            <span class="block text-surface-500 dark:text-surface-400 text-sm mb-1"
              >Time spent</span
            >
            <span class="text-2xl font-bold">{{ formatDuration(stats.totals.readingTime) }}</span>
          </template>
        </Card>
        <Card
          class="bg-surface-0 dark:bg-surface-900 border border-surface-200 dark:border-surface-700"
        >
          <template #content>
            <span class="block text-surface-500 dark:text-surface-400 text-sm mb-1"
              >Pages read</span
            >
            <span class="text-2xl font-bold tabular-nums">{{
              formatCount(stats.totals.pagesRead)
            }}</span>
          </template>
        </Card>
        <Card
          class="bg-surface-0 dark:bg-surface-900 border border-surface-200 dark:border-surface-700"
        >
          <template #content>
            <span class="block text-surface-500 dark:text-surface-400 text-sm mb-1">Days read</span>
            <span class="text-2xl font-bold tabular-nums">{{ stats.totals.days }}</span>
          </template>
        </Card>
        <Card
          class="bg-surface-0 dark:bg-surface-900 border border-surface-200 dark:border-surface-700"
        >
          <template #content>
            <span class="block text-surface-500 dark:text-surface-400 text-sm mb-1">Best day</span>
            <span v-if="stats.bestDay" class="text-2xl font-bold">{{
              formatDuration(stats.bestDay.reading_time)
            }}</span>
            <span v-else class="text-2xl font-bold">—</span>
            <span
              v-if="stats.bestDay"
              class="block text-surface-500 dark:text-surface-400 text-sm mt-1"
              >{{ stats.bestDay.date }}</span
            >
          </template>
        </Card>
      </div>

      <Card
        class="bg-surface-0 dark:bg-surface-900 border border-surface-200 dark:border-surface-700"
      >
        <template #title>
          <span class="text-xl font-semibold">Days read</span>
        </template>
        <template #content>
          <p v-if="stats.totals.first" class="mb-4 text-surface-600 dark:text-surface-400 text-sm">
            Read between {{ stats.totals.first }} and {{ stats.totals.last }}. The time is estimated
            from the gaps between your device's progress reports, and counts only what falls inside
            this book, so switching between books belongs to neither.
          </p>

          <div class="h-64">
            <Chart
              v-if="chartData"
              type="bar"
              :data="chartData"
              :options="chartOptions"
              class="h-full w-full"
            />
            <div
              v-else
              class="flex items-center justify-center h-full text-center text-surface-500 dark:text-surface-400"
            >
              <span v-if="stats.loading">Loading…</span>
              <span v-else>
                No reading recorded for this book yet. It appears here once a device pushes progress
                for the file you uploaded.
              </span>
            </div>
          </div>
        </template>
      </Card>
    </template>
  </div>
</template>
