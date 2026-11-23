import { describe, expect, it, vi, afterEach } from 'vitest'
import { fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import userEvent, { type UserEvent } from '@testing-library/user-event'
import { QueryClientProvider } from '@tanstack/react-query'
import { RangeControls } from './RangeControls'
import { App } from './App'
import type { CapacityPage } from './api'
import { deeAt36, deeAt48, golden, otherRange, page, savedDee } from './test/fixtures'
import {
  deferred,
  errorBody,
  gridLoads,
  isRowRefresh,
  json,
  patches,
  renderGrid,
  rowLoads,
  stubFetch,
  testQueryClient,
} from './test/helpers'

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
    const user = userEvent.setup()
    const calls = stubFetch((url) =>
      url.searchParams.get('cursor') === 'c1'
        ? json(page(golden.slice(2), { over_count: undefined }))
        : json(page(golden.slice(0, 2), { next_cursor: 'c1' })),
    )
    renderGrid()

    await user.click(await screen.findByRole('button', { name: 'Load more' }))

    expect(await screen.findByText('Dee Okafor')).toBeInTheDocument()
    expect(screen.getByText('Ana Ferreira')).toBeInTheDocument()
    expect(calls[1].url).toContain('cursor=c1')
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
    const user = userEvent.setup()
    let attempt = 0
    stubFetch(() => {
      attempt++
      if (attempt === 1) throw new TypeError('Failed to fetch')
      return json(page(golden))
    })
    renderGrid()

    expect(await screen.findByText("Couldn't load capacity")).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Retry' }))
    expect(await screen.findByText('Ana Ferreira')).toBeInTheDocument()
  })
})

/** Opens Dee's Jan 5 editor and types a value; returns the cell. */
async function editDeeJan5(user: UserEvent, value: string) {
  const cell = await screen.findByLabelText(/^Dee Okafor, week of Jan 5:/)
  await user.click(within(cell).getByRole('button', { name: /edit weekly hours/i }))
  const input = within(cell).getByLabelText('Weekly hours from week of Jan 5')
  await user.clear(input)
  await user.type(input, value)
  return cell
}

async function clickSave(user: UserEvent, cell: HTMLElement) {
  await user.click(within(cell).getByRole('button', { name: 'Save' }))
}

