import { afterEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import AnnouncementMarkdown from '../AnnouncementMarkdown.vue'

vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))

afterEach(() => { document.body.innerHTML = '' })

describe('AnnouncementMarkdown', () => {
  it('keeps consecutive announcement images as square thumbnails beside each other', () => {
    const wrapper = mount(AnnouncementMarkdown, {
      props: { content: 'Before\n\n![One](/one.png)\n\n![Two](/two.png)\n![Three](/three.png)\n\nAfter' },
    })

    expect(wrapper.findAll('.announcement-image-grid')).toHaveLength(1)
    expect(wrapper.findAll('.announcement-image-grid img')).toHaveLength(3)
    expect(wrapper.findAll('.markdown-body > p').map(paragraph => paragraph.text())).toEqual(['Before', 'After'])
    wrapper.unmount()
  })

  it('opens uploaded images in a bounded preview and keeps text safe', async () => {
    const wrapper = mount(AnnouncementMarkdown, {
      props: { content: '![Example](/api/v1/announcement-images/123.png)\n<script>window.bad = true</script>' },
      attachTo: document.body,
    })
    const image = wrapper.get('.markdown-body img')
    expect(image.attributes('tabindex')).toBe('0')
    expect(wrapper.find('script').exists()).toBe(false)

    await image.trigger('click')
    expect(document.body.querySelector('[role="dialog"] img')?.getAttribute('src')).toContain('/api/v1/announcement-images/123.png')
    expect(document.body.querySelector('[role="dialog"] img')?.className).toContain('max-h-[90vh]')
    document.body.querySelector<HTMLButtonElement>('[role="dialog"] button')?.click()
    await wrapper.vm.$nextTick()
    expect(document.body.querySelector('[role="dialog"]')).toBeNull()
    wrapper.unmount()
  })
})
