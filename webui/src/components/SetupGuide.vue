<!--
  File:        webui/src/components/SetupGuide.vue
  Project:     https://git.obth.eu/atjontv/kosync
  Copyright:   © 2025-2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
-->
<script setup lang="ts">
import { computed } from 'vue'

// KOReader appends its own paths to whatever it is given, so the device has to
// be pointed at the "/koreader" prefix rather than at the site root.
const syncServer = computed(() => `${window.location.origin}/koreader`)

// The catalog is a standard other readers speak too, which is why it does not
// sit under the "/koreader" prefix.
const catalog = computed(() => `${window.location.origin}/opds`)

// KOReader's statistics plugin syncs through a WebDAV cloud storage entry, and
// the trailing slash is how that dialog expects a collection to be written.
const statistics = computed(() => `${window.location.origin}/webdav/`)
</script>

<template>
  <Card class="bg-surface-0 dark:bg-surface-900 border border-surface-200 dark:border-surface-700">
    <template #title>
      <span class="text-2xl font-bold">How to set up KOReader sync</span>
    </template>
    <template #content>
      <ol class="list-decimal list-inside space-y-4 text-surface-700 dark:text-surface-300">
        <li>
          <strong>Register</strong> an account above. Registration happens here, not on the device.
        </li>
        <li>
          Open <strong>Account &rarr; KOReader credentials</strong> and add a credential. Note the
          username and password, they are shown once.
        </li>
        <li>
          In KOReader, set <strong>Custom sync server</strong> to
          <code class="px-1 rounded bg-surface-100 dark:bg-surface-800">{{ syncServer }}</code>
        </li>
        <li><strong>Log in</strong> in KOReader with the credential from step 2.</li>
        <li>Enable <strong>automatically keep documents in sync</strong>.</li>
        <li>Set <strong>periodically sync every # pages</strong> to 2.</li>
        <li>Set <strong>Document matching method</strong> to "Binary".</li>
        <li>Repeat steps 3 to 7 on every device, using the same credential or a new one.</li>
        <li>Read books.</li>
      </ol>

      <div class="mt-6 pt-6 border-t border-surface-200 dark:border-surface-700">
        <p class="mb-3 font-semibold">Getting books onto a device</p>
        <p class="mb-3 text-surface-700 dark:text-surface-300">
          Your library is also an OPDS catalog. In KOReader, open
          <strong>Search &rarr; OPDS catalog</strong>, add a new catalog with the address
          <code class="px-1 rounded bg-surface-100 dark:bg-surface-800">{{ catalog }}</code>
          and the same credential from step 2. Books downloaded from there are recognised the moment
          you start reading them, so their progress and statistics arrive without an upload.
        </p>
        <p class="text-surface-700 dark:text-surface-300">
          The catalog opens on what you are reading now and what was added last, and then lets you
          browse by author, by series, by language, or by any collection you have put together
          yourself under <strong>Collections</strong>. A collection is served in the order you
          arranged it in, which is the point of making one.
        </p>
      </div>

      <div class="mt-6 pt-6 border-t border-surface-200 dark:border-surface-700">
        <p class="mb-3 font-semibold">Syncing your reading statistics</p>
        <p class="mb-3 text-surface-700 dark:text-surface-300">
          KOReader keeps its own record of every page you turn, and can sync that file to a WebDAV
          target. This server is one. In KOReader, add a cloud storage entry of type
          <strong>WebDAV</strong> with the address
          <code class="px-1 rounded bg-surface-100 dark:bg-surface-800">{{ statistics }}</code>
          and, again, the credential from step 2 — then point the statistics plugin's own cloud sync
          at that entry. The exact menu path moves between KOReader releases, so go by these names
          rather than by where they sat last time.
        </p>
        <p class="text-surface-700 dark:text-surface-300">
          Each account gets its own copy of the file and can see no other. Once a device has synced
          it, the pages and hours it recorded — including everything from before you had an account
          here — show up in your statistics.
        </p>
      </div>
    </template>
  </Card>
</template>
