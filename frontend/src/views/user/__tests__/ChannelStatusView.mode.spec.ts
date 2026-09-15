import { describe, expect, it, vi, beforeEach } from 'vitest'
import { defineComponent, h } from 'vue'
import { mount } from '@vue/test-utils'

const mode = vi.fn(() => 'v3')
const query: Record<string, string> = {}
vi.mock('vue-router', () => ({ useRoute: () => ({ query }) }))

vi.mock('@/utils/featureFlags', () => ({
  getChannelMonitorMode: () => mode(),
}))

vi.mock('../ChannelStatusV1View.vue', () => ({
  default: defineComponent({ name: 'ChannelStatusV1View', setup: () => () => h('div', { 'data-testid': 'v1' }) }),
}))
vi.mock('../ChannelStatusV2View.vue', () => ({
  default: defineComponent({ name: 'ChannelStatusV2View', setup: () => () => h('div', { 'data-testid': 'v2' }) }),
}))

import ChannelStatusView from '../ChannelStatusView.vue'
vi.mock('../ChannelStatusV3View.vue', () => ({
  default: defineComponent({ name: 'ChannelStatusV3View', setup: () => () => h('div', { 'data-testid': 'v3' }) }),
}))

describe('ChannelStatusView mode switch', () => {
  beforeEach(() => {
    mode.mockReset()
    delete query.monitor_view
  })

  it('renders independent V3 for passive monitoring', () => {
    mode.mockReturnValue('v3')
    const wrapper = mount(ChannelStatusView)
    expect(wrapper.find('[data-testid="v3"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="v2"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="v1"]').exists()).toBe(false)
  })

  it('retains V2 as an explicit analysis view', () => {
    mode.mockReturnValue('v2')
    query.monitor_view = 'v2'
    const wrapper = mount(ChannelStatusView)
    expect(wrapper.find('[data-testid="v2"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="v3"]').exists()).toBe(false)
  })

  it('renders V1 when in v1 mode', () => {
    mode.mockReturnValue('v1')
    const wrapper = mount(ChannelStatusView)
    expect(wrapper.find('[data-testid="v1"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="v2"]').exists()).toBe(false)
  })
})
