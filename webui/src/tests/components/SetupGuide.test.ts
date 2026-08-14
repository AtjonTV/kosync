//
// File:        webui/src/tests/components/SetupGuide.test.ts
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import PrimeVue from 'primevue/config'
import Card from 'primevue/card'
import SetupGuide from '@/components/SetupGuide.vue'

vi.mock('@/pb', async () => {
  const mock = await import('../mocks/pb')
  const actual = await vi.importActual<typeof import('@/pb')>('@/pb')

  return {
    pb: mock.pbMock,
    Collections: actual.Collections,
    KosyncApi: actual.KosyncApi,
    errorMessage: actual.errorMessage,
  }
})

describe('SetupGuide', () => {
  it('points KOReader at the /koreader prefix', () => {
    const wrapper = mount(SetupGuide, {
      global: { plugins: [PrimeVue], components: { Card } },
    })

    // Getting this wrong is the single most likely reason a device fails to
    // sync, so it is worth a test.
    expect(wrapper.text()).toContain('http://localhost/koreader')
  })

  // The catalog is a standard other readers speak, so it does not sit under the
  // /koreader prefix, and an address off by that one segment finds nothing.
  it('points the OPDS catalog at /opds', () => {
    const wrapper = mount(SetupGuide, {
      global: { plugins: [PrimeVue], components: { Card } },
    })

    expect(wrapper.text()).toContain('http://localhost/opds')
  })

  it('tells the reader that registration happens in the web interface', () => {
    const wrapper = mount(SetupGuide, {
      global: { plugins: [PrimeVue], components: { Card } },
    })

    expect(wrapper.text()).toContain('not on the device')
  })
})
