import { useState, type FormEvent, type KeyboardEvent } from 'react'
import { ApiError, type WeekCell } from './api'
import { formatWeekLabel } from './dates'
import { formatHours } from './hours'
import { RefreshFailedError, useUpdateCapacity, type RowTarget, type SaveVariables } from './useCapacity'

type Props = {
  target: RowTarget
  cell: WeekCell
  onClose: () => void
  onRefreshFailed: () => void
}

/**
 * "Weekly hours from week of Jan 5: [32] Save / Cancel". Enter saves, Escape
 * cancels. The change applies from this week onward; the numbers on screen
 * change only once the server has re-read the row.
 */
export function CapacityEditor({ target, cell, onClose, onRefreshFailed }: Props) {
  const [value, setValue] = useState(String(cell.capacity_hours))
  const [emptyInput, setEmptyInput] = useState(false)
  const save = useUpdateCapacity()
  const label = `Weekly hours from week of ${formatWeekLabel(cell.week)}`

  function submit(e: FormEvent) {
    e.preventDefault()
    if (value.trim() === '') {
      setEmptyInput(true)
      return
    }
    setEmptyInput(false)
    send({ ...target, weekStart: cell.week, weeklyHours: Number(value), expected: cell.capacity_hours })
  }

  function send(variables: SaveVariables) {
    save.mutate(variables, {
      onSuccess: onClose,
      onError: (err) => {
        if (err instanceof RefreshFailedError) {
          onRefreshFailed()
          onClose()
        }
      },
    })
  }

  function onKeyDown(e: KeyboardEvent) {
    if (e.key === 'Escape') onClose()
  }

  // The server validates; the form does not, so its messages are the ones shown.
  const problem = emptyInput ? { text: 'Enter a number of hours.', retry: false } : save.error && describeSaveError(save.error)

  return (
    <form className="cell-editor" onSubmit={submit} onKeyDown={onKeyDown} noValidate>
      <label>
        {label}
        <input
          type="number"
          min={0}
          max={168}
          step="any"
          value={value}
          onChange={(e) => setValue(e.target.value)}
          autoFocus
          aria-invalid={problem ? true : undefined}
        />
      </label>
      <div className="cell-editor__actions">
        <button type="submit" disabled={save.isPending}>
          {save.isPending ? 'Saving…' : 'Save'}
        </button>
        <button type="button" onClick={onClose}>
          Cancel
        </button>
      </div>
      {problem && (
        <p className="cell-editor__problem" role="alert">
          {problem.text}
          {problem.retry && save.variables && (
            <button type="button" onClick={() => send(save.variables!)}>
              Retry
            </button>
          )}
        </p>
      )}
    </form>
  )
}

function describeSaveError(err: ApiError | RefreshFailedError): { text: string; retry: boolean } | null {
  if (err instanceof RefreshFailedError) return null // the row shows it, not the editor
  if (err.code === 'capacity_conflict') {
    const current = Number(err.details.current_weekly_hours)
    return { text: `Changed by someone else — now ${formatHours(current)} h.`, retry: false }
  }
  if (err.status === 400 || err.status === 422) return { text: err.message, retry: false }
  return { text: "Couldn't save. Your change wasn't applied.", retry: true }
}
