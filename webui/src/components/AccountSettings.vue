<!--
  File:        webui/src/components/AccountSettings.vue
  Project:     https://git.obth.eu/atjontv/kosync
  Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
-->
<script setup lang="ts">
import { computed, ref } from 'vue'
import { useToast } from 'primevue/usetoast'
import { useAuthStore } from '@/stores/auth'
import { errorMessage } from '@/pb'

const auth = useAuthStore()
const toast = useToast()

const newEmail = ref('')
const emailError = ref('')
const emailNotice = ref('')
const emailBusy = ref(false)

const oldPassword = ref('')
const newPassword = ref('')
const passwordError = ref('')
const passwordBusy = ref(false)

const isVerified = computed(() => auth.record?.verified === true)

// The legacy import parks every account it creates on this domain, so it is
// worth pointing out rather than leaving it to be discovered.
const isPlaceholderEmail = computed(() => auth.email.endsWith('@invalid.local'))

const changeEmail = async () => {
  emailError.value = ''
  emailNotice.value = ''

  const address = newEmail.value.trim()
  if (!address) {
    emailError.value = 'Please enter the new address.'
    return
  }
  if (address === auth.email) {
    emailError.value = 'That is already your address.'
    return
  }

  emailBusy.value = true
  try {
    await auth.requestEmailChange(address)
    emailNotice.value = `A confirmation link is on its way to ${address}. The address changes once you open it.`
    newEmail.value = ''
  } catch (e) {
    // A server without mail configured answers this with a bare "Failed to
    // request email change", which tells the reader nothing about what to do.
    emailError.value =
      errorMessage(e, 'Could not request the address change.') +
      ' If this server cannot send mail, a superuser can change the address in the admin interface.'
  } finally {
    emailBusy.value = false
  }
}

const changePassword = async () => {
  passwordError.value = ''

  if (!oldPassword.value || !newPassword.value) {
    passwordError.value = 'Please fill in both fields.'
    return
  }

  passwordBusy.value = true
  try {
    await auth.changePassword(oldPassword.value, newPassword.value)
    oldPassword.value = ''
    newPassword.value = ''
    toast.add({
      severity: 'success',
      summary: 'Password changed',
      detail: 'Your other sessions were signed out.',
      life: 5000,
    })
  } catch (e) {
    passwordError.value = errorMessage(e, 'Could not change the password.')
  } finally {
    passwordBusy.value = false
  }
}
</script>

<template>
  <Card class="bg-surface-0 dark:bg-surface-900 border border-surface-200 dark:border-surface-700">
    <template #title>
      <span class="text-xl font-semibold">Sign in details</span>
    </template>
    <template #content>
      <p class="mb-4 text-surface-600 dark:text-surface-400">
        These are for the web interface. Your devices keep using their own KOReader credentials
        below, and are not affected by anything on this card.
      </p>

      <Message v-if="isPlaceholderEmail" severity="warn" class="mb-4">
        Your address was generated during the import from KOsync 1 and cannot receive mail. Change
        it to a real one so you can recover your account if you forget the password.
      </Message>
      <Message v-else-if="!isVerified" severity="info" class="mb-4">
        Your address is not confirmed yet.
      </Message>

      <div class="grid grid-cols-1 lg:grid-cols-2 gap-8">
        <form class="flex flex-col gap-4" @submit.prevent="changeEmail">
          <h3 class="text-lg font-semibold">Email address</h3>

          <div class="flex flex-col gap-2">
            <label for="current-email">Current</label>
            <InputText id="current-email" :model-value="auth.email" disabled fluid />
          </div>

          <div class="flex flex-col gap-2">
            <label for="new-email">New address</label>
            <InputText
              id="new-email"
              v-model="newEmail"
              type="email"
              autocomplete="email"
              placeholder="you@example.com"
              fluid
            />
          </div>

          <Message v-if="emailError" severity="error" variant="simple">{{ emailError }}</Message>
          <Message v-if="emailNotice" severity="success" variant="simple">{{
            emailNotice
          }}</Message>

          <p class="text-sm text-surface-500 dark:text-surface-400">
            A confirmation link goes to the new address, and nothing changes until you open it. This
            needs the server to be able to send mail; if it cannot, a superuser can change the
            address in the admin interface at <code>/_/</code>.
          </p>

          <div>
            <Button type="submit" label="Send confirmation" :loading="emailBusy" />
          </div>
        </form>

        <form class="flex flex-col gap-4" @submit.prevent="changePassword">
          <h3 class="text-lg font-semibold">Password</h3>

          <div class="flex flex-col gap-2">
            <label for="old-password">Current password</label>
            <Password
              id="old-password"
              v-model="oldPassword"
              :feedback="false"
              toggle-mask
              autocomplete="current-password"
              fluid
            />
          </div>

          <div class="flex flex-col gap-2">
            <label for="new-password">New password</label>
            <Password
              id="new-password"
              v-model="newPassword"
              toggle-mask
              autocomplete="new-password"
              fluid
            />
          </div>

          <Message v-if="passwordError" severity="error" variant="simple">
            {{ passwordError }}
          </Message>

          <p class="text-sm text-surface-500 dark:text-surface-400">
            Changing this signs out every other session. You stay signed in here.
          </p>

          <div>
            <Button type="submit" label="Change password" :loading="passwordBusy" />
          </div>
        </form>
      </div>
    </template>
  </Card>
</template>
