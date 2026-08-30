import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import AppButton from './AppButton.vue'

describe('AppButton', () => {
  it('renders its label', () => {
    const wrapper = mount(AppButton, { props: { label: 'Refresh' } })

    expect(wrapper.text()).toBe('Refresh')
  })

  it('is a non-submitting button', () => {
    const wrapper = mount(AppButton, { props: { label: 'Refresh' } })

    expect(wrapper.attributes('type')).toBe('button')
  })

  it('emits click when pressed', async () => {
    const wrapper = mount(AppButton, { props: { label: 'Refresh' } })

    await wrapper.trigger('click')

    expect(wrapper.emitted('click')).toHaveLength(1)
  })
})
