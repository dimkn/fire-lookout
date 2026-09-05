import { beforeEach, describe, expect, it } from 'vitest'

import {
  clearBanners,
  currentBanner,
  dismissBanner,
  notifyError,
  notifySuccess,
} from './statusBanner'

describe('status banner queue', () => {
  beforeEach(() => {
    clearBanners()
  })

  it('shows nothing until something is announced', () => {
    expect(currentBanner.value).toBeNull()
  })

  it('shows an announced error', () => {
    notifyError('Could not refresh.')

    expect(currentBanner.value).toMatchObject({ kind: 'error', message: 'Could not refresh.' })
  })

  it('shows an announced success', () => {
    notifySuccess('Status updated.')

    expect(currentBanner.value).toMatchObject({ kind: 'success', message: 'Status updated.' })
  })

  // One at a time: the second announcement waits its turn rather than stacking.
  it('keeps showing the first of several until it is dismissed', () => {
    notifyError('First.')
    notifySuccess('Second.')
    notifyError('Third.')

    expect(currentBanner.value?.message).toBe('First.')
  })

  it('shows the next one when the current is dismissed', () => {
    notifyError('First.')
    notifySuccess('Second.')

    dismissBanner()

    expect(currentBanner.value).toMatchObject({ kind: 'success', message: 'Second.' })
  })

  it('shows nothing again once the last is dismissed', () => {
    notifyError('Only one.')

    dismissBanner()

    expect(currentBanner.value).toBeNull()
  })

  it('tolerates being dismissed when nothing is showing', () => {
    expect(() => dismissBanner()).not.toThrow()
    expect(currentBanner.value).toBeNull()
  })

  // The host keys the rendered banner on the id, so ids must never repeat: a duplicate
  // would leave the previous banner mounted with its old countdown.
  it('gives every banner a distinct id, even for identical messages', () => {
    notifyError('Same message.')
    const first = currentBanner.value?.id
    dismissBanner()
    notifyError('Same message.')

    expect(currentBanner.value?.id).not.toBe(first)
  })
})
