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
</script>

<template>
  <Card class="bg-surface-0 dark:bg-surface-900 border border-surface-200 dark:border-surface-700">
    <template #title>
      <span class="text-2xl font-bold">How to set up KOReader sync</span>
    </template>
    <template #content>
      <ol class="list-decimal list-inside space-y-4 text-surface-700 dark:text-surface-300">
        <li><strong>Register</strong> an account above. Registration happens here, not on the device.</li>
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
    </template>
  </Card>
</template>
