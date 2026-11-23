import type { Range } from '../utils/urlRange'

export const capacityKeys = {
  all: ['capacity'] as const,
  range: (r: Range, overOnly: boolean) => ['capacity', r.from, r.to, overOnly] as const,
}
