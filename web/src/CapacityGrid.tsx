import { useMemo, useState } from 'react'
import { CapacityCell } from './CapacityCell'
import { formatWeekLabel, type IsoDate } from './dates'
import { useCapacityGrid } from './useCapacity'

type Props = {
  from: IsoDate
  to: IsoDate
}

const SKELETON_ROWS = 6

/**
 * People × weeks. The range is a prop (owned by the page, in the URL); the
 * grid owns only its own view state — the over-only filter.
 */
export function CapacityGrid({ from, to }: Props) {
  const [overOnly, setOverOnly] = useState(false)
  const range = useMemo(() => ({ from, to }), [from, to])
  const query = useCapacityGrid(range, overOnly)

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
              <tr key={person.id}>
                <th scope="row" className="grid__name">
                  {person.name}
                </th>
                {person.weeks.map((cell) => (
                  <CapacityCell key={cell.week} personName={person.name} cell={cell} />
                ))}
              </tr>
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
