import { describe, expect, it } from 'vitest'

import { formatTimestamp } from './datetime'

describe('formatTimestamp', () => {
  it('renders an RFC3339 timestamp in UTC, whatever the viewer’s timezone', () => {
    expect(formatTimestamp('2026-08-18T09:05:00Z')).toBe('18 Aug 2026, 09:05 UTC')
  })

  it('converts an offset timestamp to UTC rather than trusting the offset', () => {
    expect(formatTimestamp('2026-08-18T11:05:00+02:00')).toBe('18 Aug 2026, 09:05 UTC')
  })

  it('echoes back something it cannot parse instead of showing Invalid Date', () => {
    expect(formatTimestamp('not a timestamp')).toBe('not a timestamp')
  })
})
