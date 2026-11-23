const hours = new Intl.NumberFormat('en', { maximumFractionDigits: 2 })

/** Hours as people read them: 40, 37.5, 0.25 — never 40.00 or 0.1000000000000014. */
export function formatHours(n: number): string {
  return hours.format(n)
}
