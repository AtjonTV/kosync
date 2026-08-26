//
// File:        webui/src/tests/lib/duration.test.ts
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

import { describe, expect, it } from 'vitest'
import { formatDuration } from '@/lib/duration'

describe('formatDuration', () => {
  // The number this exists for: a month of reading, which nobody can read as
  // an amount of time while it is written in minutes.
  it('breaks a long total into hours and minutes', () => {
    expect(formatDuration(2010 * 60)).toBe('33 h 30 min')
  })

  // "0 h 45 min" is not what anybody would say.
  it('leaves the hours off below an hour', () => {
    expect(formatDuration(45 * 60)).toBe('45 min')
  })

  it('says an exact hour with no minutes left over', () => {
    expect(formatDuration(3600)).toBe('1 h 0 min')
  })

  it('rounds to the nearest minute', () => {
    expect(formatDuration(930)).toBe('16 min')
    expect(formatDuration(3599)).toBe('1 h 0 min')
  })

  it('says nothing at all as no minutes', () => {
    expect(formatDuration(0)).toBe('0 min')
  })

  // A duration, not a time of day: an account that has read for five days
  // solid has done 120 hours of it.
  it('does not wrap the hours at a day', () => {
    expect(formatDuration(7215 * 60)).toBe('120 h 15 min')
  })

  it('makes nothing of a total that came out as nothing sensible', () => {
    expect(formatDuration(Number.NaN)).toBe('0 min')
    expect(formatDuration(-5)).toBe('0 min')
  })
})
