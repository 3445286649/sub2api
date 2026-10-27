import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import AccountTestModal from '../AccountTestModal.vue'

const { getAvailableModels, copyToClipboard } = vi.hoisted(() => ({
  getAvailableModels: vi.fn(),
  copyToClipboard: vi.fn()
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    accounts: {
      getAvailableModels
    }
  }
}))

vi.mock('@/composables/useClipboard', () => ({
  useClipboard: () => ({
    copyToClipboard
  })
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  const messages: Record<string, string> = {
    'admin.accounts.imagePromptDefault': 'Generate a cute orange cat astronaut sticker on a clean pastel background.'
  }
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, string | number>) => {
        if (key === 'admin.accounts.imageReceived' && params?.count) {
          return `received-${params.count}`
        }
        if (key === 'admin.accounts.imagePreviewAlt' && params?.index) {
          return `test-image-${params.index}`
        }
        if (key === 'admin.accounts.pelicanReceiving' && params?.count) {
          return `received-chars-${params.count}`
        }
        return messages[key] || key
      }
    })
  }
})

function createStreamResponse(lines: string[]) {
  const encoder = new TextEncoder()
  const chunks = lines.map((line) => encoder.encode(line))
  let index = 0

  return {
    ok: true,
    body: {
      getReader: () => ({
        read: vi.fn().mockImplementation(async () => {
          if (index < chunks.length) {
            return { done: false, value: chunks[index++] }
          }
          return { done: true, value: undefined }
        })
      })
    }
  } as Response
}

function mountModal(account: Record<string, unknown> = {
  id: 42,
  name: 'Gemini Image Test',
  platform: 'gemini',
  type: 'apikey',
  status: 'active'
}) {
  return mount(AccountTestModal, {
    props: {
      show: false,
      account
    } as any,
    global: {
      stubs: {
        BaseDialog: {
          props: ['width', 'mobileSheet'],
          template: '<div :data-dialog-width="width" :data-mobile-sheet="mobileSheet ? \'true\' : \'false\'"><slot /><slot name="footer" /></div>'
        },
        Select: {
          props: ['modelValue', 'options', 'valueKey', 'labelKey'],
          emits: ['update:modelValue'],
          template: `<select class="select-stub" :value="modelValue" @change="$emit('update:modelValue', $event.target.value)"><option v-for="option in options" :key="option[valueKey || 'value']" :value="option[valueKey || 'value']">{{ option[labelKey || 'label'] }}</option></select>`
        },
        TextArea: {
          props: ['modelValue'],
          emits: ['update:modelValue'],
          template: '<textarea class="textarea-stub" :value="modelValue" @input="$emit(\'update:modelValue\', $event.target.value)" />'
        },
        Icon: true
      }
    }
  })
}

