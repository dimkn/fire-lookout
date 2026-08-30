import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import AppButton from '@/components/AppButton/AppButton.vue'

import PageToolbar from './PageToolbar.vue'

describe('PageToolbar', () => {
  it('offers refresh first and the add action second', () => {
    const wrapper = mount(PageToolbar)

    const labels = wrapper.findAllComponents(AppButton).map((button) => button.props('label'))

    expect(labels).toEqual(['Refresh', 'Add New Status Integration'])
  })

  it('emits refresh when the left button is pressed', async () => {
    const wrapper = mount(PageToolbar)

    await wrapper.findAllComponents(AppButton)[0].trigger('click')

    expect(wrapper.emitted('refresh')).toHaveLength(1)
    expect(wrapper.emitted('add')).toBeUndefined()
  })

  it('emits add when the right button is pressed', async () => {
    const wrapper = mount(PageToolbar)

    await wrapper.findAllComponents(AppButton)[1].trigger('click')

    expect(wrapper.emitted('add')).toHaveLength(1)
    expect(wrapper.emitted('refresh')).toBeUndefined()
  })
})
