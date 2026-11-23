import { useState } from 'react'
import { addWeeks, isIsoDate, todayIso, weekEndOf, weekStartOf } from './dates'
import { DEMO_RANGE, rangeProblem, weeksIn, type Range } from './urlRange'

type Props = {
  range: Range
  onChange: (r: Range) => void
}

function shift(r: Range, weeks: number): Range {
  return { from: addWeeks(r.from, weeks), to: addWeeks(r.to, weeks) }
}

/** The same number of weeks as now, starting this Monday. */
function thisWeek(r: Range): Range {
  const from = weekStartOf(todayIso())
  return { from, to: weekEndOf(addWeeks(from, weeksIn(r) - 1)) }
}

export function RangeControls({ range, onChange }: Props) {
  // The inputs edit a draft, applied only when it is a loadable range, so a
  // half-typed year never reaches the API. The draft follows the range when
  // it changes from outside (buttons, back button).
  const [draft, setDraft] = useState(range)
  const [shown, setShown] = useState(range)
  if (shown.from !== range.from || shown.to !== range.to) {
    setShown(range)
    setDraft(range)
  }

  const problem = isIsoDate(draft.from) && isIsoDate(draft.to) ? rangeProblem(draft) : null

  function edit(next: Range) {
    setDraft(next)
    if (isIsoDate(next.from) && isIsoDate(next.to) && !rangeProblem(next)) onChange(next)
  }

  return (
    <div className="range-controls">
      <div className="range-controls__buttons">
        <button type="button" onClick={() => onChange(shift(range, -1))}>
          ← Week
        </button>
        <button type="button" onClick={() => onChange(thisWeek(range))}>
          This week
        </button>
        <button type="button" onClick={() => onChange(shift(range, 1))}>
          Week →
        </button>
        <button type="button" onClick={() => onChange(DEMO_RANGE)}>
          Demo range
        </button>
      </div>
      <label>
        From
        <input type="date" value={draft.from} onChange={(e) => edit({ ...draft, from: e.target.value })} />
      </label>
      <label>
        To
        <input type="date" value={draft.to} onChange={(e) => edit({ ...draft, to: e.target.value })} />
      </label>
      {problem && (
        <p className="range-controls__problem" role="alert">
          {problem}
        </p>
      )}
    </div>
  )
}
