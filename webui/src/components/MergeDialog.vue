<!--
  File:        webui/src/components/MergeDialog.vue
  Project:     https://git.obth.eu/atjontv/kosync
  Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
-->
<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useToast } from 'primevue/usetoast'
import { useDocumentsStore } from '@/stores/documents'
import { useDevicesStore } from '@/stores/devices'
import type { DocumentWithHistory } from '@/models'
import { errorMessage } from '@/pb'

const props = defineProps<{
  /** The document that is kept. Everything picked here is folded into it. */
  document: DocumentWithHistory | null
}>()

const visible = defineModel<boolean>('visible', { default: false })

const store = useDocumentsStore()
const devices = useDevicesStore()
const toast = useToast()

const picked = ref<string[]>([])
const merging = ref(false)

/**
 * Every other document of this account.
 *
 * All of them, not only the ones without a book: the split this exists for
 * usually has one half matched to an uploaded EPUB and the other half not, and
 * hiding either half would hide the very pair a person came here to join.
 */
const candidates = computed(() =>
  store.documents
    .filter((entry) => entry.id !== props.document?.id)
    .slice()
    .sort((a, b) => b.last_read_at.localeCompare(a.last_read_at)),
)

const nameOf = (entry: DocumentWithHistory) => entry.title || entry.document
const deviceOf = (entry: DocumentWithHistory) =>
  devices.nameOf(entry.last_device_id) || entry.last_device || 'unknown device'
const percentOf = (entry: DocumentWithHistory) =>
  `${Number((entry.progress || 0) * 100).toFixed(1)}%`
const formatDate = (value: string) => (value ? new Date(value).toLocaleDateString() : '')

// Reset on every opening: a selection left over from the last time would be
// invisible above the fold and merged without being seen.
watch(visible, (open) => {
  if (open) picked.value = []
})

const submit = async () => {
  if (!props.document || picked.value.length === 0) return

  merging.value = true
  try {
    const message = await store.merge(props.document.id, picked.value)
    toast.add({ severity: 'success', summary: 'Documents merged', detail: message, life: 5000 })
    visible.value = false
  } catch (e) {
    toast.add({
      severity: 'error',
      summary: 'Could not merge',
      detail: errorMessage(e),
      life: 5000,
    })
  } finally {
    merging.value = false
  }
}
</script>

<template>
  <Dialog
    v-model:visible="visible"
    header="Merge documents"
    modal
    :breakpoints="{ '960px': '75vw', '640px': '90vw' }"
    :style="{ width: '46rem' }"
  >
    <div v-if="document" class="flex flex-col gap-4">
      <p class="text-surface-600 dark:text-surface-400">
        The same book read from two different copies of the file is two documents here, because
        KOReader knows a book by its contents. Merging joins them: the reading of everything you
        pick moves into
        <strong>{{ nameOf(document) }}</strong
        >, and the devices that reported them carry on syncing against it.
      </p>

      <div>
        <h3 class="font-semibold mb-2">Merge into this document</h3>
        <div
          class="flex justify-between items-center gap-3 p-3 rounded border border-surface-200 dark:border-surface-700"
        >
          <span class="min-w-0 truncate">{{ nameOf(document) }}</span>
          <span class="shrink-0 text-sm text-surface-500 dark:text-surface-400 tabular-nums">
            {{ percentOf(document) }} · {{ deviceOf(document) }}
          </span>
        </div>
      </div>

      <div>
        <h3 class="font-semibold mb-2">Documents to fold in</h3>

        <p
          v-if="candidates.length === 0"
          class="p-3 text-surface-500 dark:text-surface-400 border border-dashed border-surface-300 dark:border-surface-600 rounded"
        >
          There is nothing else to merge. A merge needs a second document.
        </p>

        <div v-else class="flex flex-col gap-2 max-h-72 overflow-y-auto">
          <label
            v-for="entry in candidates"
            :key="entry.id"
            class="flex items-center gap-3 p-3 rounded border border-surface-200 dark:border-surface-700 cursor-pointer"
          >
            <Checkbox v-model="picked" :value="entry.id" />
            <span class="flex flex-col min-w-0 grow">
              <span class="truncate">{{ nameOf(entry) }}</span>
              <span class="text-xs text-surface-500 dark:text-surface-400">
                {{ percentOf(entry) }} · {{ deviceOf(entry) }} ·
                {{ formatDate(entry.last_read_at) }}
              </span>
            </span>
            <Tag v-if="!entry.book" value="Not in library" severity="warn" class="shrink-0" />
          </label>
        </div>
      </div>

      <Message severity="info" :closable="false">
        The most recent position wins. Every position it replaces is kept in the history of the
        merged document, so nothing is lost and an unwanted merge can be undone by restoring it.
      </Message>
    </div>

    <template #footer>
      <Button label="Cancel" severity="secondary" outlined @click="visible = false" />
      <Button
        label="Merge"
        icon="pi pi-link"
        :disabled="picked.length === 0 || merging"
        :loading="merging"
        @click="submit"
      />
    </template>
  </Dialog>
</template>
