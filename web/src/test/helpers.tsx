import { vi } from 'vitest'
import { render } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { CapacityGrid } from '../CapacityGrid'

export function json(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } })
}

export function errorBody(status: number, code: string, message: string, extra: Record<string, unknown> = {}): Response {
  return json({ error: { code, message, ...extra } }, status)
}

export function deferred<T>() {
  let resolve!: (v: T) => void
  const promise = new Promise<T>((r) => (resolve = r))
  return { promise, resolve }
}

export type Call = { method: string; url: string; body: unknown }

/** Replaces fetch; returns the requests made, in order. */
export function stubFetch(handler: (url: URL, init: RequestInit) => Response | Promise<Response>): Call[] {
  const calls: Call[] = []
  vi.stubGlobal(
    'fetch',
    vi.fn(async (input: RequestInfo | URL, init: RequestInit = {}) => {
      const url = new URL(String(input), 'http://localhost')
      const body = typeof init.body === 'string' ? JSON.parse(init.body) : undefined
      calls.push({ method: init.method ?? 'GET', url: url.pathname + url.search, body })
      return handler(url, init)
    }),
  )
  return calls
}

export const isRowRefresh = (url: URL) => url.searchParams.has('person_id')
export const gridLoads = (calls: Call[]) => calls.filter((c) => c.method === 'GET' && !c.url.includes('person_id'))
export const rowLoads = (calls: Call[]) => calls.filter((c) => c.method === 'GET' && c.url.includes('person_id'))
export const patches = (calls: Call[]) => calls.filter((c) => c.method === 'PATCH')

export function testQueryClient() {
  return new QueryClient({ defaultOptions: { queries: { retry: false } } })
}

export function renderGrid(from = '2025-12-29', to = '2026-01-16') {
  const client = testQueryClient()
  const ui = (f: string, t: string) => (
    <QueryClientProvider client={client}>
      <CapacityGrid from={f} to={t} />
    </QueryClientProvider>
  )
  const result = render(ui(from, to))
  return { ...result, client, setRange: (f: string, t: string) => result.rerender(ui(f, t)) }
}
