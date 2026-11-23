import { describe, expect, it, vi, afterEach } from 'vitest'
import { fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { CapacityGrid } from './CapacityGrid'
import { RangeControls } from './RangeControls'
import type { CapacityPage, CapacityStatus, PersonRow } from './api'

// Fixtures are the golden values for seed people 1–5 over the demo range
// (api/capacity_golden_test.go), as the API returns them.
const WEEKS = ['2025-12-29', '2026-01-05', '2026-01-12']
type CellSpec = [allocated: number, capacity: number, weekend: number, status: CapacityStatus]

function row(id: number, name: string, cells: CellSpec[], weeks = WEEKS): PersonRow {
  return {
    id,
    name,
    weeks: cells.map(([allocated, capacity, weekend, status], i) => ({
      week: weeks[i],
      allocated_hours: allocated,
      capacity_hours: capacity,
      weekend_hours: weekend,
      utilization: capacity === 0 ? null : allocated / capacity,
      status,
      capacity_changed: false,
    })),
  }
}

const golden = [
  row(1, 'Ana Ferreira', [[40, 40, 16, 'full'], [0, 40, 0, 'under'], [30, 40, 0, 'under']]),
  row(2, 'Bo Lindqvist', [[0, 40, 0, 'under'], [32, 40, 16, 'under'], [8, 40, 0, 'under']]),
  row(3, 'Cem Aydin', [[0, 20, 0, 'under'], [4, 20, 0, 'under'], [12, 20, 0, 'under']]),
  row(4, 'Dee Okafor', [[0, 40, 0, 'under'], [45, 40, 0, 'over'], [40, 40, 0, 'full']]),
  row(5, 'Eli Nakamura', [[0, 0, 0, 'under'], [20, 0, 0, 'over'], [0, 0, 0, 'under']]),
]

function page(people: PersonRow[], extra: Partial<CapacityPage> = {}): CapacityPage {
  return { from: '2025-12-29', to: '2026-01-18', weeks: WEEKS, people, next_cursor: null, over_count: 2, ...extra }
}

const otherRange = page([row(9, 'Zed Quinn', [[8, 40, 0, 'under']], ['2026-01-19'])], {
  from: '2026-01-19',
  to: '2026-01-25',
  weeks: ['2026-01-19'],
  over_count: 0,
})

function json(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } })
}

function deferred<T>() {
  let resolve!: (v: T) => void
  const promise = new Promise<T>((r) => (resolve = r))
  return { promise, resolve }
}

/** Replaces fetch; returns the URLs requested, in order. */
function stubFetch(handler: (url: URL) => Response | Promise<Response>): string[] {
  const calls: string[] = []
  vi.stubGlobal(
    'fetch',
    vi.fn(async (input: RequestInfo | URL) => {
      const url = new URL(String(input), 'http://localhost')
      calls.push(url.pathname + url.search)
      return handler(url)
    }),
  )
  return calls
}

function renderGrid(from = '2025-12-29', to = '2026-01-16') {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  const ui = (f: string, t: string) => (
    <QueryClientProvider client={client}>
      <CapacityGrid from={f} to={t} />
    </QueryClientProvider>
  )
  const result = render(ui(from, to))
  return { ...result, client, setRange: (f: string, t: string) => result.rerender(ui(f, t)) }
}

afterEach(() => {
  vi.unstubAllGlobals()
})

