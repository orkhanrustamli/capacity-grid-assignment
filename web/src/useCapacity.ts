import { keepPreviousData, useInfiniteQuery, useMutation, useQueryClient, type InfiniteData, type QueryClient } from '@tanstack/react-query'
import { ApiError, fetchCapacity, updateCapacity, type CapacityPage, type PersonCapacity } from './api'
import type { IsoDate } from './dates'
import type { Range } from './urlRange'

export const capacityKeys = {
  all: ['capacity'] as const,
  range: (r: Range, overOnly: boolean) => ['capacity', r.from, r.to, overOnly] as const,
}

type GridData = InfiniteData<CapacityPage, string | undefined>

/**
 * The grid's pages for one range. Each range (and filter) is its own cache
 * entry, so a slow response for an old range lands in that entry and never
 * replaces the one on screen. While a new range loads, the previous grid
 * stays as placeholder data (isPlaceholderData) instead of a blank table.
 */
export function useCapacityGrid(range: Range, overOnly: boolean) {
  return useInfiniteQuery({
    queryKey: capacityKeys.range(range, overOnly),
    queryFn: ({ pageParam, signal }) => fetchCapacity({ ...range, overOnly, cursor: pageParam }, signal),
    initialPageParam: undefined as string | undefined,
    getNextPageParam: (last) => last.next_cursor ?? undefined,
    placeholderData: keepPreviousData,
  })
}

/** One person's row in one cached grid: the range and filter the edit started in. */
export type RowTarget = { personId: number; range: Range; overOnly: boolean }

export type SaveVariables = RowTarget & {
  weekStart: IsoDate
  weeklyHours: number
  expected: number // the capacity the manager saw for that week
}

export type SaveResult = { saved: PersonCapacity; row: CapacityPage }

/** The save went through, but re-reading the row did not: the numbers on screen are old. */
export class RefreshFailedError extends Error {
  readonly saved: PersonCapacity

  constructor(saved: PersonCapacity) {
    super("Saved — couldn't refresh this row.")
    this.name = 'RefreshFailedError'
    this.saved = saved
  }
}

/**
 * Saves a capacity change, then re-reads that person's row from the server
 * and writes it into the grid it was edited in (ADR 0005). No number is
 * computed on the client: status and utilization stay server rules.
 *
 * The range travels in the variables, captured when Save was clicked, so a
 * save that returns after the manager moved on still updates the right grid.
 * Cache writes live here, not in mutate() callbacks, because these run even
 * when the editing cell has unmounted.
 */
export function useUpdateCapacity() {
  const queryClient = useQueryClient()
  return useMutation<SaveResult, ApiError | RefreshFailedError, SaveVariables>({
    mutationFn: async (v) => {
      const saved = await updateCapacity(v.personId, {
        weekly_hours: v.weeklyHours,
        effective_from: v.weekStart,
        expected_weekly_hours: v.expected,
      })
      try {
        return { saved, row: await fetchRow(v) }
      } catch {
        throw new RefreshFailedError(saved)
      }
    },
    onSuccess: ({ row }, v) => {
      writeRow(queryClient, v, row)
      markOtherRangesStale(queryClient)
    },
    onError: async (err, v) => {
      if (err instanceof RefreshFailedError) {
        markOtherRangesStale(queryClient)
      } else if (err instanceof ApiError && err.code === 'capacity_conflict') {
        // Show the value that won; if this re-read fails too, the conflict
        // message still names it.
        await fetchRow(v).then((row) => writeRow(queryClient, v, row), () => undefined)
      }
    },
  })
}

/** Re-reads one row after a save whose own refresh failed. */
export function useRefreshRow() {
  const queryClient = useQueryClient()
  return useMutation<CapacityPage, ApiError, RowTarget>({
    mutationFn: fetchRow,
    onSuccess: (row, v) => writeRow(queryClient, v, row),
  })
}

function fetchRow(t: RowTarget): Promise<CapacityPage> {
  return fetchCapacity({ from: t.range.from, to: t.range.to, personIds: [t.personId] })
}

/**
 * Replaces the person in whichever loaded page holds them, and the first
 * page's over_count with the roster-wide count the row request carries.
 */
function writeRow(queryClient: QueryClient, t: RowTarget, row: CapacityPage) {
  const person = row.people.find((p) => p.id === t.personId)
  queryClient.setQueryData<GridData>(capacityKeys.range(t.range, t.overOnly), (data) =>
    data
      ? {
          ...data,
          pages: data.pages.map((page, i) => ({
            ...page,
            people: person ? page.people.map((p) => (p.id === person.id ? person : p)) : page.people,
            over_count: i === 0 && row.over_count !== undefined ? row.over_count : page.over_count,
          })),
        }
      : data,
  )
}

/**
 * Every cached grid may hold this person's old numbers. Mark them all stale
 * without refetching: each refetches when it is next shown. The grid on
 * screen was just patched, so it is not refetched now either.
 */
function markOtherRangesStale(queryClient: QueryClient) {
  void queryClient.invalidateQueries({ queryKey: capacityKeys.all, refetchType: 'none' })
}
