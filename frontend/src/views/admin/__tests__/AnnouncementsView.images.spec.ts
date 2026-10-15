import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import AnnouncementsView from '../AnnouncementsView.vue'

const { uploadImage, showError } = vi.hoisted(() => ({ uploadImage: vi.fn(), showError: vi.fn() }))
vi.mock('vue-i18n', async () => ({
  ...await vi.importActual<typeof import('vue-i18n')>('vue-i18n'),
  useI18n: () => ({ t: (key: string) => key }),
}))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showError, showSuccess: vi.fn() }) }))
vi.mock('@/composables/usePersistedPageSize', () => ({ getPersistedPageSize: () => 20 }))
vi.mock('@/api/admin', () => ({ adminAPI: {
  groups: { getAll: vi.fn().mockResolvedValue([]) },
  announcements: {
    list: vi.fn().mockResolvedValue({ items: [], total: 0, pages: 0, page: 1, page_size: 20 }),
    uploadImage,
  },
} }))

function mountEditor() {
  return mount(AnnouncementsView, { global: { stubs: {
    AppLayout: { template: '<div><slot /></div>' },
    TablePageLayout: { template: '<div><slot name="filters"/><slot name="table"/><slot name="pagination"/></div>' },
    BaseDialog: { props: ['show'], template: '<div v-if="show"><slot/><slot name="footer"/></div>' },
    DataTable: true, Select: true, Pagination: true, EmptyState: true,
    AnnouncementTargetingEditor: true, AnnouncementReadStatusDialog: true,
    AnnouncementPopup: true, ConfirmDialog: true, Icon: true,
  } } })
}

describe('announcement image editing', () => {
  beforeEach(() => { vi.clearAllMocks() })

  it('uploads a pasted image, inserts its URL and leaves text paste alone', async () => {
    uploadImage.mockResolvedValue('/api/v1/announcement-images/abc.png')
    const wrapper = mountEditor()
    await flushPromises()
    const createButton = wrapper.findAll('button').find(button => button.text().includes('createAnnouncement'))!
    await createButton.trigger('click')
    const textarea = wrapper.get('textarea').element
    const file = new File(['image'], 'sample.png', { type: 'image/png' })
    const paste = new Event('paste', { bubbles: true, cancelable: true })
    Object.defineProperty(paste, 'clipboardData', { value: { items: [{ kind: 'file', type: 'image/png', getAsFile: () => file }] } })
    textarea.dispatchEvent(paste)
    await flushPromises()
    expect(paste.defaultPrevented).toBe(true)
    expect(uploadImage).toHaveBeenCalledWith(file, expect.any(AbortSignal))
    expect(wrapper.get('textarea').element.value).toBe('![sample.png](/api/v1/announcement-images/abc.png)')

    const textPaste = new Event('paste', { bubbles: true, cancelable: true })
    Object.defineProperty(textPaste, 'clipboardData', { value: { items: [{ kind: 'string', type: 'text/plain' }] } })
    textarea.dispatchEvent(textPaste)
    expect(textPaste.defaultPrevented).toBe(false)
    expect(uploadImage).toHaveBeenCalledTimes(1)
    wrapper.unmount()
  })

  it('does not insert an image after the editor closes during upload', async () => {
    let complete!: (url: string) => void
    uploadImage.mockImplementation(() => new Promise<string>(resolve => { complete = resolve }))
    const wrapper = mountEditor()
    await flushPromises()
    await wrapper.findAll('button').find(button => button.text().includes('createAnnouncement'))!.trigger('click')
    const input = wrapper.get('input[type="file"]')
    Object.defineProperty(input.element, 'files', { configurable: true, value: [new File(['image'], 'sample.png', { type: 'image/png' })] })
    await input.trigger('change')
    await wrapper.findAll('button').find(button => button.text().includes('common.cancel'))!.trigger('click')
    complete('/api/v1/announcement-images/late.png')
    await flushPromises()
    expect(wrapper.find('textarea').exists()).toBe(false)
    expect(showError).not.toHaveBeenCalled()
    wrapper.unmount()
  })
})