describe('CapacityGrid', () => {
  it('renders one row per person and one column per week', async () => {
    stubFetch(() => json(page(golden)))
    renderGrid()

    expect(await screen.findByRole('columnheader', { name: 'Dec 29' })).toBeInTheDocument()
    expect(screen.getByRole('columnheader', { name: 'Jan 5' })).toBeInTheDocument()
    expect(screen.getByRole('columnheader', { name: 'Jan 12' })).toBeInTheDocument()
    expect(within(screen.getByRole('table')).getAllByRole('row')).toHaveLength(1 + golden.length)

    const dee = screen.getByLabelText('Dee Okafor, week of Jan 5: 45 of 40 hours, over by 5')
    expect(dee).toHaveClass('cell--over')
    expect(dee).toHaveTextContent('▲ +5 h')

    const ana = screen.getByLabelText(/^Ana Ferreira, week of Dec 29:/)
    expect(ana).toHaveClass('cell--full')
    expect(ana).toHaveTextContent('+16 h weekend')

    expect(screen.getByLabelText(/^Eli Nakamura, week of Jan 5:/)).toHaveTextContent('No capacity')
  })

  it('shows how many people are over-allocated', async () => {
    stubFetch(() => json(page(golden)))
    renderGrid()
    expect(await screen.findByText('2 people over-allocated')).toBeInTheDocument()
  })

  it('Load more appends the next page', async () => {
    const calls = stubFetch((url) =>
      url.searchParams.get('cursor') === 'c1'
        ? json(page(golden.slice(2), { over_count: undefined }))
        : json(page(golden.slice(0, 2), { next_cursor: 'c1' })),
    )
    renderGrid()

    fireEvent.click(await screen.findByRole('button', { name: 'Load more' }))

    expect(await screen.findByText('Dee Okafor')).toBeInTheDocument()
    expect(screen.getByText('Ana Ferreira')).toBeInTheDocument()
    expect(calls[1]).toContain('cursor=c1')
    expect(screen.queryByRole('button', { name: 'Load more' })).not.toBeInTheDocument()
  })

  it('a slow older range never replaces a newer one', async () => {
    const first = deferred<Response>()
    const second = deferred<Response>()
    stubFetch((url) => (url.searchParams.get('from') === '2025-12-29' ? first.promise : second.promise))
    const { setRange } = renderGrid()
    setRange('2026-01-19', '2026-01-25')

    second.resolve(json(otherRange))
    expect(await screen.findByRole('columnheader', { name: 'Jan 19' })).toBeInTheDocument()
    first.resolve(json(page(golden)))

    await new Promise((r) => setTimeout(r, 20))
    expect(screen.queryByRole('columnheader', { name: 'Dec 29' })).not.toBeInTheDocument()
    expect(screen.getByText('Zed Quinn')).toBeInTheDocument()
  })

  it('keeps the previous grid visible (dimmed) while a new range loads', async () => {
    const pending = deferred<Response>()
    stubFetch((url) => (url.searchParams.get('from') === '2025-12-29' ? json(page(golden)) : pending.promise))
    const { setRange } = renderGrid()
    await screen.findByText('Ana Ferreira')

    setRange('2026-01-19', '2026-01-25')

    await waitFor(() => expect(screen.getByRole('table')).toHaveAttribute('aria-busy', 'true'))
    expect(screen.getByText('Ana Ferreira')).toBeInTheDocument()
    pending.resolve(json(otherRange))
    expect(await screen.findByText('Zed Quinn')).toBeInTheDocument()
    expect(screen.getByRole('table')).toHaveAttribute('aria-busy', 'false')
  })

  it('shows an error with Retry when the load fails', async () => {
    let attempt = 0
    stubFetch(() => {
      attempt++
      if (attempt === 1) throw new TypeError('Failed to fetch')
      return json(page(golden))
    })
    renderGrid()

    expect(await screen.findByText("Couldn't load capacity")).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Retry' }))
    expect(await screen.findByText('Ana Ferreira')).toBeInTheDocument()
  })
})

describe('RangeControls', () => {
  const range = { from: '2025-12-29', to: '2026-01-18' }

  it('moves the whole range by a week', () => {
    const onChange = vi.fn()
    render(<RangeControls range={range} onChange={onChange} />)
    fireEvent.click(screen.getByRole('button', { name: 'Week →' }))
    expect(onChange).toHaveBeenCalledWith({ from: '2026-01-05', to: '2026-01-25' })
    fireEvent.click(screen.getByRole('button', { name: '← Week' }))
    expect(onChange).toHaveBeenLastCalledWith({ from: '2025-12-22', to: '2026-01-11' })
  })

  it('does not apply a start after the end', () => {
    const onChange = vi.fn()
    render(<RangeControls range={range} onChange={onChange} />)
    fireEvent.change(screen.getByLabelText('From'), { target: { value: '2026-02-01' } })
    expect(onChange).not.toHaveBeenCalled()
    expect(screen.getByText('Start must be before end')).toBeInTheDocument()
  })

  it('applies a valid date', () => {
    const onChange = vi.fn()
    render(<RangeControls range={range} onChange={onChange} />)
    fireEvent.change(screen.getByLabelText('To'), { target: { value: '2026-02-01' } })
    expect(onChange).toHaveBeenCalledWith({ from: '2025-12-29', to: '2026-02-01' })
  })
})
