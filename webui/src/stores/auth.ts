//
// File:        webui/src/stores/auth.ts
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

import { computed, ref } from 'vue'
import { defineStore } from 'pinia'
import { pb, Collections, errorMessage } from '@/pb'

/**
 * The signed in account.
 *
 * PocketBase already persists the session in localStorage and refreshes it, so
 * this store only mirrors its state into Vue's reactivity.
 */
export const useAuthStore = defineStore('auth', () => {
  const record = ref(pb.authStore.record)
  const isValid = ref(pb.authStore.isValid)

  pb.authStore.onChange(() => {
    record.value = pb.authStore.record
    isValid.value = pb.authStore.isValid
  })

  const email = computed(() => (record.value?.email as string | undefined) ?? '')
  const displayName = computed(() => (record.value?.name as string) || email.value)

  async function login(identity: string, password: string): Promise<void> {
    await pb.collection(Collections.users).authWithPassword(identity, password)
  }

  async function register(email: string, password: string, name: string): Promise<void> {
    await pb.collection(Collections.users).create({
      email,
      password,
      passwordConfirm: password,
      name,
    })
    await login(email, password)

    // Best effort: an instance without SMTP cannot send anything, and that must
    // not make a successful registration look like a failure.
    try {
      await pb.collection(Collections.users).requestVerification(email)
    } catch {
      // ignored on purpose
    }
  }

  async function requestPasswordReset(email: string): Promise<void> {
    await pb.collection(Collections.users).requestPasswordReset(email)
  }

  function logout(): void {
    pb.authStore.clear()
  }

  /** Confirms that the stored session is still accepted by the server. */
  async function refresh(): Promise<boolean> {
    if (!pb.authStore.isValid) return false

    try {
      await pb.collection(Collections.users).authRefresh()
      return true
    } catch {
      pb.authStore.clear()
      return false
    }
  }

  return {
    record,
    isValid,
    email,
    displayName,
    login,
    register,
    requestPasswordReset,
    logout,
    refresh,
    errorMessage,
  }
})
