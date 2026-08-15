//
// File:        webui/src/tests/components/AchievementList.test.ts
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

import { beforeEach, describe, expect, it, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { setActivePinia, createPinia } from 'pinia'
import { createTestingPinia } from '@pinia/testing'
import PrimeVue from 'primevue/config'
import * as pbMockModule from '../mocks/pb'

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

import AchievementList from '@/components/AchievementList.vue'
import AchievementBadge from '@/components/AchievementBadge.vue'
import { useAchievementsStore } from '@/stores/achievements'
import type { Achievement } from '@/models'

function achievement(overrides: Partial<Achievement> = {}): Achievement {
  return {
    rule: 'lap-warmer',
    name: 'Lap Warmer',
    summary: 'Your longest run of days without missing one.',
    unit: 'days',
    icon: 'ach-streak',
    fur: 'cream',
    tiers: [7, 30, 100],
    value: 12,
    tier: 1,
    next: 30,
    earned: [{ tier: 1, value: 7, at: '2026-08-01 10:00:00.000Z' }],
    ...overrides,
  }
}

function mountList(achievements: Achievement[], loaded = true) {
  return mount(AchievementList, {
    global: {
      plugins: [
        createTestingPinia({
          createSpy: vi.fn,
          initialState: { achievements: { achievements, loaded } },
        }),
        PrimeVue,
      ],
    },
  })
}

describe('AchievementList', () => {
  it('names the tier that was earned', () => {
    const wrapper = mountList([achievement()])

    expect(wrapper.text()).toContain('Lap Warmer')
    expect(wrapper.text()).toContain('Bronze')
  })

  // A badge nobody has yet is the only thing on the card that says what there is
  // to aim at, so it is shown rather than hidden.
  it('shows what has not been earned', () => {
    const wrapper = mountList([
      achievement({ rule: 'night-prowler', name: 'Night Prowler', tier: 0, value: 0, next: 1 }),
    ])

    expect(wrapper.text()).toContain('Night Prowler')
    expect(wrapper.text()).toContain('Not yet')
    expect(wrapper.findComponent(AchievementBadge).props('tier')).toBe(0)
  })

  it('counts the distance to the next tier, not to zero', () => {
    const wrapper = mountList([achievement()])

    // 12 days with the bronze tier at 7 and silver at 30.
    expect(wrapper.text()).toContain('12 of 30 days')
  })

  it('says so when every tier is done', () => {
    const wrapper = mountList([achievement({ value: 140, tier: 3, next: 0 })])

    expect(wrapper.text()).toContain('every tier earned')
  })

  // Nothing loaded and nothing earned look the same from a distance, and only
  // one of them should say there is nothing to show.
  it('waits before saying there is nothing', () => {
    const wrapper = mountList([], false)

    expect(wrapper.text()).not.toContain('Nothing to show yet')
    expect(wrapper.find('.p-progressspinner').exists()).toBe(true)
  })

  it('puts the earned ones before the rest', () => {
    const wrapper = mountList([
      achievement({ rule: 'night-prowler', name: 'Night Prowler', tier: 0, earned: [] }),
      achievement(),
    ])
    const text = wrapper.text()

    expect(text.indexOf('Lap Warmer')).toBeLessThan(text.indexOf('Night Prowler'))
  })
})

describe('achievements store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    pbMockModule.reset()
  })

  it('asks the server for the rules as well as the progress', async () => {
    pbMockModule.send.mockResolvedValue({ achievements: [achievement()] })

    const store = useAchievementsStore()
    await store.load()

    expect(pbMockModule.send).toHaveBeenCalledWith('/api/kosync/achievements', { method: 'GET' })
    expect(store.achievements).toHaveLength(1)
    expect(store.earned).toHaveLength(1)
    expect(store.pending).toHaveLength(0)
  })

  // A rule with no rows arrives as null from a server that has not been updated,
  // and a computed that throws leaves the page on the last thing it drew.
  it('survives a rule that arrives without its list of tiers', async () => {
    const missing = { ...achievement({ tier: 0, value: 0 }), earned: null }
    pbMockModule.send.mockResolvedValue({ achievements: [missing] })

    const store = useAchievementsStore()
    await store.load()

    expect(store.achievements[0]?.earned).toEqual([])
    expect(() => store.earned).not.toThrow()
    expect(store.pending).toHaveLength(1)
  })

  it('reloads when a new tier is awarded', async () => {
    pbMockModule.send.mockResolvedValue({ achievements: [] })

    const store = useAchievementsStore()
    await store.load()
    await store.subscribe()
    pbMockModule.send.mockClear()

    pbMockModule.emit('achievements', 'create', { id: 'a1' })
    await flushPromises()

    expect(pbMockModule.send).toHaveBeenCalled()
  })
})
