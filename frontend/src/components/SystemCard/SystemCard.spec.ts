import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import type { StatusItem, SystemOverview } from '@/api/status'
import IncidentItem from '@/components/IncidentItem/IncidentItem.vue'
import StatusIndicator from '@/components/StatusIndicator/StatusIndicator.vue'

import SystemCard from './SystemCard.vue'

function makeSystem(overrides: Partial<SystemOverview> = {}): SystemOverview {
  return {
    feed: {
      id: 1,
      url: 'https://www.githubstatus.com/history.atom',
      title: 'GitHub',
      group_id: 0,
      enabled: true,
      refresh_interval_sec: 300,
      current_status: 'investigating',
      last_fetched_at: '2026-08-18T09:05:00Z',
      last_success_at: '2026-08-18T09:05:00Z',
      created_at: '2026-08-01T00:00:00Z',
      updated_at: '2026-08-18T09:05:00Z',
    },
    indicator: 'outage',
    last_updated_at: '2026-08-18T09:02:00Z',
    ...overrides,
  }
}

function makeItem(overrides: Partial<StatusItem> = {}): StatusItem {
  return {
    id: 101,
    feed_id: 1,
    guid: 'gh-1',
    title: 'Elevated error rates',
    link: null,
    published_at: '2026-08-18T08:41:00Z',
    updated_at: null,
    content_html: null,
    content_text: 'Investigating.',
    status: 'investigating',
    fetched_at: '2026-08-18T09:05:00Z',
    ...overrides,
  }
}

describe('SystemCard', () => {
  it('shows the indicator and the user-given name on the collapsed row', () => {
    const wrapper = mount(SystemCard, { props: { system: makeSystem() } })

    expect(wrapper.getComponent(StatusIndicator).props('indicator')).toBe('outage')
    expect(wrapper.get('.system-card__name').text()).toBe('GitHub')
  })

  it('starts collapsed', () => {
    const wrapper = mount(SystemCard, { props: { system: makeSystem() } })

    expect(wrapper.find('.system-card__detail').exists()).toBe(false)
    expect(wrapper.get('button').attributes('aria-expanded')).toBe('false')
  })

  it('reveals the detail when the row is activated', async () => {
    const wrapper = mount(SystemCard, { props: { system: makeSystem() } })

    await wrapper.get('button').trigger('click')

    expect(wrapper.find('.system-card__detail').exists()).toBe(true)
    expect(wrapper.get('button').attributes('aria-expanded')).toBe('true')
  })

  it('collapses again on a second activation', async () => {
    const wrapper = mount(SystemCard, { props: { system: makeSystem() } })

    await wrapper.get('button').trigger('click')
    await wrapper.get('button').trigger('click')

    expect(wrapper.find('.system-card__detail').exists()).toBe(false)
  })

  it('asks for its incidents the first time it opens', async () => {
    const wrapper = mount(SystemCard, { props: { system: makeSystem() } })

    await wrapper.get('button').trigger('click')

    expect(wrapper.emitted('expand')).toEqual([[1]])
  })

  // Re-opening must not refetch: the page already holds this card's incidents.
  it('does not ask again when reopened', async () => {
    const wrapper = mount(SystemCard, { props: { system: makeSystem() } })

    await wrapper.get('button').trigger('click')
    await wrapper.get('button').trigger('click')
    await wrapper.get('button').trigger('click')

    expect(wrapper.emitted('expand')).toHaveLength(1)
  })

  it('lists the incidents it was given', async () => {
    const wrapper = mount(SystemCard, {
      props: {
        system: makeSystem(),
        items: [makeItem(), makeItem({ id: 102, guid: 'gh-2', title: 'Degraded Codespaces' })],
      },
    })

    await wrapper.get('button').trigger('click')

    expect(wrapper.findAllComponents(IncidentItem)).toHaveLength(2)
  })

  // No loading state by design: an empty card body covers both "not fetched yet" and
  // "genuinely no incidents" without inventing a spinner.
  it('shows the detail without an incident list when it has no items', async () => {
    const wrapper = mount(SystemCard, { props: { system: makeSystem(), items: [] } })

    await wrapper.get('button').trigger('click')

    expect(wrapper.find('.system-card__detail').exists()).toBe(true)
    expect(wrapper.findAllComponents(IncidentItem)).toHaveLength(0)
  })

  it('surfaces the feed url and last check in the detail', async () => {
    const wrapper = mount(SystemCard, { props: { system: makeSystem() } })

    await wrapper.get('button').trigger('click')

    const detail = wrapper.get('.system-card__detail')
    expect(detail.text()).toContain('https://www.githubstatus.com/history.atom')
    expect(detail.text()).toContain('18 Aug 2026, 09:05 UTC')
  })

  it('reports a failing poll without repainting the light', async () => {
    const system = makeSystem()
    system.feed.last_error = 'dial tcp: i/o timeout'
    const wrapper = mount(SystemCard, { props: { system } })

    await wrapper.get('button').trigger('click')

    expect(wrapper.get('.system-card__error').text()).toContain('dial tcp: i/o timeout')
    expect(wrapper.getComponent(StatusIndicator).props('indicator')).toBe('outage')
  })

  it('says so plainly when a system has never been polled', async () => {
    const wrapper = mount(SystemCard, {
      props: {
        system: makeSystem({
          indicator: 'unknown',
          last_updated_at: null,
          feed: { ...makeSystem().feed, last_fetched_at: null, last_success_at: null },
        }),
      },
    })

    await wrapper.get('button').trigger('click')

    expect(wrapper.get('.system-card__detail').text()).toContain('Never')
  })

  it('shows when the system last changed on the collapsed row', () => {
    const wrapper = mount(SystemCard, { props: { system: makeSystem() } })

    expect(wrapper.get('.system-card__updated time').attributes('datetime')).toBe(
      '2026-08-18T09:02:00Z',
    )
  })

  it('omits the timestamp for a system that has never reported anything', () => {
    const wrapper = mount(SystemCard, { props: { system: makeSystem({ last_updated_at: null }) } })

    expect(wrapper.find('.system-card__updated').exists()).toBe(false)
  })
})
