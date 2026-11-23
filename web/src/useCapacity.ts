import { keepPreviousData, useInfiniteQuery } from '@tanstack/react-query'
import { fetchCapacity } from './api'
import type { Range } from './urlRange'

export const capacityKeys = {
  all: ['capacity'] as const,
  range: (r: Range, overOnly: boolean) => ['capacity', r.from, r.to, overOnly] as const,
}

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
