import type { CapacityPage, CapacityStatus, PersonRow } from '../api/capacity.api'

// The golden values for seed people 1–5 over the demo range
// (api/capacity_golden_test.go), as the API returns them.
export const WEEKS = ['2025-12-29', '2026-01-05', '2026-01-12']
type CellSpec = [allocated: number, capacity: number, weekend: number, status: CapacityStatus]

export function row(id: number, name: string, cells: CellSpec[], weeks = WEEKS): PersonRow {
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

export const golden = [
  row(1, 'Ana Ferreira', [[40, 40, 16, 'full'], [0, 40, 0, 'under'], [30, 40, 0, 'under']]),
  row(2, 'Bo Lindqvist', [[0, 40, 0, 'under'], [32, 40, 16, 'under'], [8, 40, 0, 'under']]),
  row(3, 'Cem Aydin', [[0, 20, 0, 'under'], [4, 20, 0, 'under'], [12, 20, 0, 'under']]),
  row(4, 'Dee Okafor', [[0, 40, 0, 'under'], [45, 40, 0, 'over'], [40, 40, 0, 'full']]),
  row(5, 'Eli Nakamura', [[0, 0, 0, 'under'], [20, 0, 0, 'over'], [0, 0, 0, 'under']]),
]

export function page(people: PersonRow[], extra: Partial<CapacityPage> = {}): CapacityPage {
  return { from: '2025-12-29', to: '2026-01-18', weeks: WEEKS, people, next_cursor: null, over_count: 2, ...extra }
}

export const otherRange = page([row(9, 'Zed Quinn', [[8, 40, 0, 'under']], ['2026-01-19'])], {
  from: '2026-01-19',
  to: '2026-01-25',
  weeks: ['2026-01-19'],
  over_count: 0,
})

// Dee after her capacity is raised to 48 h from the week of Jan 5.
export const deeAt48 = row(4, 'Dee Okafor', [[0, 40, 0, 'under'], [45, 48, 0, 'under'], [40, 48, 0, 'under']])
deeAt48.weeks[1].capacity_changed = true
export const deeAt36 = row(4, 'Dee Okafor', [[0, 40, 0, 'under'], [45, 36, 0, 'over'], [40, 36, 0, 'over']])

export const savedDee = {
  id: 4,
  name: 'Dee Okafor',
  weekly_hours: 48,
  effective_from: '2026-01-05',
  capacity_changes: [
    { effective_from: null, weekly_hours: 40 },
    { effective_from: '2026-01-05', weekly_hours: 48 },
  ],
}
