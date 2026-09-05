import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import AppModal from './AppModal.vue'

const mountModal = () =>
  mount(AppModal, {
    props: { title: 'Add New Status Integration' },
    slots: {
      default: '<p class="body-content">fields go here</p>',
      actions: '<button>Save</button>',
    },
    attachTo: document.body,
  })

describe('AppModal', () => {
  it('shows its title and slotted content', () => {
    const wrapper = mountModal()

    expect(wrapper.get('.app-modal__title').text()).toBe('Add New Status Integration')
    expect(wrapper.find('.body-content').exists()).toBe(true)
    expect(wrapper.get('.app-modal__actions').text()).toContain('Save')

    wrapper.unmount()
  })

  it('presents itself as a modal dialog', () => {
    const wrapper = mountModal()

    const panel = wrapper.get('.app-modal__panel')
    expect(panel.attributes('role')).toBe('dialog')
    expect(panel.attributes('aria-modal')).toBe('true')
    expect(panel.attributes('aria-label')).toBe('Add New Status Integration')

    wrapper.unmount()
  })

  it('closes on Escape', async () => {
    const wrapper = mountModal()

    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }))
    await wrapper.vm.$nextTick()

    expect(wrapper.emitted('close')).toHaveLength(1)
    wrapper.unmount()
  })

  it('ignores other keys', async () => {
    const wrapper = mountModal()

    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'a' }))
    await wrapper.vm.$nextTick()

    expect(wrapper.emitted('close')).toBeUndefined()
    wrapper.unmount()
  })

  // Clicking the backdrop would be an easy way to lose a half-typed form, so it does not
  // close: the Close button is the deliberate way out.
  it('does not close when the backdrop is clicked', async () => {
    const wrapper = mountModal()

    await wrapper.get('.app-modal__backdrop').trigger('click')

    expect(wrapper.emitted('close')).toBeUndefined()
    wrapper.unmount()
  })

  it('stops listening for Escape once unmounted', async () => {
    const wrapper = mountModal()
    wrapper.unmount()

    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }))

    expect(wrapper.emitted('close')).toBeUndefined()
  })
})
