import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import AppIconButton from './AppIconButton.vue'

const icon = '<svg class="test-icon" viewBox="0 0 16 16"></svg>'

describe('AppIconButton', () => {
  it('renders the slotted icon', () => {
    const wrapper = mount(AppIconButton, { props: { label: 'Edit' }, slots: { default: icon } })

    expect(wrapper.find('svg.test-icon').exists()).toBe(true)
  })

  // Icon-only: the label must reach assistive tech and the tooltip, never the canvas.
  it('carries the label as its accessible name and tooltip, not as text', () => {
    const wrapper = mount(AppIconButton, { props: { label: 'Edit' }, slots: { default: icon } })

    expect(wrapper.attributes('aria-label')).toBe('Edit')
    expect(wrapper.attributes('title')).toBe('Edit')
    expect(wrapper.text()).toBe('')
  })

  it('is a non-submitting button', () => {
    const wrapper = mount(AppIconButton, { props: { label: 'Edit' } })

    expect(wrapper.attributes('type')).toBe('button')
  })

  it('emits click when pressed', async () => {
    const wrapper = mount(AppIconButton, { props: { label: 'Edit' } })

    await wrapper.trigger('click')

    expect(wrapper.emitted('click')).toHaveLength(1)
  })

  describe('as a toggle', () => {
    it('reports itself pressed when on', () => {
      const wrapper = mount(AppIconButton, { props: { label: 'Turn off', pressed: true } })

      expect(wrapper.attributes('aria-pressed')).toBe('true')
    })

    it('reports itself unpressed when off', () => {
      const wrapper = mount(AppIconButton, { props: { label: 'Turn on', pressed: false } })

      expect(wrapper.attributes('aria-pressed')).toBe('false')
    })

    // A plain action button must not look like a toggle that happens to be off.
    it('omits aria-pressed entirely when it is not a toggle', () => {
      const wrapper = mount(AppIconButton, { props: { label: 'Edit' } })

      expect(wrapper.attributes('aria-pressed')).toBeUndefined()
    })
  })
})
