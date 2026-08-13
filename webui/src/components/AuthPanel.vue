<!--
  File:        webui/src/components/AuthPanel.vue
  Project:     https://git.obth.eu/atjontv/kosync
  Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
-->
<script setup lang="ts">
import { ref } from 'vue'
import { useAuthStore } from '@/stores/auth'
import { errorMessage } from '@/pb'

const auth = useAuthStore()

const mode = ref<'login' | 'register' | 'reset'>('login')
const email = ref('')
const password = ref('')
const name = ref('')
const error = ref('')
const notice = ref('')
const loading = ref(false)

const setMode = (next: 'login' | 'register' | 'reset') => {
  mode.value = next
  error.value = ''
  notice.value = ''
}

const submit = async () => {
  error.value = ''
  notice.value = ''

  if (!email.value || (mode.value !== 'reset' && !password.value)) {
    error.value = 'Please fill in every field.'
    return
  }

  loading.value = true
  try {
    if (mode.value === 'login') {
      await auth.login(email.value, password.value)
    } else if (mode.value === 'register') {
      await auth.register(email.value, password.value, name.value)
    } else {
      await auth.requestPasswordReset(email.value)
      notice.value = 'If that address has an account, a reset link is on its way.'
    }
    password.value = ''
  } catch (e) {
    error.value = errorMessage(e, 'Sign in failed. Please check your credentials.')
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <Card
    class="bg-surface-0 dark:bg-surface-900 border border-surface-200 dark:border-surface-700 shadow-sm"
  >
    <template #title>
      <span v-if="mode === 'login'">Sign in</span>
      <span v-else-if="mode === 'register'">Create an account</span>
      <span v-else>Reset your password</span>
    </template>
    <template #content>
      <form class="flex flex-col gap-4 max-w-md" @submit.prevent="submit">
        <div class="flex flex-col gap-2">
          <label for="email">Email</label>
          <InputText id="email" v-model="email" type="email" autocomplete="email" fluid />
        </div>

        <div v-if="mode === 'register'" class="flex flex-col gap-2">
          <label for="name">Display name (optional)</label>
          <InputText id="name" v-model="name" autocomplete="nickname" fluid />
        </div>

        <div v-if="mode !== 'reset'" class="flex flex-col gap-2">
          <label for="password">Password</label>
          <Password
            id="password"
            v-model="password"
            :feedback="mode === 'register'"
            toggle-mask
            fluid
          />
        </div>

        <Message v-if="error" severity="error" variant="simple">{{ error }}</Message>
        <Message v-if="notice" severity="success" variant="simple">{{ notice }}</Message>

        <div class="flex flex-wrap items-center gap-3">
          <Button
            type="submit"
            :loading="loading"
            :label="mode === 'login' ? 'Sign in' : mode === 'register' ? 'Register' : 'Send link'"
          />
          <Button
            v-if="mode !== 'login'"
            type="button"
            label="Back to sign in"
            variant="text"
            @click="setMode('login')"
          />
          <template v-else>
            <Button type="button" label="Register" variant="text" @click="setMode('register')" />
            <Button
              type="button"
              label="Forgot password"
              variant="text"
              @click="setMode('reset')"
            />
          </template>
        </div>
      </form>
    </template>
  </Card>
</template>
