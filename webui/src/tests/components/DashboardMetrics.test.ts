//
// File:        webui/src/tests/components/DashboardMetrics.test.ts
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { createTestingPinia } from '@pinia/testing'
import PrimeVue from 'primevue/config'
import Card from 'primevue/card'
import DashboardMetrics from '@/components/DashboardMetrics.vue'
import { useDocumentsStore } from '@/stores/documents'
import { useStatsStore } from '@/stores/stats'
import type { DocumentWithHistory, ReadingDay } from '@/models'

vi.mock('@/pb', async () => {
  const mock = await import('../mocks/pb')
  const actual = await vi.importActual<typeof import('@/pb')>('@/pb')

  return {
    pb: mock.pbMock,
    Collections: actual.Collections,
    KosyncApi: actual.KosyncApi,
    errorMessage: actual.errorMessage,
    browserTimezone: actual.browserTimezone,
    timezoneNames: actual.timezoneNames,
  }
})

function mountMetrics(documents: Partial<DocumentWithHistory>[], days: Partial<ReadingDay>[]) {
  const wrapper = mount(DashboardMetrics, {
    global: {
      plugins: [createTestingPinia({ createSpy: vi.fn }), PrimeVue],
      components: { Card },
    },
  })

  useDocumentsStore().documents = documents as DocumentWithHistory[]
  useStatsStore().days = days as ReadingDay[]

  return wrapper
}

describe('DashboardMetrics', () => {
  it('averages the progress of every document', async () => {
    const wrapper = mountMetrics([{ progress: 0.2 }, { progress: 0.8 }], [])
    await wrapper.vm.$nextTick()

    expect(wrapper.text()).toContain('50.0%')
    expect(wrapper.text()).toContain('2')
  })

  it('shows zero progress without documents', async () => {
    const wrapper = mountMetrics([], [])
    await wrapper.vm.$nextTick()

    expect(wrapper.text()).toContain('0.0%')
  })

  it('reports the reading time in whole minutes', async () => {
    const wrapper = mountMetrics([], [{ reading_time: 600 }, { reading_time: 330 }])
    await wrapper.vm.$nextTick()

    // 930 seconds is 15.5 minutes, rounded to 16.
    expect(wrapper.text()).toContain('16 min')
  })
})
