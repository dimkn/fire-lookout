import { mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { BANNER_TIMEOUT_MS } from '@/utils/timing'

import StatusBanner from './StatusBanner.vue'

describe('StatusBanner', () => {
  it('shows the message', () => {
    const wrapper = mount(StatusBanner, { props: { kind: 'error', message: 'Could not refresh.' } })

    expect(wrapper.get('.status-banner__message').text()).toBe('Could not refresh.')
  })

  it('carries a modifier class per kind — the colour is the only difference', () => {
    const error = mount(StatusBanner, { props: { kind: 'error', message: 'Nope.' } })
    const success = mount(StatusBanner, { props: { kind: 'success', message: 'Nope.' } })

    expect(error.get('.status-banner').classes()).toContain('status-banner--error')
    expect(success.get('.status-banner').classes()).toContain('status-banner--success')
    // Same structure either way.
    expect(error.get('.status-banner__message').text()).toBe(
      success.get('.status-banner__message').text(),
    )
    expect(success.find('.status-banner__close').exists()).toBe(true)
  })

  it('announces errors assertively and successes politely', () => {
    const error = mount(StatusBanner, { props: { kind: 'error', message: 'Nope.' } })
    const success = mount(StatusBanner, { props: { kind: 'success', message: 'Done.' } })

    expect(error.get('.status-banner').attributes('role')).toBe('alert')
    expect(success.get('.status-banner').attributes('role')).toBe('status')
  })

  it('holds nothing but the message and a labelled close button', () => {
    const wrapper = mount(StatusBanner, { props: { kind: 'success', message: 'Done.' } })

    expect(wrapper.get('.status-banner__close').attributes('aria-label')).toBe('Close')
    expect(wrapper.findAll('button')).toHaveLength(1)
    expect(wrapper.findAll('svg')).toHaveLength(0)
  })

  describe('message length', () => {
    it('shows a 100-character message in full', () => {
      const exact = 'x'.repeat(100)

      const wrapper = mount(StatusBanner, { props: { kind: 'error', message: exact } })

      expect(wrapper.get('.status-banner__message').text()).toBe(exact)
    })

    it('cuts anything longer to 97 characters plus an ellipsis', () => {
      const wrapper = mount(StatusBanner, {
        props: { kind: 'error', message: 'y'.repeat(250) },
      })

      const shown = wrapper.get('.status-banner__message').text()
      expect(shown).toHaveLength(100)
      expect(shown).toBe('y'.repeat(97) + '...')
    })
  })

  describe('dismissal', () => {
    beforeEach(() => {
      vi.useFakeTimers()
    })

    afterEach(() => {
      vi.useRealTimers()
    })

    it('emits close when the close button is pressed', async () => {
      const wrapper = mount(StatusBanner, { props: { kind: 'error', message: 'Nope.' } })

      await wrapper.get('.status-banner__close').trigger('click')

      expect(wrapper.emitted('close')).toHaveLength(1)
    })

    it('dismisses itself once the timeout passes', async () => {
      const wrapper = mount(StatusBanner, { props: { kind: 'success', message: 'Done.' } })

      await vi.advanceTimersByTimeAsync(BANNER_TIMEOUT_MS - 1)
      expect(wrapper.emitted('close')).toBeUndefined()

      await vi.advanceTimersByTimeAsync(1)
      expect(wrapper.emitted('close')).toHaveLength(1)
    })

    // Closing by hand must not leave a timer behind that fires into nothing.
    it('drops its timer when unmounted', async () => {
      const wrapper = mount(StatusBanner, { props: { kind: 'error', message: 'Nope.' } })

      wrapper.unmount()
      await vi.advanceTimersByTimeAsync(BANNER_TIMEOUT_MS * 2)

      expect(wrapper.emitted('close')).toBeUndefined()
    })
  })
})
