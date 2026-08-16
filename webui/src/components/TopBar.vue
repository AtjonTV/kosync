<!--
  File:        webui/src/components/TopBar.vue
  Project:     https://git.obth.eu/atjontv/kosync
  Copyright:   © 2025-2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
-->
<script setup lang="ts">
import { computed, ref, useTemplateRef } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { useDocumentsStore } from '@/stores/documents'
import { useStatsStore } from '@/stores/stats'
import { useKoreaderStore } from '@/stores/koreader'
import { useBooksStore } from '@/stores/books'
import { useBookStatsStore } from '@/stores/bookStats'
import { useDevicesStore } from '@/stores/devices'
import { useCollectionsStore } from '@/stores/collections'
import type { MenuItem } from 'primevue/menuitem'

const auth = useAuthStore()
const documents = useDocumentsStore()
const stats = useStatsStore()
const koreader = useKoreaderStore()
const books = useBooksStore()
const bookStats = useBookStatsStore()
const devices = useDevicesStore()
const collections = useCollectionsStore()
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
  books.clear()
  bookStats.clear()
  devices.clear()
  collections.clear()
  await router.push({ name: 'home' })
}

// Navigation on the left, in the order things are reached for: the dashboard,
// then the library, then the shelves cut out of it, then the documents behind
// it all.
const navigation = [
  { label: 'Home', icon: 'pi pi-home', route: 'home' },
  { label: 'Library', icon: 'pi pi-book', route: 'library' },
  { label: 'Collections', icon: 'pi pi-bookmark', route: 'collections' },
  { label: 'Documents', icon: 'pi pi-file', route: 'documents' },
]

// Everything about the account collapses into one menu on the right, so the bar
// stops being a row of unrelated icons. The address is the menu's first item
// rather than a label beside it: it says whose account this is, which is the
// question the menu answers.
const accountMenu = useTemplateRef<{ toggle: (event: Event) => void }>('accountMenu')

const accountItems = computed<MenuItem[]>(() => [
  { label: auth.email || auth.displayName, disabled: true },
  { separator: true },
  {
    label: 'Account settings',
    icon: 'pi pi-cog',
    command: () => {
      void router.push({ name: 'account' })
    },
  },
  { separator: true },
  { label: 'Sign out', icon: 'pi pi-sign-out', command: doLogout },
])
</script>

<template>
  <div
    class="flex justify-between items-center px-6 py-4 border-b border-surface-200 dark:border-surface-700 bg-surface-0 dark:bg-surface-900"
  >
    <div class="flex items-center gap-1 sm:gap-4">
      <RouterLink :to="{ name: 'home' }" class="text-2xl font-bold no-underline text-inherit">
        KOsync
      </RouterLink>

      <nav v-if="auth.isValid" class="flex items-center gap-1">
        <RouterLink
          v-for="entry in navigation"
          :key="entry.route"
          :to="{ name: entry.route }"
          class="px-3 py-2 rounded-md no-underline text-surface-600 dark:text-surface-400 hover:bg-surface-100 dark:hover:bg-surface-800"
          active-class="text-surface-900 dark:text-surface-0 font-medium"
        >
          <i :class="entry.icon" class="sm:mr-2"></i>
          <span class="hidden sm:inline">{{ entry.label }}</span>
        </RouterLink>
      </nav>
    </div>

    <div class="flex items-center gap-2">
      <Button
        :icon="isDarkMode ? 'pi pi-sun' : 'pi pi-moon'"
        variant="text"
        rounded
        title="Toggle Theme"
        aria-label="Toggle Theme"
        @click="toggleTheme"
      />

      <template v-if="auth.isValid">
        <Button
          icon="pi pi-user"
          variant="text"
          rounded
          title="Account"
          aria-label="Account"
          aria-haspopup="true"
          aria-controls="account-menu"
          @click="accountMenu?.toggle($event)"
        />
        <Menu id="account-menu" ref="accountMenu" :model="accountItems" :popup="true" />
      </template>
    </div>
  </div>
</template>
