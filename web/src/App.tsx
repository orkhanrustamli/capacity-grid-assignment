import { CapacityGrid } from './components/CapacityGrid/CapacityGrid'
import { RangeControls } from './components/RangeControls'
import { useSaveInProgress } from './api/capacity.hooks'
import { useUrlRange } from './utils/urlRange'

// The page owns the range (in the URL) so the grid and, later, the logged-time
// timeline beside it read the same weeks.
export function App() {
  const [range, setRange] = useUrlRange()
  const saving = useSaveInProgress()
  return (
    <main>
      <h1>Team capacity</h1>
      <RangeControls range={range} onChange={setRange} disabled={saving} />
      <CapacityGrid from={range.from} to={range.to} />
    </main>
  )
}
