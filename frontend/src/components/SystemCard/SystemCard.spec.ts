import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import type { StatusItem, SystemOverview } from '@/api/status'
import AppIconButton from '@/components/AppIconButton/AppIconButton.vue'
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

  describe('row actions', () => {
    it('offers edit, delete and an on/off toggle', () => {
      const wrapper = mount(SystemCard, { props: { system: makeSystem() } })

      const labels = wrapper.findAllComponents(AppIconButton).map((b) => b.props('label') as string)
      expect(labels).toEqual(['Edit integration', 'Delete integration', 'Turn polling off'])
    })

    it('draws them as icons, with no visible text', () => {
      const wrapper = mount(SystemCard, { props: { system: makeSystem() } })

      const actions = wrapper.get('.system-card__actions')
      expect(actions.findAll('svg')).toHaveLength(3)
      expect(actions.text()).toBe('')
    })

    // The chevron is gone: the actions occupy that end of the row now.
    it('no longer shows a chevron', () => {
      const wrapper = mount(SystemCard, { props: { system: makeSystem() } })

      expect(wrapper.find('.system-card__chevron').exists()).toBe(false)
      expect(wrapper.text()).not.toContain('▸')
      expect(wrapper.text()).not.toContain('▾')
    })

    // The whole point of taking the buttons out of the row button: pressing Delete must not
    // also open the card.
    it('does not expand the card when an action is pressed', async () => {
      const wrapper = mount(SystemCard, { props: { system: makeSystem() } })

      for (const button of wrapper.findAllComponents(AppIconButton)) {
        await button.trigger('click')
      }

      expect(wrapper.find('.system-card__detail').exists()).toBe(false)
      expect(wrapper.get('.system-card__row').attributes('aria-expanded')).toBe('false')
      expect(wrapper.emitted('expand')).toBeUndefined()
    })

    it('asks the page to pause a running system', async () => {
      const wrapper = mount(SystemCard, { props: { system: makeSystem() } })

      await wrapper.findAllComponents(AppIconButton)[2].trigger('click')

      expect(wrapper.emitted('setEnabled')).toEqual([[{ feedId: 1, enabled: false }]])
    })

    it('asks the page to resume a paused system', async () => {
      const system = makeSystem()
      system.feed.enabled = false
      const wrapper = mount(SystemCard, { props: { system } })

      await wrapper.findAllComponents(AppIconButton)[2].trigger('click')

      expect(wrapper.emitted('setEnabled')).toEqual([[{ feedId: 1, enabled: true }]])
    })

    // While the request is in flight the switch is unavailable, so a double-click cannot
    // send a second, contradictory request.
    it('disables the switch while the page is busy with this feed', async () => {
      const wrapper = mount(SystemCard, { props: { system: makeSystem(), busy: true } })

      const toggle = wrapper.findAllComponents(AppIconButton)[2]
      expect(toggle.props('disabled')).toBe(true)

      await toggle.trigger('click')
      expect(wrapper.emitted('setEnabled')).toBeUndefined()
    })

    it('shows the toggle as on for a polling system', () => {
      const wrapper = mount(SystemCard, { props: { system: makeSystem() } })

      const toggle = wrapper.findAllComponents(AppIconButton)[2]
      expect(toggle.props('pressed')).toBe(true)
      expect(toggle.props('label')).toBe('Turn polling off')
    })

    // Paused is a property of the system, not of one button, so the whole row reads as off.
    it('dims the entire row when the system is paused', () => {
      const system = makeSystem()
      system.feed.enabled = false
      const wrapper = mount(SystemCard, { props: { system } })

      expect(wrapper.get('.system-card__header').classes()).toContain('system-card__header--paused')
    })

    it('leaves the row undimmed while the system is polling', () => {
      const wrapper = mount(SystemCard, { props: { system: makeSystem() } })

      expect(wrapper.get('.system-card__header').classes()).not.toContain(
        'system-card__header--paused',
      )
    })

    // The row dim covers the actions too, so the toggle must not dim itself as well.
    it('leaves the paused look to the row rather than the toggle button', () => {
      const system = makeSystem()
      system.feed.enabled = false
      const wrapper = mount(SystemCard, { props: { system } })

      const toggle = wrapper.findAllComponents(AppIconButton)[2]
      expect(toggle.props('pressed')).toBe(false)
      expect(toggle.classes()).not.toContain('app-icon-button--off')
    })

    it('shows the toggle as off for a paused system', () => {
      const system = makeSystem()
      system.feed.enabled = false
      const wrapper = mount(SystemCard, { props: { system } })

      const toggle = wrapper.findAllComponents(AppIconButton)[2]
      expect(toggle.props('pressed')).toBe(false)
      expect(toggle.props('label')).toBe('Turn polling on')
    })
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

describe('SystemCard while paused', () => {
  function pausedSystem() {
    const system = makeSystem()
    system.feed.enabled = false
    return system
  }

  it('cannot be expanded', async () => {
    const wrapper = mount(SystemCard, { props: { system: pausedSystem() } })

    await wrapper.get('.system-card__row').trigger('click')

    expect(wrapper.find('.system-card__detail').exists()).toBe(false)
    expect(wrapper.emitted('expand')).toBeUndefined()
  })

  it('marks the row as unavailable rather than silently ignoring clicks', () => {
    const wrapper = mount(SystemCard, { props: { system: pausedSystem() } })

    expect(wrapper.get('.system-card__row').attributes('disabled')).toBeDefined()
  })

  // Pausing an open card closes it: its detail describes a state we have stopped tracking.
  it('collapses when it is paused while open', async () => {
    const wrapper = mount(SystemCard, { props: { system: makeSystem() } })
    await wrapper.get('.system-card__row').trigger('click')
    expect(wrapper.find('.system-card__detail').exists()).toBe(true)

    await wrapper.setProps({ system: pausedSystem() })

    expect(wrapper.find('.system-card__detail').exists()).toBe(false)
  })

  it('can be opened again once it is resumed', async () => {
    const wrapper = mount(SystemCard, { props: { system: pausedSystem() } })
    await wrapper.get('.system-card__row').trigger('click')
    expect(wrapper.find('.system-card__detail').exists()).toBe(false)

    await wrapper.setProps({ system: makeSystem() })
    await wrapper.get('.system-card__row').trigger('click')

    expect(wrapper.find('.system-card__detail').exists()).toBe(true)
  })
})
