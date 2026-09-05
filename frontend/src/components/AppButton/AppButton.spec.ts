import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import DotsLoader from '@/components/DotsLoader/DotsLoader.vue'

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

  describe('when disabled', () => {
    const disabled = { props: { label: 'Save', disabled: true } }

    it('carries the disabled attribute', () => {
      const wrapper = mount(AppButton, disabled)

      expect(wrapper.attributes('disabled')).toBeDefined()
    })

    it('does not emit click', async () => {
      const wrapper = mount(AppButton, disabled)

      await wrapper.trigger('click')

      expect(wrapper.emitted('click')).toBeUndefined()
    })

    // Disabled is not busy: no loader, and nothing is in flight.
    it('shows no loader', () => {
      const wrapper = mount(AppButton, disabled)

      expect(wrapper.findComponent(DotsLoader).exists()).toBe(false)
      expect(wrapper.attributes('aria-busy')).toBe('false')
    })
  })

  it('shows no loader and stays enabled by default', () => {
    const wrapper = mount(AppButton, { props: { label: 'Refresh' } })

    expect(wrapper.findComponent(DotsLoader).exists()).toBe(false)
    expect(wrapper.attributes('disabled')).toBeUndefined()
    expect(wrapper.attributes('aria-busy')).toBe('false')
  })

  describe('while loading', () => {
    const loading = { props: { label: 'Refresh', loading: true } }

    it('swaps the text for the animated loader', () => {
      const wrapper = mount(AppButton, loading)

      expect(wrapper.findComponent(DotsLoader).exists()).toBe(true)
      expect(wrapper.get('.app-button__label').classes()).toContain('app-button__label--loading')
    })

    // The label must stay in the DOM: it is what reserves the button's width, so the
    // button cannot change size when the loader appears, and it keeps the accessible name.
    it('keeps the label in the DOM to hold the button’s size', () => {
      const wrapper = mount(AppButton, loading)

      expect(wrapper.get('.app-button__label').text()).toBe('Refresh')
    })

    it('reports itself as busy and disabled', () => {
      const wrapper = mount(AppButton, loading)

      expect(wrapper.attributes('aria-busy')).toBe('true')
      expect(wrapper.attributes('disabled')).toBeDefined()
    })

    it('does not emit click', async () => {
      const wrapper = mount(AppButton, loading)

      await wrapper.trigger('click')

      expect(wrapper.emitted('click')).toBeUndefined()
    })
  })
})
