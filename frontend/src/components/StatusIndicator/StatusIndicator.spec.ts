import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import type { Indicator } from '@/api/status'

import StatusIndicator from './StatusIndicator.vue'

describe('StatusIndicator', () => {
  const cases: Array<[Indicator, string]> = [
    ['operational', 'Operational'],
    ['degraded', 'Degraded'],
    ['outage', 'Outage'],
    ['unknown', 'Unknown'],
  ]

  it.each(cases)('renders %s with its own modifier class and label', (indicator, label) => {
    const wrapper = mount(StatusIndicator, { props: { indicator } })

    expect(wrapper.classes()).toContain(`status-indicator--${indicator}`)
    expect(wrapper.attributes('aria-label')).toBe(label)
    expect(wrapper.attributes('title')).toBe(label)
  })

  it('exposes itself to assistive tech as an image, not decoration', () => {
    const wrapper = mount(StatusIndicator, { props: { indicator: 'outage' } })

    expect(wrapper.attributes('role')).toBe('img')
  })
})
