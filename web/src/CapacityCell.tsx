import { useState } from 'react'
import type { WeekCell } from './api'
import { CapacityEditor } from './CapacityEditor'
import { formatWeekLabel } from './dates'
import { formatHours } from './hours'
import type { RowTarget } from './useCapacity'

type Props = {
  personName: string
  cell: WeekCell
  /** Where an edit of this cell is saved and refreshed; null while the grid shows an older range. */
  editTarget: RowTarget | null
  onRefreshFailed: () => void
}

/**
 * One person-week. Status comes from the server and is shown by colour and
 * by text (▲ and the excess), never by colour alone. The capacity number
 * opens the editor.
 */
export function CapacityCell({ personName, cell, editTarget, onRefreshFailed }: Props) {
  const [editing, setEditing] = useState(false)
  const noCapacity = cell.capacity_hours === 0
  const className = ['cell', `cell--${cell.status}`, noCapacity && 'cell--no-capacity'].filter(Boolean).join(' ')
  const capacity = formatHours(cell.capacity_hours)

  return (
    <td className={className} aria-label={describe(personName, cell)}>
      <span className="cell__numbers">
        {formatHours(cell.allocated_hours)} /{' '}
        {editTarget ? (
          <button
            type="button"
            className="cell__capacity"
            aria-label={`Edit weekly hours for ${personName} from week of ${formatWeekLabel(cell.week)}`}
            aria-expanded={editing}
            onClick={() => setEditing(true)}
          >
            {capacity}
          </button>
        ) : (
          capacity
        )}{' '}
        h
      </span>
      {cell.status === 'over' && (
        <span className="cell__over">
          {noCapacity ? 'No capacity' : `▲ +${formatHours(cell.allocated_hours - cell.capacity_hours)} h`}
        </span>
      )}
      {cell.weekend_hours > 0 && <span className="cell__weekend">+{formatHours(cell.weekend_hours)} h weekend</span>}
      {cell.capacity_changed && <span className="cell__changed" title="Capacity changes this week" />}
      {editing && editTarget && (
        <CapacityEditor
          target={editTarget}
          cell={cell}
          onClose={() => setEditing(false)}
          onRefreshFailed={onRefreshFailed}
        />
      )}
    </td>
  )
}

/** The cell as a sentence, for screen readers. */
function describe(personName: string, cell: WeekCell): string {
  const parts = [
    `${personName}, week of ${formatWeekLabel(cell.week)}: ` +
      `${formatHours(cell.allocated_hours)} of ${formatHours(cell.capacity_hours)} hours`,
  ]
  if (cell.status === 'over') {
    parts.push(cell.capacity_hours === 0 ? 'no capacity' : `over by ${formatHours(cell.allocated_hours - cell.capacity_hours)}`)
  } else if (cell.status === 'full') {
    parts.push('fully allocated')
  }
  if (cell.weekend_hours > 0) parts.push(`plus ${formatHours(cell.weekend_hours)} weekend hours`)
  if (cell.capacity_changed) parts.push('capacity changes this week')
  return parts.join(', ')
}
