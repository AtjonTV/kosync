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

  /**
   * Sets a new account password.
   *
   * Changing the password invalidates every session of the account, this one
   * included, so the store signs back in right away with the new password
   * instead of dropping the user onto the login form.
   */
  async function changePassword(oldPassword: string, newPassword: string): Promise<void> {
    const id = record.value?.id
    const identity = email.value
    if (!id) throw new Error('You are not signed in.')

    await pb.collection(Collections.users).update(id, {
      oldPassword,
      password: newPassword,
      passwordConfirm: newPassword,
    })

    await login(identity, newPassword)
  }

  /**
   * Asks for the account address to be changed.
   *
   * PocketBase sends a confirmation link to the *new* address and only applies
   * the change once that link is opened, which is what makes this safe to offer
   * to the placeholder addresses the legacy import creates.
   */
  async function requestEmailChange(newEmail: string): Promise<void> {
    await pb.collection(Collections.users).requestEmailChange(newEmail)
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
    changePassword,
    requestEmailChange,
    logout,
    refresh,
    errorMessage,
  }
})
