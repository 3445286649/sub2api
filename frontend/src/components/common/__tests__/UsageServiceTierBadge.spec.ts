import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'
import UsageServiceTierBadge from '../UsageServiceTierBadge.vue'

const i18n = createI18n({ legacy: false, locale: 'zh', messages: { zh: { usage: { billedFast: () => 'Fast 计费', billedUltrafast: () => 'Ultrafast 计费', freeFastEnabled: () => 'Fast 已开启，免费加速 · 按标准档计费', freeUltrafastEnabled: () => 'Ultrafast 已开启，免费加速 · 按标准档计费' } } } })

describe('UsageServiceTierBadge', () => {
  it.each(['priority', 'ultrafast'])('shows enabled free acceleration without claiming accelerated billing: %s', async (tier) => {
    const wrapper = mount(UsageServiceTierBadge, { props: { tier, free: true }, global: { plugins: [i18n], stubs: { Teleport: true } } })
    const label = `${tier === 'ultrafast' ? 'Ultrafast' : 'Fast'} 已开启，免费加速 · 按标准档计费`
    expect(wrapper.get('button').attributes('aria-label')).toBe(label)
    await wrapper.get('button').trigger('click')
    expect(wrapper.get('[role="tooltip"]').text()).toBe(label)
    wrapper.unmount()
  })
  it.each(['priority', 'fast', 'ultrafast'])('uses the customer tier and supports mouse, touch and keyboard: %s', async (tier) => {
    const wrapper = mount(UsageServiceTierBadge, { props: { tier }, global: { plugins: [i18n], stubs: { Teleport: true } } })
    const button = wrapper.get('button')
    const label = tier === 'ultrafast' ? 'Ultrafast 计费' : 'Fast 计费'
    expect(button.attributes('aria-label')).toBe(label)
    await button.trigger('mouseenter')
    expect(wrapper.get('[role="tooltip"]').text()).toBe(label)
    await button.trigger('mouseleave')
    expect(wrapper.find('[role="tooltip"]').exists()).toBe(false)
    await button.trigger('click')
    expect(wrapper.get('[role="tooltip"]').text()).toBe(label)
    await button.trigger('keydown', { key: 'Escape' })
    expect(wrapper.find('[role="tooltip"]').exists()).toBe(false)
    await button.trigger('focus')
    expect(wrapper.find('[role="tooltip"]').exists()).toBe(true)
    await button.trigger('blur')
    expect(wrapper.find('[role="tooltip"]').exists()).toBe(false)
    wrapper.unmount()
  })

  it.each([undefined, null, '', 'standard', 'default', 'flex', 'unknown'])('does not invent a badge for %s', (tier) => {
    const wrapper = mount(UsageServiceTierBadge, { props: { tier }, global: { plugins: [i18n] } })
    expect(wrapper.find('button').exists()).toBe(false)
    wrapper.unmount()
  })

  it('uses independent masks when multiple Ultrafast rows are rendered', () => {
    const wrapper = mount({ components: { UsageServiceTierBadge }, template: '<div><UsageServiceTierBadge tier="ultrafast"/><UsageServiceTierBadge tier="ultrafast"/></div>' }, { global: { plugins: [i18n] } })
    const ids = wrapper.findAll('mask').map(mask => mask.attributes('id'))
    expect(new Set(ids).size).toBe(2)
    wrapper.unmount()
  })
})
