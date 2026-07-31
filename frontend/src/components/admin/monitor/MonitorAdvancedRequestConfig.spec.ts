import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'

import MonitorAdvancedRequestConfig from './MonitorAdvancedRequestConfig.vue'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({ t: (key: string) => key }),
}))

describe('MonitorAdvancedRequestConfig Responses transport', () => {
  it('enables streaming through the existing merge body override', async () => {
    const wrapper = mount(MonitorAdvancedRequestConfig, {
      props: {
        provider: 'openai',
        apiMode: 'responses',
        extraHeaders: {},
        bodyOverrideMode: 'off',
        bodyOverride: null,
      },
    })

    await wrapper.get('[data-testid="monitor-response-stream-on"]').trigger('click')

    expect(wrapper.emitted('update:bodyOverrideMode')?.at(-1)).toEqual(['merge'])
    expect(wrapper.emitted('update:bodyOverride')?.at(-1)).toEqual([{ stream: true }])
  })

  it('persists an explicit non-streaming selection without dropping template fields', async () => {
    const wrapper = mount(MonitorAdvancedRequestConfig, {
      props: {
        provider: 'openai',
        apiMode: 'responses',
        extraHeaders: {},
        bodyOverrideMode: 'merge',
        bodyOverride: { stream: true, max_output_tokens: 128 },
      },
    })

    await wrapper.get('[data-testid="monitor-response-stream-off"]').trigger('click')

    expect(wrapper.emitted('update:bodyOverrideMode')).toBeUndefined()
    expect(wrapper.emitted('update:bodyOverride')?.at(-1)).toEqual([
      { stream: false, max_output_tokens: 128 },
    ])
  })

  it('does not expose the Responses transport control for Chat Completions', () => {
    const wrapper = mount(MonitorAdvancedRequestConfig, {
      props: {
        provider: 'openai',
        apiMode: 'chat_completions',
        extraHeaders: {},
        bodyOverrideMode: 'off',
        bodyOverride: null,
      },
    })

    expect(wrapper.find('[data-testid="monitor-response-stream-on"]').exists()).toBe(false)
  })
})
