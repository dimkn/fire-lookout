import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import AppSelect from './AppSelect.vue'

const options = [
  { label: '30 seconds', value: 30 },
  { label: '1 minute', value: 60 },
  { label: '5 minutes', value: 300 },
]

const mountSelect = (modelValue = 300) =>
  mount(AppSelect, { props: { label: 'Check every', modelValue, options } })

describe('AppSelect', () => {
  it('labels the select', () => {
    const wrapper = mountSelect()

    expect(wrapper.get('.app-select__label').text()).toBe('Check every')
    expect(wrapper.get('label').attributes('for')).toBe(wrapper.get('select').attributes('id'))
  })

  it('renders every option in order', () => {
    const wrapper = mountSelect()

    const rendered = wrapper.findAll('option').map((o) => ({
      label: o.text(),
      value: o.attributes('value'),
    }))
    expect(rendered).toEqual([
      { label: '30 seconds', value: '30' },
      { label: '1 minute', value: '60' },
      { label: '5 minutes', value: '300' },
    ])
  })

  it('shows the current value as selected', () => {
    const wrapper = mountSelect(60)

    expect((wrapper.get('select').element as HTMLSelectElement).value).toBe('60')
  })

  // The DOM hands back strings; callers want the number they put in. Only an offered value
  // can ever be chosen, which is why no parsing fallback is needed here.
  it('emits the chosen value as a number', async () => {
    const wrapper = mountSelect()

    await wrapper.get('select').setValue('60')

    expect(wrapper.emitted('update:modelValue')).toEqual([[60]])
  })

  it('emits each option that is picked', async () => {
    const wrapper = mountSelect()

    await wrapper.get('select').setValue('30')
    await wrapper.get('select').setValue('300')

    expect(wrapper.emitted('update:modelValue')).toEqual([[30], [300]])
  })
})
