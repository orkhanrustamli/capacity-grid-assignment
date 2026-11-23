import type { WeekCell } from './api'
import { formatWeekLabel } from './dates'

const hours = new Intl.NumberFormat('en', { maximumFractionDigits: 2 })

type Props = {
  personName: string
  cell: WeekCell
}

/**
 * One person-week. Status comes from the server and is shown by colour and
 * by text (▲ and the excess), never by colour alone.
 */
export function CapacityCell({ personName, cell }: Props) {
  const excess = cell.allocated_hours - cell.capacity_hours
  const noCapacity = cell.capacity_hours === 0
  const className = ['cell', `cell--${cell.status}`, noCapacity && 'cell--no-capacity'].filter(Boolean).join(' ')

  return (
    <td className={className} aria-label={describe(personName, cell)}>
      <span className="cell__numbers">
        {hours.format(cell.allocated_hours)} / {hours.format(cell.capacity_hours)} h
      </span>
      {cell.status === 'over' && (
        <span className="cell__over">{noCapacity ? 'No capacity' : `▲ +${hours.format(excess)} h`}</span>
      )}
      {cell.weekend_hours > 0 && <span className="cell__weekend">+{hours.format(cell.weekend_hours)} h weekend</span>}
      {cell.capacity_changed && <span className="cell__changed" title="Capacity changes this week" />}
    </td>
  )
}

/** The cell as a sentence, for screen readers. */
function describe(personName: string, cell: WeekCell): string {
  const parts = [
    `${personName}, week of ${formatWeekLabel(cell.week)}: ` +
      `${hours.format(cell.allocated_hours)} of ${hours.format(cell.capacity_hours)} hours`,
  ]
  if (cell.status === 'over') {
    parts.push(cell.capacity_hours === 0 ? 'no capacity' : `over by ${hours.format(cell.allocated_hours - cell.capacity_hours)}`)
  } else if (cell.status === 'full') {
    parts.push('fully allocated')
  }
  if (cell.weekend_hours > 0) parts.push(`plus ${hours.format(cell.weekend_hours)} weekend hours`)
  if (cell.capacity_changed) parts.push('capacity changes this week')
  return parts.join(', ')
}
