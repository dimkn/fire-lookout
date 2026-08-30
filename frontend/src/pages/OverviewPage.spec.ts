import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import type { StatusItem, SystemOverview } from '@/api/status'
import { fetchFeedItems, fetchOverview } from '@/api/status'
import SystemCard from '@/components/SystemCard/SystemCard.vue'
import PageToolbar from '@/components/PageToolbar/PageToolbar.vue'

import OverviewPage from './OverviewPage.vue'

vi.mock('@/api/status', () => ({
  fetchOverview: vi.fn(),
  fetchFeedItems: vi.fn(),
}))

const fetchOverviewMock = vi.mocked(fetchOverview)
const fetchFeedItemsMock = vi.mocked(fetchFeedItems)

function makeSystem(id: number, title: string): SystemOverview {
  return {
    feed: {
      id,
      url: `https://${title.toLowerCase()}.test/feed.atom`,
      title,
      group_id: 0,
      enabled: true,
      refresh_interval_sec: 300,
      last_fetched_at: '2026-08-18T09:05:00Z',
      last_success_at: '2026-08-18T09:05:00Z',
      created_at: '2026-08-01T00:00:00Z',
      updated_at: '2026-08-18T09:05:00Z',
    },
    indicator: 'operational',
    last_updated_at: '2026-08-18T09:00:00Z',
  }
}

function makeItem(id: number, feedId: number): StatusItem {
  return {
    id,
    feed_id: feedId,
    guid: `guid-${id}`,
    title: `Incident ${id}`,
    link: null,
    published_at: '2026-08-18T08:00:00Z',
    updated_at: null,
    content_html: null,
    content_text: 'Something happened.',
    status: 'resolved',
    fetched_at: '2026-08-18T09:05:00Z',
  }
}

async function mountPage() {
  const wrapper = mount(OverviewPage)
  await flushPromises()
  return wrapper
}

describe('OverviewPage', () => {
  beforeEach(() => {
    fetchOverviewMock.mockReset()
    fetchFeedItemsMock.mockReset()
    fetchOverviewMock.mockResolvedValue([])
    fetchFeedItemsMock.mockResolvedValue([])
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('asks for the overview exactly once on load', async () => {
    await mountPage()

    expect(fetchOverviewMock).toHaveBeenCalledTimes(1)
    expect(fetchFeedItemsMock).not.toHaveBeenCalled()
  })

  it('renders a card per system', async () => {
    fetchOverviewMock.mockResolvedValue([makeSystem(1, 'GitHub'), makeSystem(2, 'Datadog')])

    const wrapper = await mountPage()

    expect(wrapper.findAllComponents(SystemCard)).toHaveLength(2)
  })

  it('leaves the area empty when nothing is saved yet', async () => {
    const wrapper = await mountPage()

    expect(wrapper.findAllComponents(SystemCard)).toHaveLength(0)
    expect(wrapper.get('.overview__systems').text()).toBe('')
  })

  it('fetches only the expanded system’s incidents', async () => {
    fetchOverviewMock.mockResolvedValue([makeSystem(1, 'GitHub'), makeSystem(2, 'Datadog')])
    fetchFeedItemsMock.mockResolvedValue([makeItem(201, 2)])

    const wrapper = await mountPage()
    await wrapper.findAll('.system-card__row')[1].trigger('click')
    await flushPromises()

    expect(fetchFeedItemsMock).toHaveBeenCalledTimes(1)
    expect(fetchFeedItemsMock).toHaveBeenCalledWith(2)
  })

  it('passes the fetched incidents back to the card that asked', async () => {
    fetchOverviewMock.mockResolvedValue([makeSystem(1, 'GitHub'), makeSystem(2, 'Datadog')])
    fetchFeedItemsMock.mockResolvedValue([makeItem(201, 2), makeItem(202, 2)])

    const wrapper = await mountPage()
    await wrapper.findAll('.system-card__row')[1].trigger('click')
    await flushPromises()

    const cards = wrapper.findAllComponents(SystemCard)
    expect(cards[1].props('items')).toHaveLength(2)
    expect(cards[0].props('items')).toEqual([])
  })

  it('keeps incidents it already has instead of refetching', async () => {
    fetchOverviewMock.mockResolvedValue([makeSystem(1, 'GitHub')])
    fetchFeedItemsMock.mockResolvedValue([makeItem(101, 1)])

    const wrapper = await mountPage()
    const row = wrapper.get('.system-card__row')
    await row.trigger('click')
    await flushPromises()
    await row.trigger('click')
    await row.trigger('click')
    await flushPromises()

    expect(fetchFeedItemsMock).toHaveBeenCalledTimes(1)
  })

  it('survives a failing overview request with an empty area', async () => {
    const logged = vi.spyOn(console, 'error').mockImplementation(() => {})
    fetchOverviewMock.mockRejectedValue(new Error('fetch overview: 500'))

    const wrapper = await mountPage()

    expect(wrapper.findAllComponents(SystemCard)).toHaveLength(0)
    expect(logged).toHaveBeenCalled()
  })

  it('survives a failing incident request with the card still open', async () => {
    const logged = vi.spyOn(console, 'error').mockImplementation(() => {})
    fetchOverviewMock.mockResolvedValue([makeSystem(1, 'GitHub')])
    fetchFeedItemsMock.mockRejectedValue(new Error('fetch items for feed 1: 404'))

    const wrapper = await mountPage()
    await wrapper.get('.system-card__row').trigger('click')
    await flushPromises()

    expect(wrapper.find('.system-card__detail').exists()).toBe(true)
    expect(wrapper.getComponent(SystemCard).props('items')).toEqual([])
    expect(logged).toHaveBeenCalled()
  })

  // Both buttons are deliberately inert in this slice.
  it('does not call the API when the toolbar buttons are pressed', async () => {
    fetchOverviewMock.mockResolvedValue([makeSystem(1, 'GitHub')])

    const wrapper = await mountPage()
    const toolbar = wrapper.getComponent(PageToolbar)
    toolbar.vm.$emit('refresh')
    toolbar.vm.$emit('add')
    await flushPromises()

    expect(fetchOverviewMock).toHaveBeenCalledTimes(1)
    expect(fetchFeedItemsMock).not.toHaveBeenCalled()
  })
})
