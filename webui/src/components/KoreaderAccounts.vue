<!--
  File:        webui/src/components/KoreaderAccounts.vue
  Project:     https://git.obth.eu/atjontv/kosync
  Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
-->
<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useConfirm } from 'primevue/useconfirm'
import { useToast } from 'primevue/usetoast'
import { useKoreaderStore } from '@/stores/koreader'
import type { KoreaderAccount } from '@/models'
import { errorMessage } from '@/pb'

const koreader = useKoreaderStore()
const confirm = useConfirm()
const toast = useToast()

const showCreate = ref(false)
const showRotate = ref(false)
const rotateTarget = ref<KoreaderAccount | null>(null)

const form = ref({ username: '', password: '', label: '' })
const newPassword = ref('')
const error = ref('')
const busy = ref(false)

// The plain password exists only in the browser, once. The server stores the
// MD5 digest that KOReader sends and can never hand it back.
const lastCreated = ref<{ username: string; password: string } | null>(null)

const formatDateTime = (value: string) => (value ? new Date(value).toLocaleString() : 'never')

const openCreate = () => {
  form.value = { username: '', password: '', label: '' }
  error.value = ''
  showCreate.value = true
}

const create = async () => {
  error.value = ''
  busy.value = true
  try {
    await koreader.create(form.value.username, form.value.password, form.value.label)
    lastCreated.value = { username: form.value.username, password: form.value.password }
    showCreate.value = false
  } catch (e) {
    error.value = errorMessage(e, 'Could not create the credential.')
  } finally {
    busy.value = false
  }
}

const openRotate = (account: KoreaderAccount) => {
  rotateTarget.value = account
  newPassword.value = ''
  error.value = ''
  showRotate.value = true
}

const rotate = async () => {
  if (!rotateTarget.value) return

  error.value = ''
  busy.value = true
  try {
    await koreader.changePassword(rotateTarget.value.id, newPassword.value)
    lastCreated.value = { username: rotateTarget.value.username, password: newPassword.value }
    showRotate.value = false
    toast.add({
      severity: 'success',
      summary: 'Password changed',
      detail: 'Sign in again on every device that used this credential.',
      life: 6000,
    })
  } catch (e) {
    error.value = errorMessage(e, 'Could not change the password.')
  } finally {
    busy.value = false
  }
}

const toggleDisabled = async (account: KoreaderAccount) => {
  try {
    await koreader.setDisabled(account.id, !account.disabled)
  } catch (e) {
    toast.add({ severity: 'error', summary: 'Failed', detail: errorMessage(e), life: 5000 })
  }
}

const remove = (account: KoreaderAccount) => {
  confirm.require({
    message: `Delete the credential "${account.username}"? Devices using it stop syncing, the reading progress they pushed is kept.`,
    header: 'Confirmation',
    icon: 'pi pi-exclamation-triangle',
    rejectProps: { label: 'Cancel', severity: 'secondary', outlined: true },
    acceptProps: { label: 'Delete', severity: 'danger' },
    accept: async () => {
      try {
        await koreader.remove(account.id)
      } catch (e) {
        toast.add({ severity: 'error', summary: 'Failed', detail: errorMessage(e), life: 5000 })
      }
    },
  })
}

onMounted(() => {
  koreader.load()
})
</script>

<template>
  <Card class="bg-surface-0 dark:bg-surface-900 border border-surface-200 dark:border-surface-700">
    <template #title>
      <div class="flex justify-between items-center">
        <span class="text-xl font-semibold">KOReader credentials</span>
        <Button label="Add credential" icon="pi pi-plus" size="small" @click="openCreate" />
      </div>
    </template>
    <template #content>
      <p class="mb-4 text-surface-600 dark:text-surface-400">
        These are the username and password you enter in KOReader. They are separate from your
        account password, because KOReader can only protect them with MD5.
      </p>

      <Message v-if="lastCreated" severity="warn" class="mb-4">
        <div class="flex flex-col gap-1">
          <span>
            Write this down now, it is shown only once:
            <strong>{{ lastCreated.username }}</strong> /
            <strong>{{ lastCreated.password }}</strong>
          </span>
          <Button
            label="I noted it down"
            size="small"
            variant="text"
            @click="lastCreated = null"
          />
        </div>
      </Message>

      <DataTable :value="koreader.accounts" data-key="id" :loading="koreader.loading">
        <Column field="username" header="Username" :sortable="true"></Column>
        <Column field="label" header="Device"></Column>
        <Column field="last_used" header="Last used" :sortable="true">
          <template #body="{ data }">{{ formatDateTime(data.last_used) }}</template>
        </Column>
        <Column header="Status">
          <template #body="{ data }">
            <Tag
              :value="data.disabled ? 'Disabled' : 'Active'"
              :severity="data.disabled ? 'danger' : 'success'"
            />
          </template>
        </Column>
        <Column header="Actions" style="width: 12rem">
          <template #body="{ data }">
            <div class="flex gap-2">
              <Button
                icon="pi pi-key"
                variant="text"
                rounded
                title="Change password"
                @click="openRotate(data)"
              />
              <Button
                :icon="data.disabled ? 'pi pi-play' : 'pi pi-pause'"
                variant="text"
                rounded
                :title="data.disabled ? 'Enable' : 'Disable'"
                @click="toggleDisabled(data)"
              />
              <Button
                icon="pi pi-trash"
                severity="danger"
                variant="text"
                rounded
                title="Delete"
                @click="remove(data)"
              />
            </div>
          </template>
        </Column>
        <template #empty>
          <div class="p-4 text-center text-surface-500 dark:text-surface-400">
            No credentials yet. Add one to connect a device.
          </div>
        </template>
      </DataTable>
    </template>
  </Card>

  <Dialog
    v-model:visible="showCreate"
    header="Add a KOReader credential"
    modal
    :style="{ width: '28rem' }"
  >
    <form class="flex flex-col gap-4" @submit.prevent="create">
      <div class="flex flex-col gap-2">
        <label for="ko-username">Username</label>
        <InputText id="ko-username" v-model="form.username" autofocus fluid />
      </div>
      <div class="flex flex-col gap-2">
        <label for="ko-password">Password</label>
        <Password id="ko-password" v-model="form.password" :feedback="false" toggle-mask fluid />
      </div>
      <div class="flex flex-col gap-2">
        <label for="ko-label">Device (optional)</label>
        <InputText id="ko-label" v-model="form.label" placeholder="Kobo Clara" fluid />
      </div>
      <Message v-if="error" severity="error" variant="simple">{{ error }}</Message>
      <div class="flex justify-end gap-2">
        <Button
          type="button"
          label="Cancel"
          severity="secondary"
          @click="showCreate = false"
        />
        <Button type="submit" label="Create" :loading="busy" />
      </div>
    </form>
  </Dialog>

  <Dialog
    v-model:visible="showRotate"
    header="Change the KOReader password"
    modal
    :style="{ width: '28rem' }"
  >
    <form class="flex flex-col gap-4" @submit.prevent="rotate">
      <div class="flex flex-col gap-2">
        <label for="ko-new-password">New password</label>
        <Password id="ko-new-password" v-model="newPassword" :feedback="false" toggle-mask fluid />
      </div>
      <Message v-if="error" severity="error" variant="simple">{{ error }}</Message>
      <div class="flex justify-end gap-2">
        <Button
          type="button"
          label="Cancel"
          severity="secondary"
          @click="showRotate = false"
        />
        <Button type="submit" label="Change" :loading="busy" />
      </div>
    </form>
  </Dialog>
</template>
