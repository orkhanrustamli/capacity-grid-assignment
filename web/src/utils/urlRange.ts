import { useCallback, useEffect, useState } from 'react'
import { addWeeks, daysBetween, isIsoDate, todayIso, weekEndOf, weekStartOf, type IsoDate } from './dates'

export type Range = { from: IsoDate; to: IsoDate }

export const DEFAULT_WEEKS = 8

/** The longest range the API serves in one request (week.MaxRangeWeeks in api/week/week.go). */
export const MAX_RANGE_WEEKS = 53

/** The weeks the seed's hand-crafted people are built around. */
export const DEMO_RANGE: Range = { from: '2025-12-29', to: '2026-01-16' }

export function defaultRange(today: IsoDate): Range {
  const from = weekStartOf(today)
  return { from, to: weekEndOf(addWeeks(from, DEFAULT_WEEKS - 1)) }
}

/** How many ISO weeks the range touches, once widened to whole weeks. */
export function weeksIn(r: Range): number {
  return daysBetween(weekStartOf(r.from), weekStartOf(r.to)) / 7 + 1
}

/** Why a range cannot be loaded, or null when it can. Both dates must already be valid. */
export function rangeProblem(r: Range): string | null {
  if (r.from > r.to) return 'Start must be before end'
  if (weeksIn(r) > MAX_RANGE_WEEKS) return `A range covers at most ${MAX_RANGE_WEEKS} weeks`
  return null
}

/** The range in the URL's query string, or the default when it is missing or invalid. */
export function readRange(search: string, today: IsoDate): Range {
  const params = new URLSearchParams(search)
  const from = params.get('from') ?? ''
  const to = params.get('to') ?? ''
  if (!isIsoDate(from) || !isIsoDate(to) || rangeProblem({ from, to })) return defaultRange(today)
  return { from, to }
}

/**
 * The range lives in the URL so a view can be shared and the back button
 * steps through ranges. Setting it pushes a history entry.
 */
export function useUrlRange(): [Range, (r: Range) => void] {
  const [range, setRangeState] = useState(() => readRange(window.location.search, todayIso()))

  useEffect(() => {
    const onPopState = () => setRangeState(readRange(window.location.search, todayIso()))
    window.addEventListener('popstate', onPopState)
    return () => window.removeEventListener('popstate', onPopState)
  }, [])

  const setRange = useCallback((r: Range) => {
    const url = new URL(window.location.href)
    url.searchParams.set('from', r.from)
    url.searchParams.set('to', r.to)
    window.history.pushState(null, '', url)
    setRangeState(r)
  }, [])

  return [range, setRange]
}
