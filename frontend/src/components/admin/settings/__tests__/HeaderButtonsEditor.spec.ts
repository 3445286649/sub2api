import { defineComponent, ref } from 'vue'
import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import HeaderButtonsEditor from '../HeaderButtonsEditor.vue'
import type { HeaderButton } from '@/types'

vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))

describe('header button editor', () => {
  it('edits, disables, sorts and deletes custom buttons', async () => {
    const wrapper = mount(defineComponent({
      components: { HeaderButtonsEditor },
      setup: () => ({ buttons: ref<HeaderButton[]>([{ action: 'help', name: 'Help', url: '', enabled: true }]) }),
      template: '<HeaderButtonsEditor v-model="buttons" />',
    }))
    await wrapper.get('button.btn').trigger('click')
    await wrapper.findAll('input')[2].setValue('Status')
    await wrapper.findAll('input')[3].setValue('https://example.com')
    await wrapper.findAll('[role="switch"]')[1].trigger('click')
    await wrapper.findAll('[aria-label="headerButtons.up"]')[1].trigger('click')
    expect(wrapper.findAll('input')[0].element.value).toBe('Status')
    expect(wrapper.findAll('[role="switch"]')[0].attributes('aria-checked')).toBe('false')
    await wrapper.get('[aria-label="common.delete"]').trigger('click')
    expect(wrapper.findAll('input')).toHaveLength(2)
    expect(wrapper.findAll('input')[0].element.value).toBe('Help')
    wrapper.unmount()
  })
})
