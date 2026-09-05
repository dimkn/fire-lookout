import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import DotsLoader from './DotsLoader.vue'

describe('DotsLoader', () => {
  it('renders three dots', () => {
    const wrapper = mount(DotsLoader)

    expect(wrapper.findAll('.dots-loader__dot')).toHaveLength(3)
  })

  // Each dot hops once per cycle, and a dot starts exactly when the previous one lands:
  // delays are one hop apart, and the cycle lasts three hops.
  it('staggers each dot by one hop so they jump in sequence', () => {
    const wrapper = mount(DotsLoader)

    const dots = wrapper.findAll('.dots-loader__dot').map((dot) => dot.element as HTMLElement)
    const delays = dots.map((dot) => dot.style.animationDelay)
    const durations = dots.map((dot) => dot.style.animationDuration)

    expect(delays).toEqual(['0ms', '400ms', '800ms'])
    expect(durations).toEqual(['1200ms', '1200ms', '1200ms'])
  })

  it('is decorative: the surrounding control carries the meaning', () => {
    const wrapper = mount(DotsLoader)

    expect(wrapper.attributes('aria-hidden')).toBe('true')
  })
})
