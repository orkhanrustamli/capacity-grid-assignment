// Calendar dates as 'YYYY-MM-DD' strings. A date here is a day on the
// calendar, not an instant, so every computation goes through UTC midnight:
// local-time Date arithmetic shifts a day near daylight-saving changes and
// for viewers west of UTC. Weeks are ISO weeks, Monday to Sunday (ADR 0001).
export type IsoDate = string

const DAY_MS = 24 * 60 * 60 * 1000
const ISO_DATE = /^\d{4}-\d{2}-\d{2}$/

function toUtc(d: IsoDate): Date {
  const [y, m, day] = d.split('-').map(Number)
  return new Date(Date.UTC(y, m - 1, day))
}

function fromUtc(d: Date): IsoDate {
  return d.toISOString().slice(0, 10)
}

export function isIsoDate(s: string): boolean {
  return ISO_DATE.test(s) && fromUtc(toUtc(s)) === s
}

/** The viewer's own calendar date — what "this week" means to them. */
export function todayIso(now: Date = new Date()): IsoDate {
  return fromUtc(new Date(Date.UTC(now.getFullYear(), now.getMonth(), now.getDate())))
}

export function addDays(d: IsoDate, n: number): IsoDate {
  return fromUtc(new Date(toUtc(d).getTime() + n * DAY_MS))
}

/** Whole calendar days from a to b (negative when b is earlier). */
export function daysBetween(a: IsoDate, b: IsoDate): number {
  return Math.round((toUtc(b).getTime() - toUtc(a).getTime()) / DAY_MS)
}

export function addWeeks(d: IsoDate, n: number): IsoDate {
  return addDays(d, 7 * n)
}

export function weekStartOf(d: IsoDate): IsoDate {
  const daysSinceMonday = (toUtc(d).getUTCDay() + 6) % 7
  return addDays(d, -daysSinceMonday)
}

export function weekEndOf(d: IsoDate): IsoDate {
  return addDays(weekStartOf(d), 6)
}

const weekLabel = new Intl.DateTimeFormat('en', { month: 'short', day: 'numeric', timeZone: 'UTC' })

export function formatWeekLabel(d: IsoDate): string {
  return weekLabel.format(toUtc(d))
}
