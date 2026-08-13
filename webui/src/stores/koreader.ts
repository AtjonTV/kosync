//
// File:        webui/src/stores/koreader.ts
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

import { ref } from 'vue'
import { defineStore } from 'pinia'
import { pb, Collections, KosyncApi } from '@/pb'
import type { KoreaderAccount } from '@/models'

/**
 * The KOReader credentials of the signed in account.
 *
 * Creating one and changing its password go through the KOsync API, because the
 * server has to store the password in the form KOReader will send it in. The
 * plain password is shown to the user exactly once, at creation time, and can
 * never be read back afterwards.
 */
export const useKoreaderStore = defineStore('koreader', () => {
  const accounts = ref<KoreaderAccount[]>([])
  const loading = ref(false)

  async function load(): Promise<void> {
    loading.value = true
    try {
      accounts.value = await pb
        .collection(Collections.koreaderAccounts)
        .getFullList<KoreaderAccount>({ sort: 'username' })
    } finally {
      loading.value = false
    }
  }

  async function create(username: string, password: string, label: string): Promise<void> {
    await pb.send(KosyncApi.koreaderAccounts, {
      method: 'POST',
      body: { username, password, label },
    })
    await load()
  }

  async function changePassword(id: string, password: string): Promise<void> {
    await pb.send(KosyncApi.koreaderAccountPassword(id), {
      method: 'POST',
      body: { password },
    })
  }

  async function setDisabled(id: string, disabled: boolean): Promise<void> {
    await pb.collection(Collections.koreaderAccounts).update(id, { disabled })
    await load()
  }

  async function setLabel(id: string, label: string): Promise<void> {
    await pb.collection(Collections.koreaderAccounts).update(id, { label })
    await load()
  }

  async function remove(id: string): Promise<void> {
    await pb.collection(Collections.koreaderAccounts).delete(id)
    await load()
  }

  function clear(): void {
    accounts.value = []
  }

  return { accounts, loading, load, create, changePassword, setDisabled, setLabel, remove, clear }
})
