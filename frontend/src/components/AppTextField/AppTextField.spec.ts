import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import AppTextField from './AppTextField.vue'

describe('AppTextField', () => {
  it('labels the input', () => {
    const wrapper = mount(AppTextField, { props: { label: 'Name', modelValue: '' } })

    expect(wrapper.get('.app-text-field__label').text()).toBe('Name')
    const id = wrapper.get('input').attributes('id')
    expect(wrapper.get('label').attributes('for')).toBe(id)
  })

  it('shows the current value', () => {
    const wrapper = mount(AppTextField, { props: { label: 'Name', modelValue: 'GitHub' } })

    expect(wrapper.get('input').element.value).toBe('GitHub')
  })

  it('emits every keystroke so the caller can validate on change', async () => {
    const wrapper = mount(AppTextField, { props: { label: 'Name', modelValue: '' } })

    await wrapper.get('input').setValue('Git')

    expect(wrapper.emitted('update:modelValue')).toEqual([['Git']])
  })

  it('defaults to a text input but accepts another type', () => {
    const plain = mount(AppTextField, { props: { label: 'Name', modelValue: '' } })
    const url = mount(AppTextField, { props: { label: 'RSS link', modelValue: '', type: 'url' } })

    expect(plain.get('input').attributes('type')).toBe('text')
    expect(url.get('input').attributes('type')).toBe('url')
  })
})
