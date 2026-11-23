import { CapacityGrid } from './CapacityGrid'
import { RangeControls } from './RangeControls'
import { useUrlRange } from './urlRange'

// The page owns the range (in the URL) so the grid and, later, the logged-time
// timeline beside it read the same weeks.
export function App() {
  const [range, setRange] = useUrlRange()
  return (
    <main>
      <h1>Team capacity</h1>
      <RangeControls range={range} onChange={setRange} />
      <CapacityGrid from={range.from} to={range.to} />
    </main>
  )
}
