<!--
  File:        webui/src/components/TopBar.vue
  Project:     https://git.obth.eu/atjontv/kosync
  Copyright:   © 2025-2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
-->
<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { useDocumentsStore } from '@/stores/documents'
import { useStatsStore } from '@/stores/stats'
import { useKoreaderStore } from '@/stores/koreader'

const auth = useAuthStore()
const documents = useDocumentsStore()
const stats = useStatsStore()
const koreader = useKoreaderStore()
const router = useRouter()

const isDarkMode = ref(false)

const initTheme = () => {
  const savedTheme = localStorage.getItem('theme')
  if (
    savedTheme === 'dark' ||
    (!savedTheme && window.matchMedia('(prefers-color-scheme: dark)').matches)
  ) {
    document.documentElement.classList.add('p-dark')
    isDarkMode.value = true
  } else {
    document.documentElement.classList.remove('p-dark')
    isDarkMode.value = false
  }
}

initTheme()

const toggleTheme = () => {
  const root = document.documentElement
  if (root.classList.contains('p-dark')) {
    root.classList.remove('p-dark')
    localStorage.setItem('theme', 'light')
    isDarkMode.value = false
  } else {
    root.classList.add('p-dark')
    localStorage.setItem('theme', 'dark')
    isDarkMode.value = true
  }
}

const doLogout = async () => {
  auth.logout()
  documents.clear()
  stats.clear()
  koreader.clear()
  await router.push({ name: 'home' })
}
</script>

<template>
  <div
    class="flex justify-between items-center px-6 py-4 border-b border-surface-200 dark:border-surface-700 bg-surface-0 dark:bg-surface-900"
  >
    <div class="flex items-center gap-2">
      <RouterLink :to="{ name: 'home' }" class="text-2xl font-bold no-underline text-inherit">
        KOsync
      </RouterLink>
    </div>
    <div class="flex items-center gap-3">
      <Button
        :icon="isDarkMode ? 'pi pi-sun' : 'pi pi-moon'"
        variant="text"
        rounded
        title="Toggle Theme"
        aria-label="Toggle Theme"
        @click="toggleTheme"
      />
      <template v-if="auth.isValid">
        <RouterLink :to="{ name: 'account' }">
          <Button icon="pi pi-user" variant="text" rounded title="Account" aria-label="Account" />
        </RouterLink>
        <span class="hidden sm:inline text-surface-600 dark:text-surface-400">
          {{ auth.displayName }}
        </span>
        <Button label="Logout" @click="doLogout" />
      </template>
    </div>
  </div>
</template>