describe('editing capacity', () => {
  it('saving updates the row without reloading the grid', async () => {
    const user = userEvent.setup()
    const calls = stubFetch((url, init) => {
      if (init.method === 'PATCH') return json(savedDee)
      if (isRowRefresh(url)) return json(page([deeAt48], { over_count: 1 }))
      return json(page(golden))
    })
    renderGrid()

    const cell = await editDeeJan5(user, '48')
    await clickSave(user, cell)

    await waitFor(() => expect(cell).toHaveTextContent('45 / 48 h'))
    expect(cell).toHaveClass('cell--under')
    expect(screen.getByText('1 person over-allocated')).toBeInTheDocument()
    expect(gridLoads(calls)).toHaveLength(1)
    expect(rowLoads(calls)[0].url).toContain('person_id=4')
    expect(patches(calls)[0]).toMatchObject({
      url: '/api/people/4',
      body: { weekly_hours: 48, effective_from: '2026-01-05', expected_weekly_hours: 40 },
    })
    expect(within(cell).queryByRole('button', { name: 'Save' })).not.toBeInTheDocument()
  })

  it('refresh targets the range the save started in', async () => {
    const user = userEvent.setup()
    const patch = deferred<Response>()
    const calls = stubFetch((url, init) => {
      if (init.method === 'PATCH') return patch.promise
      if (isRowRefresh(url)) return json(page([deeAt48]))
      return json(url.searchParams.get('from') === '2025-12-29' ? page(golden) : otherRange)
    })
    const { setRange, client } = renderGrid()

    await clickSave(user, await editDeeJan5(user, '48'))
    setRange('2026-01-19', '2026-01-25')
    await screen.findByText('Zed Quinn')
    patch.resolve(json(savedDee))

    await waitFor(() => expect(rowLoads(calls)).toHaveLength(1))
    expect(rowLoads(calls)[0].url).toContain('from=2025-12-29&to=2026-01-16')
    await waitFor(() => {
      const cached = client.getQueryData<{ pages: CapacityPage[] }>(['capacity', '2025-12-29', '2026-01-16', false])
      expect(cached?.pages[0].people.find((p) => p.id === 4)?.weeks[1].capacity_hours).toBe(48)
    })
    expect(screen.getByText('Zed Quinn')).toBeInTheDocument()
  })

  it('a failed save keeps the old numbers and offers Retry', async () => {
    const user = userEvent.setup()
    let attempt = 0
    const calls = stubFetch((url, init) => {
      if (init.method === 'PATCH') return ++attempt === 1 ? errorBody(500, 'internal', 'internal error') : json(savedDee)
      if (isRowRefresh(url)) return json(page([deeAt48]))
      return json(page(golden))
    })
    renderGrid()

    const cell = await editDeeJan5(user, '48')
    await clickSave(user, cell)

    expect(await within(cell).findByText("Couldn't save. Your change wasn't applied.")).toBeInTheDocument()
    expect(cell).toHaveTextContent('45 / 40 h')
    await user.click(within(cell).getByRole('button', { name: 'Retry' }))
    await waitFor(() => expect(cell).toHaveTextContent('45 / 48 h'))
    expect(patches(calls)).toHaveLength(2)
    expect(patches(calls)[1].body).toEqual(patches(calls)[0].body)
  })

  it('a validation error is shown next to the input', async () => {
    const user = userEvent.setup()
    stubFetch((_url, init) =>
      init.method === 'PATCH'
        ? errorBody(422, 'invalid_weekly_hours', 'weekly_hours must be between 0 and 168')
        : json(page(golden)),
    )
    renderGrid()

    const cell = await editDeeJan5(user, '200')
    await clickSave(user, cell)

    expect(await within(cell).findByText('weekly_hours must be between 0 and 168')).toBeInTheDocument()
    expect(within(cell).getByLabelText('Weekly hours from week of Jan 5')).toHaveValue(200)
  })

  it('a conflict reloads the row and says who won', async () => {
    const user = userEvent.setup()
    const calls = stubFetch((url, init) => {
      if (init.method === 'PATCH') return errorBody(409, 'capacity_conflict', 'capacity changed', { current_weekly_hours: 36 })
      if (isRowRefresh(url)) return json(page([deeAt36]))
      return json(page(golden))
    })
    renderGrid()

    const cell = await editDeeJan5(user, '48')
    await clickSave(user, cell)

    expect(await within(cell).findByText('Changed by someone else — now 36 h.')).toBeInTheDocument()
    expect(rowLoads(calls)).toHaveLength(1)
    expect(cell).toHaveTextContent('45 / 36 h')
  })

  it('saved but refresh failed is never shown as current', async () => {
    const user = userEvent.setup()
    let rowAttempt = 0
    stubFetch((url, init) => {
      if (init.method === 'PATCH') return json(savedDee)
      if (isRowRefresh(url)) return ++rowAttempt === 1 ? errorBody(500, 'internal', 'internal error') : json(page([deeAt48]))
      return json(page(golden))
    })
    renderGrid()

    const cell = await editDeeJan5(user, '48')
    await clickSave(user, cell)

    const deeRow = cell.closest('tr')!
    expect(await within(deeRow).findByText("Saved — couldn't refresh this row.")).toBeInTheDocument()
    expect(deeRow).toHaveAttribute('data-stale', 'true')
    expect(cell).toHaveTextContent('45 / 40 h')

    await user.click(within(deeRow).getByRole('button', { name: 'Retry' }))
    await waitFor(() => expect(cell).toHaveTextContent('45 / 48 h'))
    expect(deeRow).not.toHaveAttribute('data-stale')
    expect(within(deeRow).queryByText("Saved — couldn't refresh this row.")).not.toBeInTheDocument()
  })

  it('shows a pending state while saving', async () => {
    const user = userEvent.setup()
    const patch = deferred<Response>()
    stubFetch((_url, init) => (init.method === 'PATCH' ? patch.promise : json(page(golden))))
    renderGrid()

    const cell = await editDeeJan5(user, '48')
    await clickSave(user, cell)

    const saving = await within(cell).findByRole('button', { name: 'Saving…' })
    expect(saving).toBeDisabled()
  })

  it('a Load more in flight when the save lands does not bring back the old numbers', async () => {
    const user = userEvent.setup()
    // The server's state: Dee's capacity changes when the PATCH is applied.
    let dee = golden[3]
    const pageTwo = deferred<Response>()
    stubFetch((url, init) => {
      if (init.method === 'PATCH') {
        dee = deeAt48
        return json(savedDee)
      }
      if (isRowRefresh(url)) return json(page([dee]))
      if (url.searchParams.get('cursor') === 'c1') return pageTwo.promise
      return json(page([...golden.slice(0, 3), dee], { next_cursor: 'c1' }))
    })
    renderGrid()

    await user.click(await screen.findByRole('button', { name: 'Load more' }))
    const cell = await editDeeJan5(user, '48')
    await clickSave(user, cell)
    await waitFor(() => expect(within(cell).queryByRole('button', { name: /Sav/ })).not.toBeInTheDocument())
    pageTwo.resolve(json(page([golden[4]], { over_count: undefined })))

    await new Promise((r) => setTimeout(r, 20))
    expect(screen.getByLabelText(/^Dee Okafor, week of Jan 5:/)).toHaveTextContent('45 / 48 h')
  })

  it('cannot be cancelled, and the range cannot change, while a save is in flight', async () => {
    const user = userEvent.setup()
    window.history.replaceState(null, '', '/?from=2025-12-29&to=2026-01-16')
    const patch = deferred<Response>()
    stubFetch((_url, init) => (init.method === 'PATCH' ? patch.promise : json(page(golden))))
    const client = testQueryClient()
    render(
      <QueryClientProvider client={client}>
        <App />
      </QueryClientProvider>,
    )

    const cell = await editDeeJan5(user, '48')
    await clickSave(user, cell)
    await within(cell).findByRole('button', { name: 'Saving…' })

    expect(within(cell).getByRole('button', { name: 'Cancel' })).toBeDisabled()
    await user.type(within(cell).getByLabelText('Weekly hours from week of Jan 5'), '{Escape}')
    expect(within(cell).getByLabelText('Weekly hours from week of Jan 5')).toBeInTheDocument()
    for (const name of ['← Week', 'This week', 'Week →', 'Demo range']) {
      expect(screen.getByRole('button', { name })).toBeDisabled()
    }
    expect(screen.getByLabelText('From')).toBeDisabled()
    expect(screen.getByRole('checkbox', { name: 'Only over-allocated' })).toBeDisabled()
  })

  it('a save whose outcome is unknown says so and offers Retry', async () => {
    const user = userEvent.setup()
    let attempt = 0
    stubFetch((url, init) => {
      if (init.method === 'PATCH' && ++attempt === 1) throw new TypeError('Failed to fetch')
      if (init.method === 'PATCH') return json(savedDee)
      if (isRowRefresh(url)) return json(page([deeAt48]))
      return json(page(golden))
    })
    renderGrid()

    const cell = await editDeeJan5(user, '48')
    await clickSave(user, cell)

    expect(await within(cell).findByText("Couldn't reach the server — your change may not have been saved.")).toBeInTheDocument()
    await user.click(within(cell).getByRole('button', { name: 'Retry' }))
    await waitFor(() => expect(cell).toHaveTextContent('45 / 48 h'))
  })

  it('Escape cancels without saving', async () => {
    const user = userEvent.setup()
    const calls = stubFetch(() => json(page(golden)))
    renderGrid()

    const cell = await editDeeJan5(user, '48')
    await user.keyboard('{Escape}')

    expect(within(cell).queryByLabelText('Weekly hours from week of Jan 5')).not.toBeInTheDocument()
    expect(patches(calls)).toHaveLength(0)
  })
})

describe('RangeControls', () => {
  const range = { from: '2025-12-29', to: '2026-01-18' }

  // The date inputs keep fireEvent.change: jsdom has no date picker and
  // user-event cannot type into a date input, so a change event carrying the
  // picked date is the closest stand-in for what the browser sends.

  it('moves the whole range by a week', async () => {
    const user = userEvent.setup()
    const onChange = vi.fn()
    render(<RangeControls range={range} onChange={onChange} />)
    await user.click(screen.getByRole('button', { name: 'Week →' }))
    expect(onChange).toHaveBeenCalledWith({ from: '2026-01-05', to: '2026-01-25' })
    await user.click(screen.getByRole('button', { name: '← Week' }))
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
