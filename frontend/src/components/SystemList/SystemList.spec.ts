import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import type { StatusItem, SystemOverview } from '@/api/status'
import SystemCard from '@/components/SystemCard/SystemCard.vue'

import SystemList from './SystemList.vue'

function makeSystem(id: number, title: string, indicator: SystemOverview['indicator']) {
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
    indicator,
    last_updated_at: '2026-08-18T09:00:00Z',
  } satisfies SystemOverview
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

describe('SystemList', () => {
  it('renders one card per system, in the order the API returned them', () => {
    const wrapper = mount(SystemList, {
      props: {
        systems: [
          makeSystem(1, 'GitHub', 'outage'),
          makeSystem(2, 'Datadog', 'degraded'),
          makeSystem(3, 'Cloudflare', 'operational'),
        ],
      },
    })

    const names = wrapper.findAll('.system-card__name').map((node) => node.text())
    expect(names).toEqual(['GitHub', 'Datadog', 'Cloudflare'])
  })

  it('renders nothing at all when the user has saved no systems', () => {
    const wrapper = mount(SystemList, { props: { systems: [] } })

    expect(wrapper.findAllComponents(SystemCard)).toHaveLength(0)
    expect(wrapper.text()).toBe('')
  })

  // The headline requirement: expanding one card must not disturb any other.
  it('keeps expansion state per card', async () => {
    const wrapper = mount(SystemList, {
      props: {
        systems: [
          makeSystem(1, 'GitHub', 'outage'),
          makeSystem(2, 'Datadog', 'degraded'),
          makeSystem(3, 'Cloudflare', 'operational'),
        ],
      },
    })

    const rows = wrapper.findAll('.system-card__row')
    await rows[0].trigger('click')
    await rows[2].trigger('click')

    const cards = wrapper.findAllComponents(SystemCard)
    expect(cards[0].find('.system-card__detail').exists()).toBe(true)
    expect(cards[1].find('.system-card__detail').exists()).toBe(false)
    expect(cards[2].find('.system-card__detail').exists()).toBe(true)

    // Collapsing the first one leaves the third open.
    await rows[0].trigger('click')
    expect(cards[0].find('.system-card__detail').exists()).toBe(false)
    expect(cards[2].find('.system-card__detail').exists()).toBe(true)
  })

  it('forwards a card’s request for its incidents', async () => {
    const wrapper = mount(SystemList, {
      props: { systems: [makeSystem(1, 'GitHub', 'outage'), makeSystem(2, 'Datadog', 'degraded')] },
    })

    await wrapper.findAll('.system-card__row')[1].trigger('click')

    expect(wrapper.emitted('expand')).toEqual([[2]])
  })

  it('hands each card only its own incidents', async () => {
    const wrapper = mount(SystemList, {
      props: {
        systems: [makeSystem(1, 'GitHub', 'outage'), makeSystem(2, 'Datadog', 'degraded')],
        itemsByFeed: { 1: [makeItem(101, 1), makeItem(102, 1)], 2: [makeItem(201, 2)] },
      },
    })

    const cards = wrapper.findAllComponents(SystemCard)
    expect(cards[0].props('items')).toHaveLength(2)
    expect(cards[1].props('items')).toHaveLength(1)
  })

  it('gives a card an empty list when its incidents are not loaded', () => {
    const wrapper = mount(SystemList, {
      props: { systems: [makeSystem(1, 'GitHub', 'outage')], itemsByFeed: {} },
    })

    expect(wrapper.getComponent(SystemCard).props('items')).toEqual([])
  })
})
