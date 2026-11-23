import { CapacityGrid } from './CapacityGrid'
import { RangeControls } from './RangeControls'
import { useSaveInProgress } from './useCapacity'
import { useUrlRange } from './urlRange'

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
