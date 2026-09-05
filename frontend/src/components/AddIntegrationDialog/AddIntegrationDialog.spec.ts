import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import AppButton from '@/components/AppButton/AppButton.vue'
import AppTextField from '@/components/AppTextField/AppTextField.vue'

import AddIntegrationDialog from './AddIntegrationDialog.vue'

function mountDialog(props: { saving?: boolean } = {}) {
  return mount(AddIntegrationDialog, { props, attachTo: document.body })
}

const buttons = (wrapper: ReturnType<typeof mountDialog>) => wrapper.findAllComponents(AppButton)
const saveButton = (wrapper: ReturnType<typeof mountDialog>) =>
  buttons(wrapper).find((b) => b.props('label') === 'Save')!
const closeButton = (wrapper: ReturnType<typeof mountDialog>) =>
  buttons(wrapper).find((b) => b.props('label') === 'Close')!

async function fill(wrapper: ReturnType<typeof mountDialog>, name: string, url: string) {
  const fields = wrapper.findAllComponents(AppTextField)
  await fields[0].get('input').setValue(name)
  await fields[1].get('input').setValue(url)
}

describe('AddIntegrationDialog', () => {
  it('asks for a name and an RSS link, and nothing else', () => {
    const wrapper = mountDialog()

    const labels = wrapper.findAllComponents(AppTextField).map((f) => f.props('label'))
    expect(labels).toEqual(['Name', 'RSS link'])

    wrapper.unmount()
  })

  it('offers exactly two buttons: Close and Save', () => {
    const wrapper = mountDialog()

    expect(buttons(wrapper).map((b) => b.props('label'))).toEqual(['Close', 'Save'])

    wrapper.unmount()
  })

  it('starts empty', () => {
    const wrapper = mountDialog()

    const values = wrapper.findAll('input').map((i) => (i.element as HTMLInputElement).value)
    expect(values).toEqual(['', ''])

    wrapper.unmount()
  })

  describe('Save availability', () => {
    it('is unavailable until both fields hold something', async () => {
      const wrapper = mountDialog()
      expect(saveButton(wrapper).props('disabled')).toBe(true)

      await fill(wrapper, 'GitHub', '')
      expect(saveButton(wrapper).props('disabled')).toBe(true)

      await fill(wrapper, '', 'https://a.test/feed.atom')
      expect(saveButton(wrapper).props('disabled')).toBe(true)

      await fill(wrapper, 'GitHub', 'https://a.test/feed.atom')
      expect(saveButton(wrapper).props('disabled')).toBe(false)

      wrapper.unmount()
    })

    // Whitespace is not content — the backend would reject it, so the button stays off.
    it('treats whitespace-only values as empty', async () => {
      const wrapper = mountDialog()

      await fill(wrapper, '   ', '  \t ')

      expect(saveButton(wrapper).props('disabled')).toBe(true)
      wrapper.unmount()
    })

    it('re-checks on every change, including going back to empty', async () => {
      const wrapper = mountDialog()
      await fill(wrapper, 'GitHub', 'https://a.test/feed.atom')
      expect(saveButton(wrapper).props('disabled')).toBe(false)

      await fill(wrapper, 'GitHub', '')

      expect(saveButton(wrapper).props('disabled')).toBe(true)
      wrapper.unmount()
    })
  })

  it('submits trimmed values', async () => {
    const wrapper = mountDialog()
    await fill(wrapper, '  GitHub  ', '  https://a.test/feed.atom  ')

    await saveButton(wrapper).trigger('click')

    expect(wrapper.emitted('submit')).toEqual([
      [{ title: 'GitHub', url: 'https://a.test/feed.atom' }],
    ])
    wrapper.unmount()
  })

  it('does not submit while incomplete', async () => {
    const wrapper = mountDialog()
    await fill(wrapper, 'GitHub', '')

    await saveButton(wrapper).trigger('click')

    expect(wrapper.emitted('submit')).toBeUndefined()
    wrapper.unmount()
  })

  it('emits close when Close is pressed, without submitting', async () => {
    const wrapper = mountDialog()
    await fill(wrapper, 'GitHub', 'https://a.test/feed.atom')

    await closeButton(wrapper).trigger('click')

    expect(wrapper.emitted('close')).toHaveLength(1)
    expect(wrapper.emitted('submit')).toBeUndefined()
    wrapper.unmount()
  })

  describe('while saving', () => {
    it('puts the Save button — and only it — into the loading state', async () => {
      const wrapper = mountDialog({ saving: true })
      await fill(wrapper, 'GitHub', 'https://a.test/feed.atom')

      expect(saveButton(wrapper).props('loading')).toBe(true)
      expect(closeButton(wrapper).props('loading')).toBe(false)

      wrapper.unmount()
    })

    it('does not submit again', async () => {
      const wrapper = mountDialog({ saving: true })
      await fill(wrapper, 'GitHub', 'https://a.test/feed.atom')

      await saveButton(wrapper).trigger('click')

      expect(wrapper.emitted('submit')).toBeUndefined()
      wrapper.unmount()
    })

    // The modal stays put on failure, so Close must remain usable.
    it('leaves Close usable', () => {
      const wrapper = mountDialog({ saving: true })

      expect(closeButton(wrapper).props('disabled')).toBe(false)

      wrapper.unmount()
    })
  })
})