describe('AccountTestModal', () => {
  beforeEach(() => {
    getAvailableModels.mockResolvedValue([
      { id: 'gemini-2.0-flash', display_name: 'Gemini 2.0 Flash' },
      { id: 'gemini-2.5-flash-image', display_name: 'Gemini 2.5 Flash Image' },
      { id: 'gemini-3.1-flash-image', display_name: 'Gemini 3.1 Flash Image' }
    ])
    copyToClipboard.mockReset()
    Object.defineProperty(globalThis, 'localStorage', {
      value: {
        getItem: vi.fn((key: string) => (key === 'auth_token' ? 'test-token' : null)),
        setItem: vi.fn(),
        removeItem: vi.fn(),
        clear: vi.fn()
      },
      configurable: true
    })
    global.fetch = vi.fn().mockResolvedValue(
      createStreamResponse([
        'data: {"type":"test_start","model":"gemini-2.5-flash-image"}\n',
        'data: {"type":"image","image_url":"data:image/png;base64,QUJD","mime_type":"image/png"}\n',
        'data: {"type":"test_complete","success":true}\n'
      ])
    ) as any
  })

  afterEach(() => {
    vi.restoreAllMocks()
    vi.unstubAllGlobals()
    vi.useRealTimers()
  })

  it('gemini 图片模型测试会携带提示词并渲染图片预览', async () => {
    const wrapper = mountModal()
    await wrapper.setProps({ show: true })
    await flushPromises()
    expect(wrapper.find('[data-mobile-sheet="true"]').exists()).toBe(true)

    const promptInput = wrapper.find('textarea.textarea-stub')
    expect(promptInput.exists()).toBe(true)
    await promptInput.setValue('draw a tiny orange cat astronaut')

    const buttons = wrapper.findAll('button')
    const startButton = buttons.find((button) => button.text().includes('admin.accounts.startTest'))
    expect(startButton).toBeTruthy()

    await startButton!.trigger('click')
    await flushPromises()
    await flushPromises()

    expect(global.fetch).toHaveBeenCalledTimes(1)
    const [, request] = (global.fetch as any).mock.calls[0]
    expect(JSON.parse(request.body)).toEqual({
      model_id: 'gemini-3.1-flash-image',
      prompt: 'draw a tiny orange cat astronaut',
      mode: 'image'
    })

    const preview = wrapper.find('img[alt="test-image-1"]')
    expect(preview.exists()).toBe(true)
    expect(preview.attributes('src')).toBe('data:image/png;base64,QUJD')
  })

  it('runs the independent pelican test and shows a sandboxed SVG preview', async () => {
    getAvailableModels.mockResolvedValue([{ id: 'gemini-2.5-flash', display_name: 'Gemini Flash' }])
    global.fetch = vi.fn().mockResolvedValue(createStreamResponse([
      'data: {"type":"test_start","model":"gemini-2.5-flash"}\n',
      'data: {"type":"technical_details","data":{"method":"POST","path":"/v1beta/models/gemini-2.5-flash:streamGenerateContent","headers":{"x-goog-api-key":"***"}}}\n',
      `data: ${JSON.stringify({ type: 'content', text: '<svg xmlns="http://www.w3.org/2000/svg" width="800" height="640"><script>alert(1)</script><circle r="20"><animate attributeName="r" from="10" to="20" dur="1s" repeatCount="indefinite"/></circle></svg>' })}\n`,
      'data: {"type":"test_complete","success":true}\n'
    ])) as any
    const wrapper = mountModal()
    await wrapper.setProps({ show: true })
    await flushPromises()
    expect(wrapper.find('[data-mobile-sheet="true"]').exists()).toBe(true)
    await wrapper.findAll('button').find(button => button.text().includes('admin.accounts.pelicanTest'))!.trigger('click')
    expect(wrapper.find('[data-dialog-width="wide"]').exists()).toBe(true)
    expect(wrapper.find('textarea.textarea-stub').exists()).toBe(false)
    await wrapper.findAll('button').find(button => button.text().includes('admin.accounts.startTest'))!.trigger('click')
    await flushPromises()
    expect(JSON.parse((global.fetch as any).mock.calls[0][1].body)).toEqual({ model_id: 'gemini-2.5-flash', prompt: '', mode: 'pelican' })
    expect(wrapper.text()).toContain('/v1beta/models/gemini-2.5-flash:streamGenerateContent')
    const preview = wrapper.get('iframe[title="admin.accounts.pelicanPreview"]')
    expect(preview.attributes('sandbox')).toBe('')
    expect(preview.attributes('srcdoc')).toContain('<circle')
    expect(preview.attributes('srcdoc')).not.toContain('<script')
    expect(preview.attributes('srcdoc')).toContain('<animate')
    expect(preview.attributes('srcdoc')).toContain('viewBox="0 0 800 640"')
    expect(preview.attributes('srcdoc')).toMatch(/object-fit:\s*contain/)
    expect(preview.attributes('srcdoc')).toContain("script-src 'none'")
    expect(wrapper.text()).toContain('admin.accounts.pelicanReady')
    expect(wrapper.text()).not.toContain('<svg')
    expect(wrapper.get('details').attributes('open')).toBeUndefined()
  })

  it('previews animated HTML in a script-only sandbox with a restrictive CSP', async () => {
    getAvailableModels.mockResolvedValue([{ id: 'gemini-2.5-flash', display_name: 'Gemini Flash' }])
    const html = '<!doctype html><html><head><meta http-equiv="refresh" content="0;url=https://example.com"><base href="https://example.com/"><style>@keyframes ride { to { transform: translateX(10px) } }</style></head><body><svg viewBox="0 0 10 10"><circle r="4"/></svg><script>requestAnimationFrame(() => window.parent.document.body)</script></body></html>'
    global.fetch = vi.fn().mockResolvedValue(createStreamResponse([
      `data: ${JSON.stringify({ type: 'content', text: html })}\n`,
      `data: ${JSON.stringify({ type: 'test_complete', success: true })}\n`
    ])) as any
    const wrapper = mountModal()
    await wrapper.setProps({ show: true })
    await flushPromises()
    await wrapper.findAll('button').find(button => button.text().includes('admin.accounts.pelicanTest'))!.trigger('click')
    await wrapper.findAll('button').find(button => button.text().includes('admin.accounts.startTest'))!.trigger('click')
    await flushPromises()

    const preview = wrapper.get('iframe[title="admin.accounts.pelicanPreview"]')
    const srcdoc = preview.attributes('srcdoc')
    expect(preview.attributes('sandbox')).toBe('allow-scripts')
    expect(preview.attributes('sandbox')).not.toContain('allow-same-origin')
    expect(srcdoc).toContain('<script>requestAnimationFrame')
    expect(srcdoc).toContain("connect-src 'none'")
    expect(srcdoc).toContain('data-preview-layout="fit"')
    expect(srcdoc).toContain('overflow:hidden!important')
    expect(srcdoc).toContain('max-height:100vh!important')
    expect(srcdoc).not.toContain('http-equiv="refresh"')
    expect(srcdoc).not.toContain('<base')
    expect(wrapper.text()).toContain('admin.accounts.pelicanReady')
    expect(wrapper.text()).toContain('admin.accounts.downloadPelicanHtml')

    const createObjectURL = vi.fn((_blob: Blob) => 'blob:pelican-test')
    const revokeObjectURL = vi.fn()
    vi.stubGlobal('URL', { createObjectURL, revokeObjectURL })
    let downloadName = ''
    let downloadHref = ''
    vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(function () {
      downloadName = this.download
      downloadHref = this.href
    })
    vi.useFakeTimers()
    await wrapper.findAll('button').find(button => button.text().includes('admin.accounts.downloadPelicanHtml'))!.trigger('click')
    expect(createObjectURL).toHaveBeenCalledTimes(1)
    const downloadBlob = createObjectURL.mock.calls[0][0] as Blob
    expect(downloadBlob.type).toContain('text/html')
    expect(downloadBlob.size).toBe(new Blob([srcdoc]).size)
    expect(downloadName).toMatch(/^pelican-gemini-2\.5-flash-.*\.html$/)
    expect(downloadHref).toBe('blob:pelican-test')
    vi.runOnlyPendingTimers()
    expect(revokeObjectURL).toHaveBeenCalledWith('blob:pelican-test')
  })

  it('submits candy mode and renders the backend grading result independently of request success', async () => {
    getAvailableModels.mockResolvedValue([{ id: 'gemini-2.5-flash', display_name: 'Gemini Flash' }])
    global.fetch = vi.fn().mockResolvedValue(createStreamResponse([
      `data: ${JSON.stringify({
        type: 'content',
        text: JSON.stringify({
          minimum_draws: 20,
          round: 8,
          star: 12,
          proof: { round_watermelon_capacity: 8, star_capacity_without_apple: 10, star_capacity_without_peach: 11, one_less_round_quota: 8 }
        })
      })}\n`,
      `data: ${JSON.stringify({
        type: 'test_complete',
        success: true,
        data: { status: 'incorrect', minimum_draws: 21, parsed_answer: { minimum_draws: 21, round: 8, star: 13 }, explanation: 'The guaranteed shape counts are 9 and 12.' }
      })}\n`
    ])) as any
    const wrapper = mountModal()
    await wrapper.setProps({ show: true })
    await flushPromises()
    await wrapper.findAll('button').find(button => button.text().includes('admin.accounts.candyTest'))!.trigger('click')
    expect(wrapper.find('[data-dialog-width="wide"]').exists()).toBe(true)
    expect(wrapper.find('table').classes()).toContain('table-fixed')
    expect(wrapper.text()).toContain('admin.accounts.candyQuestion')
    expect(wrapper.text()).toContain('admin.accounts.candyWatermelon')
    expect(wrapper.text()).toContain('798')
    await wrapper.findAll('button').find(button => button.text().includes('admin.accounts.startTest'))!.trigger('click')
    await flushPromises()

    const [, request] = (global.fetch as any).mock.calls[0]
    expect(JSON.parse(request.body)).toEqual({ model_id: 'gemini-2.5-flash', prompt: '', mode: 'candy' })
    expect(wrapper.text()).toContain('admin.accounts.candyIncorrect')
    expect(wrapper.text()).toContain('The guaranteed shape counts are 9 and 12.')
    const candyVerdict = wrapper.findAll('[role="status"]').find(node => node.text().includes('The guaranteed shape counts are 9 and 12.'))
    expect(candyVerdict?.classes()).toContain('border-red-200')
    expect(wrapper.text()).not.toContain('admin.accounts.testFailed')
    expect(wrapper.text()).toContain('admin.accounts.candyMinimum')
    expect(wrapper.text()).toContain('admin.accounts.candyAnswerMinimum: 20')
    expect(wrapper.text()).toContain('admin.accounts.candyAnswerRound: 8')
    expect(wrapper.text()).toContain('admin.accounts.candyAnswerProof')
    expect(wrapper.text()).toContain('admin.accounts.candyProofRoundWatermelon: 8')
    expect(wrapper.text()).not.toContain('{"minimum_draws":20')
    const copyButton = wrapper.findAll('button').find(button => button.text().includes('admin.accounts.copyCandyAnswer'))!
    await copyButton.trigger('click')
    expect(copyToClipboard).toHaveBeenCalledWith(expect.stringContaining('admin.accounts.candyProofRoundWatermelon: 8'), 'admin.accounts.candyAnswerCopied')
  })

  it('shows the received character count while streaming without printing SVG source', async () => {
    getAvailableModels.mockResolvedValue([{ id: 'gemini-2.5-flash', display_name: 'Gemini Flash' }])
    const svgChunk = '<svg width="800" height="640">'
    let finishStream!: () => void
    const waiting = new Promise<void>(resolve => { finishStream = resolve })
    global.fetch = vi.fn().mockResolvedValue({
      ok: true,
      body: { getReader: () => {
        let sent = false
        return { read: async () => {
          if (!sent) {
            sent = true
            return { done: false, value: new TextEncoder().encode(`data: ${JSON.stringify({ type: 'content', text: svgChunk })}\n`) }
          }
          await waiting
          return { done: true }
        } }
      } }
    }) as any
    const wrapper = mountModal()
    await wrapper.setProps({ show: true })
    await flushPromises()
    await wrapper.findAll('button').find(button => button.text().includes('admin.accounts.pelicanTest'))!.trigger('click')
    const started = wrapper.findAll('button').find(button => button.text().includes('admin.accounts.startTest'))!.trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain('received-chars-')
    expect(wrapper.text()).toContain(String(svgChunk.length))
    expect(wrapper.text()).not.toContain(svgChunk)
    finishStream()
    await started
    await flushPromises()
    expect(wrapper.text()).toContain('admin.accounts.pelicanInterrupted')
    expect(wrapper.find('iframe[title="admin.accounts.pelicanPreview"]').exists()).toBe(false)
  })

  it('does not report a successful pelican image when the model returns plain text', async () => {
    getAvailableModels.mockResolvedValue([{ id: 'gemini-2.5-flash', display_name: 'Gemini Flash' }])
    global.fetch = vi.fn().mockResolvedValue(createStreamResponse([
      'data: {"type":"test_start","model":"gemini-2.5-flash"}\n',
      'data: {"type":"content","text":"I cannot draw that."}\n',
      'data: {"type":"test_complete","success":true}\n'
    ])) as any
    const wrapper = mountModal()
    await wrapper.setProps({ show: true })
    await flushPromises()
    await wrapper.findAll('button').find(button => button.text().includes('admin.accounts.pelicanTest'))!.trigger('click')
    await wrapper.findAll('button').find(button => button.text().includes('admin.accounts.startTest'))!.trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain('admin.accounts.noPelicanPreview')
    expect(wrapper.text()).not.toContain('admin.accounts.testCompleted')
    expect(wrapper.find('iframe[title="admin.accounts.pelicanPreview"]').exists()).toBe(false)
  })

  it('rejects an oversized pelican response instead of previewing truncated SVG', async () => {
    getAvailableModels.mockResolvedValue([{ id: 'gemini-2.5-flash', display_name: 'Gemini Flash' }])
    const oversized = `<svg width="800" height="640">${'x'.repeat(256 * 1024)}</svg>`
    global.fetch = vi.fn().mockResolvedValue(createStreamResponse([
      `data: ${JSON.stringify({ type: 'content', text: oversized })}\n`,
      'data: {"type":"test_complete","success":true}\n'
    ])) as any
    const wrapper = mountModal()
    await wrapper.setProps({ show: true })
    await flushPromises()
    await wrapper.findAll('button').find(button => button.text().includes('admin.accounts.pelicanTest'))!.trigger('click')
    await wrapper.findAll('button').find(button => button.text().includes('admin.accounts.startTest'))!.trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain('admin.accounts.pelicanTooLarge')
    expect(wrapper.find('iframe[title="admin.accounts.pelicanPreview"]').exists()).toBe(false)
  })

  it('已标记的自定义 Gemini 模型默认使用生图探测', async () => {
    getAvailableModels.mockResolvedValue([{ id: 'nano-banana-2', display_name: 'nano-banana-2' }])
    const wrapper = mountModal({
      id: 55,
      name: 'Custom Gemini Image',
      platform: 'gemini',
      type: 'apikey',
      status: 'active',
      extra: { image_generation_models: ['nano-banana-2'] }
    })
    await wrapper.setProps({ show: true })
    await flushPromises()

    const promptInput = wrapper.find('textarea.textarea-stub')
    expect(promptInput.exists()).toBe(true)
    await promptInput.setValue('draw a red apple')
    const startButton = wrapper.findAll('button').find(button => button.text().includes('admin.accounts.startTest'))
    await startButton!.trigger('click')
    await flushPromises()

    const [, request] = (global.fetch as any).mock.calls[0]
    expect(JSON.parse(request.body)).toEqual({
      model_id: 'nano-banana-2',
      prompt: 'draw a red apple',
      mode: 'image'
    })
  })

  it('未知 Gemini 模型可以手动切换生图和文本', async () => {
    getAvailableModels.mockResolvedValue([{ id: 'nano-banana-2', display_name: 'nano-banana-2' }])
    const wrapper = mountModal()
    await wrapper.setProps({ show: true })
    await flushPromises()
    expect(wrapper.find('textarea.textarea-stub').exists()).toBe(false)
    const mode = wrapper.findAll('select.select-stub')[1]
    await mode.setValue('image')
    expect(wrapper.find('textarea.textarea-stub').exists()).toBe(true)
    await wrapper.find('textarea.textarea-stub').setValue('draw a pear')
    const startButton = wrapper.findAll('button').find(button => button.text().includes('admin.accounts.startTest'))!
    await startButton.trigger('click')
    await flushPromises()
    expect(JSON.parse((global.fetch as any).mock.calls[0][1].body)).toMatchObject({ mode: 'image', prompt: 'draw a pear' })
    await mode.setValue('text')
    expect(wrapper.find('textarea.textarea-stub').exists()).toBe(false)
  })

  it('OpenAI APIKEY 账号的 Gemini 图片模型会使用生图提示词并渲染预览', async () => {
    getAvailableModels.mockResolvedValue([
      { id: 'gemini-2.5-flash-image', display_name: 'Gemini 2.5 Flash Image' }
    ])

    const wrapper = mountModal({
      id: 54,
      name: 'OpenAI Compatible Gemini Image',
      platform: 'openai',
      type: 'apikey',
      status: 'active'
    })
    await wrapper.setProps({ show: true })
    await flushPromises()

    const promptInput = wrapper.find('textarea.textarea-stub')
    expect(promptInput.exists()).toBe(true)
    expect((promptInput.element as HTMLTextAreaElement).value).toBe(
      'Generate a cute orange cat astronaut sticker on a clean pastel background.'
    )

    const buttons = wrapper.findAll('button')
    const startButton = buttons.find((button) => button.text().includes('admin.accounts.startTest'))
    expect(startButton).toBeTruthy()

    await startButton!.trigger('click')
    await flushPromises()
    await flushPromises()

    const [, request] = (global.fetch as any).mock.calls[0]
    expect(JSON.parse(request.body)).toEqual({
      model_id: 'gemini-2.5-flash-image',
      prompt: 'Generate a cute orange cat astronaut sticker on a clean pastel background.',
      mode: 'default'
    })

    const preview = wrapper.find('img[alt="test-image-1"]')
    expect(preview.exists()).toBe(true)
    expect(preview.attributes('src')).toBe('data:image/png;base64,QUJD')
  })

  it('grok 账号测试默认选择 Grok 模型', async () => {
    getAvailableModels.mockResolvedValue([
      { id: 'grok-4.3', display_name: 'Grok 4.3' },
      { id: 'grok-build-0.1', display_name: 'Grok Build 0.1' }
    ])
    global.fetch = vi.fn().mockResolvedValue(
      createStreamResponse([
        'data: {"type":"test_start","model":"grok-4.3"}\n',
        'data: {"type":"content","text":"ok"}\n',
        'data: {"type":"test_complete","success":true}\n'
      ])
    ) as any

    const wrapper = mountModal({
      id: 13,
      name: 'Grok Account',
      platform: 'grok',
      type: 'oauth',
      status: 'active'
    })
    await wrapper.setProps({ show: true })
    await flushPromises()

    const buttons = wrapper.findAll('button')
    const startButton = buttons.find((button) => button.text().includes('admin.accounts.startTest'))
    expect(startButton).toBeTruthy()

    await startButton!.trigger('click')
    await flushPromises()

    expect(global.fetch).toHaveBeenCalledTimes(1)
    const [, request] = (global.fetch as any).mock.calls[0]
    expect(JSON.parse(request.body)).toEqual({
      model_id: 'grok-4.3',
      prompt: '',
      mode: 'text'
    })
  })

  it('OpenAI Compact 探测会携带 compact 测试模式', async () => {
    getAvailableModels.mockResolvedValue([
      { id: 'gpt-5.4', display_name: 'GPT-5.4' }
    ])
    global.fetch = vi.fn().mockResolvedValue(
      createStreamResponse([
        'data: {"type":"test_complete","success":true}\n'
      ])
    ) as any

    const wrapper = mountModal({
      id: 42,
      name: 'OpenAI OAuth',
      platform: 'openai',
      type: 'oauth',
      status: 'active'
    })
    await wrapper.setProps({ show: true })
    await flushPromises()

    ;(wrapper.vm as any).selectedModelId = 'gpt-5.4'
    ;(wrapper.vm as any).testMode = 'compact'
    await (wrapper.vm as any).startTest()
    await flushPromises()

    expect(global.fetch).toHaveBeenCalledTimes(1)
    const [, request] = (global.fetch as any).mock.calls[0]
    expect(JSON.parse(request.body)).toMatchObject({
      model_id: 'gpt-5.4',
      prompt: '',
      mode: 'compact'
    })
  })
})
