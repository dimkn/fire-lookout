import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import type { StatusItem } from '@/api/status'

import IncidentItem from './IncidentItem.vue'

function makeItem(overrides: Partial<StatusItem> = {}): StatusItem {
  return {
    id: 101,
    feed_id: 1,
    guid: 'gh-incident-3',
    title: 'Elevated error rates for Actions',
    link: 'https://www.githubstatus.com/incidents/abc123',
    published_at: '2026-08-18T08:41:00Z',
    updated_at: '2026-08-18T09:02:00Z',
    content_html: '<p>Update - investigating.</p>',
    content_text: 'Update - investigating.',
    status: 'investigating',
    fetched_at: '2026-08-18T09:05:00Z',
    ...overrides,
  }
}

describe('IncidentItem', () => {
  it('shows the incident title as a link to the provider', () => {
    const wrapper = mount(IncidentItem, { props: { item: makeItem() } })

    const link = wrapper.get('a')
    expect(link.text()).toBe('Elevated error rates for Actions')
    expect(link.attributes('href')).toBe('https://www.githubstatus.com/incidents/abc123')
    expect(link.attributes('rel')).toContain('noopener')
  })

  it('shows a plain title when the entry carries no permalink', () => {
    const wrapper = mount(IncidentItem, { props: { item: makeItem({ link: null }) } })

    expect(wrapper.find('a').exists()).toBe(false)
    expect(wrapper.text()).toContain('Elevated error rates for Actions')
  })

  it('labels the incident status', () => {
    const wrapper = mount(IncidentItem, { props: { item: makeItem({ status: 'monitoring' }) } })

    expect(wrapper.get('.incident__status').text()).toBe('Monitoring')
  })

  it('publishes the timestamp in a machine-readable time element', () => {
    const wrapper = mount(IncidentItem, { props: { item: makeItem() } })

    const time = wrapper.get('time')
    expect(time.attributes('datetime')).toBe('2026-08-18T08:41:00Z')
    expect(time.text()).toBe('18 Aug 2026, 08:41 UTC')
  })

  it('falls back to when we first saw an entry the provider never dated', () => {
    const wrapper = mount(IncidentItem, {
      props: { item: makeItem({ published_at: null, updated_at: null }) },
    })

    const time = wrapper.get('time')
    expect(time.attributes('datetime')).toBe('2026-08-18T09:05:00Z')
    expect(wrapper.text()).toContain('First seen')
  })

  it('renders the plaintext body', () => {
    const wrapper = mount(IncidentItem, {
      props: { item: makeItem({ content_text: 'A fix has been applied.' }) },
    })

    expect(wrapper.get('.incident__body').text()).toBe('A fix has been applied.')
  })

  it('omits the body entirely when there is no text', () => {
    const wrapper = mount(IncidentItem, {
      props: { item: makeItem({ content_text: null, content_html: null }) },
    })

    expect(wrapper.find('.incident__body').exists()).toBe(false)
  })

  // The body is remote content: it must never reach the DOM as markup.
  it('escapes markup in the body instead of injecting it', () => {
    const wrapper = mount(IncidentItem, {
      props: {
        item: makeItem({
          content_text: '<b>bold</b><script>alert(1)</script>',
          content_html: '<b>bold</b><script>alert(1)</script>',
        }),
      },
    })

    expect(wrapper.find('.incident__body b').exists()).toBe(false)
    expect(wrapper.find('.incident__body script').exists()).toBe(false)
    expect(wrapper.get('.incident__body').text()).toContain('<b>bold</b>')
  })
})
