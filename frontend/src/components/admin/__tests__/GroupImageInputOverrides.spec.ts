import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'

import GroupImageInputOverrides from '../GroupImageInputOverrides.vue'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return { ...actual, useI18n: () => ({ t: (key: string) => key }) }
})

describe('GroupImageInputOverrides', () => {
  it('adds an exact model, allows disabling, and removes the override with Auto', async () => {
    const wrapper = mount(GroupImageInputOverrides, {
      props: { modelValue: {}, candidates: ['deepseek-flash'], candidateListId: 'candidate-models' },
    })
    await wrapper.find('input').setValue('deepseek-flash')
    await wrapper.find('button').trigger('click')
    expect(wrapper.emitted('update:modelValue')?.[0]).toEqual([{ 'deepseek-flash': true }])

    await wrapper.setProps({ modelValue: { 'deepseek-flash': true } })
    await wrapper.find('select').setValue('disabled')
    expect(wrapper.emitted('update:modelValue')?.[1]).toEqual([{ 'deepseek-flash': false }])

    await wrapper.setProps({ modelValue: { 'deepseek-flash': false } })
    await wrapper.find('select').setValue('auto')
    expect(wrapper.emitted('update:modelValue')?.[2]).toEqual([{}])
  })

  it('rejects wildcard model names', async () => {
    const wrapper = mount(GroupImageInputOverrides, {
      props: { modelValue: {}, candidates: [], candidateListId: 'candidate-models' },
    })
    await wrapper.find('input').setValue('deepseek-*')
    await wrapper.find('button').trigger('click')
    expect(wrapper.emitted('update:modelValue')).toBeUndefined()
    expect(wrapper.find('[role="alert"]').exists()).toBe(true)
  })
})
