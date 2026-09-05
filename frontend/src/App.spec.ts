import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import StatusBanner from '@/components/StatusBanner/StatusBanner.vue'
import { clearBanners, notifyError, notifySuccess } from '@/composables/statusBanner'
import { BANNER_TIMEOUT_MS } from '@/utils/timing'

import App from './App.vue'

vi.mock('@/api/status', () => ({
  fetchOverview: vi.fn(() => Promise.resolve([])),
  fetchFeedItems: vi.fn(() => Promise.resolve([])),
}))

async function mountApp() {
  const wrapper = mount(App)
  await flushPromises()
  return wrapper
}

describe('App', () => {
  beforeEach(() => {
    clearBanners()
    vi.useFakeTimers()
  })

  afterEach(() => {
    vi.useRealTimers()
    clearBanners()
  })

  it('shows no banner when nothing has been announced', async () => {
    const wrapper = await mountApp()

    expect(wrapper.findComponent(StatusBanner).exists()).toBe(false)
  })

  it('shows an announced banner', async () => {
    const wrapper = await mountApp()

    notifyError('Could not refresh. Showing the last known status.')
    await flushPromises()

    const banner = wrapper.getComponent(StatusBanner)
    expect(banner.props('kind')).toBe('error')
    expect(banner.props('message')).toBe('Could not refresh. Showing the last known status.')
  })

  it('replaces the banner with the next queued one when it is closed', async () => {
    const wrapper = await mountApp()
    notifyError('First.')
    notifySuccess('Second.')
    await flushPromises()

    await wrapper.get('.status-banner__close').trigger('click')
    await flushPromises()

    const banner = wrapper.getComponent(StatusBanner)
    expect(banner.props('message')).toBe('Second.')
    expect(banner.props('kind')).toBe('success')
  })

  it('shows only one banner at a time', async () => {
    const wrapper = await mountApp()

    notifyError('First.')
    notifyError('Second.')
    notifyError('Third.')
    await flushPromises()

    expect(wrapper.findAllComponents(StatusBanner)).toHaveLength(1)
  })

  it('moves on by itself when a banner times out', async () => {
    const wrapper = await mountApp()
    notifyError('First.')
    notifySuccess('Second.')
    await flushPromises()

    await vi.advanceTimersByTimeAsync(BANNER_TIMEOUT_MS)

    expect(wrapper.getComponent(StatusBanner).props('message')).toBe('Second.')
  })

  // Each banner gets its own countdown: the second must not inherit the first's remaining
  // time, which is what keying on the id buys us.
  it('gives the next banner a full timeout of its own', async () => {
    const wrapper = await mountApp()
    notifyError('First.')
    notifySuccess('Second.')
    await flushPromises()

    await vi.advanceTimersByTimeAsync(BANNER_TIMEOUT_MS)
    expect(wrapper.getComponent(StatusBanner).props('message')).toBe('Second.')

    await vi.advanceTimersByTimeAsync(BANNER_TIMEOUT_MS - 1)
    expect(wrapper.findComponent(StatusBanner).exists()).toBe(true)

    await vi.advanceTimersByTimeAsync(1)
    expect(wrapper.findComponent(StatusBanner).exists()).toBe(false)
  })

  it('renders the banner outside the width-constrained page area', async () => {
    const wrapper = await mountApp()
    notifyError('Anywhere but inside.')
    await flushPromises()

    expect(wrapper.get('.app').find('.status-banner').exists()).toBe(false)
    expect(wrapper.find('.status-banner').exists()).toBe(true)
  })
})
