import { useMemo, useState } from 'react'
import type { PersonRow } from './api'
import { CapacityCell } from './CapacityCell'
import { formatWeekLabel, type IsoDate } from './dates'
import { capacityKeys, useCapacityGrid, useRefreshRow, type RowTarget } from './useCapacity'
import type { Range } from './urlRange'

type Props = {
  from: IsoDate
  to: IsoDate
}

const SKELETON_ROWS = 6

/**
 * People × weeks. The range is a prop (owned by the page, in the URL); the
 * grid owns only its own view state — the over-only filter and which rows
 * were saved but could not be re-read.
 */
export function CapacityGrid({ from, to }: Props) {
  const [overOnly, setOverOnly] = useState(false)
  const range = useMemo(() => ({ from, to }), [from, to])
  const query = useCapacityGrid(range, overOnly)
  const staleRows = useStaleRows(range, overOnly)
  const refreshRow = useRefreshRow()

  const filter = (
    <label className="grid-toolbar__filter">
      <input type="checkbox" checked={overOnly} onChange={(e) => setOverOnly(e.target.checked)} />
      Only over-allocated
    </label>
  )

  if (query.isPending) {
    return (
      <section className="grid-section">
        <div className="grid-toolbar">{filter}</div>
        <div className="grid-skeleton" role="status" aria-label="Loading capacity">
          {Array.from({ length: SKELETON_ROWS }, (_, i) => (
            <div key={i} className="grid-skeleton__row" />
          ))}
        </div>
      </section>
    )
  }

  if (query.isError && !query.data) {
    return (
      <section className="grid-section">
        <div className="grid-toolbar">{filter}</div>
        <LoadError message={query.error.message} onRetry={() => query.refetch()} />
      </section>
    )
  }

  const pages = query.data.pages
  const { weeks, over_count: overCount } = pages[0]
  const people = pages.flatMap((p) => p.people)
  const stale = query.isPlaceholderData

  return (
    <section className="grid-section">
      <div className="grid-toolbar">
        {overCount !== undefined && (
          <p className={overCount > 0 ? 'over-count over-count--some' : 'over-count'}>
            {overCount === 1 ? '1 person' : `${overCount} people`} over-allocated
          </p>
        )}
        {filter}
      </div>
      {query.isError && (
        <LoadError
          message={query.error.message}
          onRetry={() => (query.isFetchNextPageError ? query.fetchNextPage() : query.refetch())}
        />
      )}
      <div className="grid-scroll">
        <table className={stale ? 'grid grid--stale' : 'grid'} aria-busy={stale}>
          <thead>
            <tr>
              <th scope="col" className="grid__corner">
                Person
              </th>
              {weeks.map((week) => (
                <th key={week} scope="col">
                  {formatWeekLabel(week)}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {people.map((person) => (
              <GridRow
                key={person.id}
                person={person}
                // Editing is off while the previous range is shown as a placeholder:
                // its cells belong to a range that is no longer the one requested.
                editTarget={stale ? null : { personId: person.id, range, overOnly }}
                notRefreshed={staleRows.has(person.id)}
                retrying={refreshRow.isPending && refreshRow.variables?.personId === person.id}
                onRefreshFailed={() => staleRows.add(person.id)}
                onRetryRefresh={(target) => refreshRow.mutate(target, { onSuccess: () => staleRows.remove(person.id) })}
              />
            ))}
          </tbody>
        </table>
        {people.length === 0 && (
          <p className="grid-empty">{overOnly ? 'No one is over-allocated in these weeks.' : 'No people to show.'}</p>
        )}
      </div>
      {query.hasNextPage && !stale && (
        <button type="button" className="load-more" onClick={() => query.fetchNextPage()} disabled={query.isFetchingNextPage}>
          {query.isFetchingNextPage ? 'Loading…' : 'Load more'}
        </button>
      )}
    </section>
  )
}

type GridRowProps = {
  person: PersonRow
  editTarget: RowTarget | null
  notRefreshed: boolean
  retrying: boolean
  onRefreshFailed: () => void
  onRetryRefresh: (target: RowTarget) => void
}

/**
 * A row whose save went through but whose re-read failed is dimmed and says
 * so: its numbers are from before the edit and must not pass for current.
 */
function GridRow({ person, editTarget, notRefreshed, retrying, onRefreshFailed, onRetryRefresh }: GridRowProps) {
  return (
    <tr data-stale={notRefreshed || undefined}>
      <th scope="row" className="grid__name">
        {person.name}
        {notRefreshed && editTarget && (
          <span className="row-stale" role="alert">
            Saved — couldn't refresh this row.{' '}
            <button type="button" disabled={retrying} onClick={() => onRetryRefresh(editTarget)}>
              {retrying ? 'Retrying…' : 'Retry'}
            </button>
          </span>
        )}
      </th>
      {person.weeks.map((cell) => (
        <CapacityCell key={cell.week} personName={person.name} cell={cell} editTarget={editTarget} onRefreshFailed={onRefreshFailed} />
      ))}
    </tr>
  )
}

/**
 * Person ids saved but not re-read, for the grid currently shown. Another
 * range or filter is a different cache entry, fetched fresh when shown, so
 * the marks belong to one range and are dropped when it changes.
 */
function useStaleRows(range: Range, overOnly: boolean) {
  const gridKey = capacityKeys.range(range, overOnly).join('|')
  const [marks, setMarks] = useState<{ gridKey: string; ids: ReadonlySet<number> }>({ gridKey, ids: new Set() })
  const ids = marks.gridKey === gridKey ? marks.ids : new Set<number>()
  const update = (fn: (ids: Set<number>) => void) =>
    setMarks((m) => {
      const next = new Set(m.gridKey === gridKey ? m.ids : [])
      fn(next)
      return { gridKey, ids: next }
    })
  return {
    has: (id: number) => ids.has(id),
    add: (id: number) => update((s) => s.add(id)),
    remove: (id: number) => update((s) => s.delete(id)),
  }
}

function LoadError({ message, onRetry }: { message: string; onRetry: () => void }) {
  return (
    <div className="load-error" role="alert">
      <p>
        <strong>Couldn't load capacity</strong> — {message}
      </p>
      <button type="button" onClick={onRetry}>
        Retry
      </button>
    </div>
  )
}
