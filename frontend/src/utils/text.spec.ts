import { describe, expect, it } from 'vitest'

import { truncate } from './text'

describe('truncate', () => {
  it('leaves text shorter than the limit alone', () => {
    expect(truncate('Status updated.', 100)).toBe('Status updated.')
  })

  it('leaves text of exactly the limit alone', () => {
    const exact = 'x'.repeat(100)

    expect(truncate(exact, 100)).toBe(exact)
  })

  // One character over: the result is still exactly the limit, ellipsis included.
  it('replaces the tail with an ellipsis once the limit is passed', () => {
    const long = 'y'.repeat(101)

    const result = truncate(long, 100)

    expect(result).toHaveLength(100)
    expect(result).toBe('y'.repeat(97) + '...')
  })

  it('keeps the beginning of a much longer message', () => {
    const result = truncate('Could not refresh. ' + 'z'.repeat(500), 100)

    expect(result).toHaveLength(100)
    expect(result.startsWith('Could not refresh. ')).toBe(true)
    expect(result.endsWith('...')).toBe(true)
  })

  it('handles an empty string', () => {
    expect(truncate('', 100)).toBe('')
  })
})
