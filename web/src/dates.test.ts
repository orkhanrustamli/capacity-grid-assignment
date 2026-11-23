import { describe, expect, it } from 'vitest'
import { addDays, addWeeks, formatWeekLabel, isIsoDate, todayIso, weekEndOf, weekStartOf } from './dates'
import { defaultRange, readRange } from './urlRange'

// npm test runs under TZ=America/Los_Angeles: a viewer west of UTC is where a
// local-time Date shifts a calendar date back by one day.
it('runs in a non-UTC time zone', () => {
  expect(new Date().getTimezoneOffset()).not.toBe(0)
})

describe('weekStartOf', () => {
  it.each([
    ['2026-01-07', '2026-01-05'],
    ['2026-01-05', '2026-01-05'],
    ['2026-01-11', '2026-01-05'],
    ['2025-12-31', '2025-12-29'],
    ['2026-03-11', '2026-03-09'], // US daylight saving starts 2026-03-08
    ['2026-11-04', '2026-11-02'], // and ends 2026-11-01
  ])('%s → %s', (d, monday) => {
    expect(weekStartOf(d)).toBe(monday)
  })
})

describe('week arithmetic', () => {
  it.each([
    ['2026-03-02', 1, '2026-03-09'],
    ['2026-10-26', 1, '2026-11-02'],
    ['2026-12-28', 1, '2027-01-04'],
    ['2026-01-05', -1, '2025-12-29'],
  ])('addWeeks(%s, %i) = %s', (d, n, want) => {
    expect(addWeeks(d, n)).toBe(want)
  })

  it('weekEndOf is the Sunday', () => {
    expect(weekEndOf('2026-01-07')).toBe('2026-01-11')
    expect(weekEndOf('2026-03-04')).toBe('2026-03-08')
  })

  it('addDays crosses a month and a year', () => {
    expect(addDays('2026-12-31', 1)).toBe('2027-01-01')
    expect(addDays('2026-03-01', -1)).toBe('2026-02-28')
  })
})

it('labels a week by its Monday, not the day before', () => {
  expect(formatWeekLabel('2026-01-05')).toBe('Jan 5')
})

it('todayIso uses the viewer’s local calendar date', () => {
  // 23:30 in Los Angeles is already the 24th in UTC.
  expect(todayIso(new Date(2026, 10, 23, 23, 30))).toBe('2026-11-23')
})

it('isIsoDate accepts only real calendar dates', () => {
  expect(isIsoDate('2026-02-28')).toBe(true)
  expect(isIsoDate('2026-02-30')).toBe(false)
  expect(isIsoDate('2026-1-5')).toBe(false)
  expect(isIsoDate('bogus')).toBe(false)
})

describe('readRange', () => {
  it('keeps a valid range from the URL', () => {
    expect(readRange('?from=2025-12-29&to=2026-01-16', '2026-11-23')).toEqual({ from: '2025-12-29', to: '2026-01-16' })
  })

  it('falls back to eight weeks from this Monday on an invalid date', () => {
    expect(readRange('?from=bogus', '2026-11-23')).toEqual({ from: '2026-11-23', to: '2027-01-17' })
  })

  it('falls back when from is after to', () => {
    expect(readRange('?from=2026-02-01&to=2026-01-01', '2026-11-25')).toEqual(defaultRange('2026-11-25'))
  })

  it('default range starts on the Monday of today', () => {
    expect(defaultRange('2026-11-25')).toEqual({ from: '2026-11-23', to: '2027-01-17' })
  })
})
