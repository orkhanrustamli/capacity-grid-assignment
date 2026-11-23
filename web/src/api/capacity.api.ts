import type { IsoDate } from '../utils/dates'

// Types mirror the API's JSON exactly (snake_case kept), so there is no
// mapping layer to drift out of step with the server.

export type CapacityStatus = 'over' | 'full' | 'under'

export type WeekCell = {
  week: IsoDate
  allocated_hours: number
  capacity_hours: number
  weekend_hours: number
  utilization: number | null // null when capacity is 0
  status: CapacityStatus
  capacity_changed: boolean
}

export type PersonRow = {
  id: number
  name: string
  weeks: WeekCell[]
}

export type CapacityPage = {
  from: IsoDate
  to: IsoDate
  weeks: IsoDate[]
  people: PersonRow[]
  next_cursor: string | null
  over_count?: number // first page and person_id requests only
}

export type CapacityChange = {
  effective_from: IsoDate | null // null = baseline
  weekly_hours: number
}

export type PersonCapacity = {
  id: number
  name: string
  weekly_hours: number
  effective_from: IsoDate
  capacity_changes: CapacityChange[]
}

/** Every failed request: an HTTP error with the server's code, or status 0 / code 'network'. */
export class ApiError extends Error {
  readonly status: number
  readonly code: string
  readonly details: Record<string, unknown>

  constructor(status: number, code: string, message: string, details: Record<string, unknown> = {}) {
    super(message)
    this.name = 'ApiError'
    this.status = status
    this.code = code
    this.details = details
  }
}

export type CapacityParams = {
  from: IsoDate
  to: IsoDate
  cursor?: string
  personIds?: number[]
  overOnly?: boolean
  limit?: number
}

export function fetchCapacity(p: CapacityParams, signal?: AbortSignal): Promise<CapacityPage> {
  const q = new URLSearchParams({ from: p.from, to: p.to })
  if (p.cursor) q.set('cursor', p.cursor)
  for (const id of p.personIds ?? []) q.append('person_id', String(id))
  if (p.overOnly) q.set('over_only', 'true')
  if (p.limit !== undefined) q.set('limit', String(p.limit))
  return request<CapacityPage>(`/api/capacity?${q}`, { signal })
}

export type CapacityUpdateBody = {
  weekly_hours: number
  effective_from: IsoDate
  expected_weekly_hours?: number
}

export function updateCapacity(personId: number, body: CapacityUpdateBody): Promise<PersonCapacity> {
  return request<PersonCapacity>(`/api/people/${personId}`, {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  })
}

async function request<T>(url: string, init: RequestInit): Promise<T> {
  let res: Response
  try {
    res = await fetch(url, init)
  } catch (err) {
    if (init.signal?.aborted) throw err // cancelled by the caller, not a failure
    throw new ApiError(0, 'network', 'Could not reach the server')
  }
  if (res.ok) return (await res.json()) as T
  throw await errorFrom(res)
}

async function errorFrom(res: Response): Promise<ApiError> {
  try {
    const body = (await res.json()) as { error?: { code?: string; message?: string } & Record<string, unknown> }
    if (body.error?.code) {
      const { code, message, ...details } = body.error
      return new ApiError(res.status, code, message ?? res.statusText, details)
    }
  } catch {
    // not our JSON error body — e.g. a proxy's HTML page
  }
  return new ApiError(res.status, 'http_error', `Request failed (${res.status})`)
}
