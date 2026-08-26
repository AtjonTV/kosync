//
// File:        webui/src/lib/duration.ts
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

/**
 * A duration in seconds as hours and minutes: "33 h 30 min".
 *
 * Minutes on their own stop meaning anything once they run into the hundreds.
 * A month of reading is four figures of them, and "2010 min" has to be divided
 * in the head before it can be read as an amount of time.
 *
 * Under an hour the hours are left off rather than written as a zero, because
 * "45 min" is what somebody would say and "0 h 45 min" is not.
 *
 * The reading time is measured in seconds everywhere it is stored, so that is
 * what this takes; the rounding to whole minutes happens once, here.
 */
export function formatDuration(seconds: number): string {
  const total = Number.isFinite(seconds) && seconds > 0 ? seconds : 0
  const minutes = Math.round(total / 60)

  if (minutes < 60) return `${minutes} min`

  return `${Math.floor(minutes / 60)} h ${minutes % 60} min`
}
