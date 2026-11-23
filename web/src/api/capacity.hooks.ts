import {
  hashKey,
  keepPreviousData,
  useInfiniteQuery,
  useIsMutating,
  useMutation,
  useQueryClient,
  type InfiniteData,
  type QueryClient,
} from '@tanstack/react-query'
import { ApiError, fetchCapacity, updateCapacity, type CapacityPage, type PersonCapacity } from './capacity.api'
import type { IsoDate } from '../utils/dates'
import type { Range } from '../utils/urlRange'
import { capacityKeys } from './capacity.keys'

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

const saveCapacityKey = ['saveCapacity'] as const

/** True while any capacity save is in flight; the range and filter stay put until it settles. */
export function useSaveInProgress(): boolean {
  return useIsMutating({ mutationKey: saveCapacityKey }) > 0
}

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
 * Cache writes and onRefreshFailed live here, not in mutate() callbacks,
 * because these run even when the editing cell has unmounted.
 */
export function useUpdateCapacity({ onRefreshFailed }: { onRefreshFailed?: (v: SaveVariables) => void } = {}) {
  const queryClient = useQueryClient()
  return useMutation<SaveResult, ApiError | RefreshFailedError, SaveVariables>({
    mutationKey: saveCapacityKey,
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
    onSuccess: ({ row }, v) => applyRow(queryClient, v, row),
    onError: async (err, v) => {
      if (err instanceof RefreshFailedError) {
        onRefreshFailed?.(v)
        refreshOtherGrids(queryClient, v)
      } else if (err instanceof ApiError && err.code === 'capacity_conflict') {
        // Show the value that won; if this re-read fails too, the conflict
        // message still names it.
        await fetchRow(v).then((row) => applyRow(queryClient, v, row), () => undefined)
      }
    },
  })
}

/** Re-reads one row after a save whose own refresh failed. */
export function useRefreshRow() {
  const queryClient = useQueryClient()
  return useMutation<CapacityPage, ApiError, RowTarget>({
    mutationFn: fetchRow,
    onSuccess: (row, v) => applyRow(queryClient, v, row),
  })
}

function fetchRow(t: RowTarget): Promise<CapacityPage> {
  return fetchCapacity({ from: t.range.from, to: t.range.to, personIds: [t.personId] })
}

/**
 * Puts a freshly read row into the grid it was edited in, and makes every
 * other grid that may hold the person's old numbers refetch.
 *
 * A fetch of that grid already in flight (Load more, a refetch) was started
 * before the save and would write the old row back when it lands, so in that
 * case the grid is refetched instead of patched. The refetch cancels the
 * in-flight request; a Load more caught by it has to be clicked again.
 */
async function applyRow(queryClient: QueryClient, t: RowTarget, row: CapacityPage) {
  const key = capacityKeys.range(t.range, t.overOnly)
  refreshOtherGrids(queryClient, t)
  if (queryClient.isFetching({ queryKey: key, exact: true }) > 0) {
    await queryClient.invalidateQueries({ queryKey: key, exact: true })
    return
  }
  writeRow(queryClient, t, row)
  // Over-only membership is not patched (ADR 0005); stale, so it is fresh next time it is shown.
  void queryClient.invalidateQueries({ queryKey: key, exact: true, refetchType: 'none' })
}

/**
 * Every other cached grid may hold the edited person's old numbers. One on
 * screen (a second grid, or a range reached while the save was in flight)
 * refetches now; the rest refetch when next shown.
 */
function refreshOtherGrids(queryClient: QueryClient, t: RowTarget) {
  const edited = hashKey(capacityKeys.range(t.range, t.overOnly))
  void queryClient.invalidateQueries({
    queryKey: capacityKeys.all,
    predicate: (q) => hashKey(q.queryKey) !== edited,
    refetchType: 'active',
  })
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
