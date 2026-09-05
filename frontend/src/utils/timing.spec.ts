import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { delay } from './timing'

describe('delay', () => {
  beforeEach(() => {
    vi.useFakeTimers()
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('stays pending until the time has passed', async () => {
    let settled = false
    void delay(500).then(() => {
      settled = true
    })

    await vi.advanceTimersByTimeAsync(499)
    expect(settled).toBe(false)

    await vi.advanceTimersByTimeAsync(1)
    expect(settled).toBe(true)
  })
})
