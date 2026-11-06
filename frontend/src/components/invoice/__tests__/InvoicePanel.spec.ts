import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { defineComponent, h } from 'vue'
import type { InvoiceAPI } from '@/api/invoice'
import type { AdminInvoiceAPI } from '@/api/admin/invoice'

const showError = vi.hoisted(() => vi.fn())

// 用 dev 预览用的同一份内存假数据驱动整条流程，接上后端后换成真实接口即可
vi.mock('@/api/invoice', async () => {
  const { invoiceMockAPI, adminInvoiceMockAPI } = await import('@/api/invoiceMock')
  return { invoiceAPI: invoiceMockAPI, adminInvoiceAPI: adminInvoiceMockAPI }
})
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showError }) }))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => ({ user: { email: 'me@example.com' } }) }))

const IconStub = defineComponent({
  props: { name: String },
  setup: (props) => () => h('i', { 'data-icon': props.name }),
})
const BaseDialogStub = defineComponent({
  props: { show: Boolean, title: String },
  setup: (props, { slots }) => () =>
    props.show ? h('div', { 'data-testid': 'dialog' }, [h('h2', props.title), slots.default?.(), slots.footer?.()]) : null,
})

// Select 用原生 <select> 替身，测试里直接选值，不依赖 Teleport 下拉
const SelectStub = defineComponent({
  props: { modelValue: null, options: { type: Array, default: () => [] } },
  emits: ['update:modelValue'],
  setup: (props, { emit }) => () =>
    h('select', { onChange: (e: Event) => emit('update:modelValue', Number((e.target as HTMLSelectElement).value)) }, [
      h('option', { value: '' }, ''),
      ...(props.options as { value: number; label: string }[]).map((o) => h('option', { value: o.value }, o.label)),
    ]),
})

async function mountPanel() {
  // resetModules 之后组件拿到的是新的 vue-i18n 实例，插件也要从同一份模块创建。
  // vitest 用的是 runtime 构建，需与 vite.config.ts 一样打开 JIT，否则文案不编译、只显示键名
  vi.stubGlobal('__INTLIFY_JIT_COMPILATION__', true)
  const { createI18n } = await import('vue-i18n')
  const { default: zh } = await import('@/i18n/locales/zh')
  const { default: InvoicePanel } = await import('../InvoicePanel.vue')
  const i18n = createI18n({ legacy: false, locale: 'zh', messages: { zh }, missingWarn: false, fallbackWarn: false })
  return mount(InvoicePanel, { global: { plugins: [i18n], stubs: { Icon: IconStub, BaseDialog: BaseDialogStub, Select: SelectStub, transition: true } } })
}

async function settle() {
  await vi.runAllTimersAsync()
  await flushPromises()
}

/** 假数据的 delay 走 setTimeout，假定时器下要手动推进才会结束 */
async function withTimers<T>(promise: Promise<T>): Promise<T> {
  await vi.runAllTimersAsync()
  return promise
}

const nextButton = (w: Awaited<ReturnType<typeof mountPanel>>) => w.get('[data-testid="invoice-next"]')

describe('InvoicePanel', () => {
  beforeEach(() => {
    vi.resetModules()
    vi.useFakeTimers({ toFake: ['setTimeout', 'clearTimeout', 'Date'] })
    showError.mockReset()
  })
  afterEach(() => {
    vi.useRealTimers()
  })

  it('shows only open orders as selectable and keeps claimed ones disabled', async () => {
    const wrapper = await mountPanel()
    await settle()

    // 假数据里 #1005 占用了 319921、319802，#1002 占用了 319755
    const claimed = wrapper.get('[data-testid="invoice-order-319921"] input')
    expect(claimed.attributes('disabled')).toBeDefined()
    expect(wrapper.get('[data-testid="invoice-order-319921"]').text()).toContain('已申请 #1005')
    expect(wrapper.get('[data-testid="invoice-order-320118"] input').attributes('disabled')).toBeUndefined()
    expect(nextButton(wrapper).attributes('disabled')).toBeDefined()
  })

  it('shows orders the admin excluded as invoiced, without a request link, and refuses to submit them', async () => {
    // 取和面板同一份内存数据，直接 import invoiceMock 在 resetModules 后会拿到另一个实例
    const mocked = (await import('@/api/invoice')) as unknown as { invoiceAPI: InvoiceAPI; adminInvoiceAPI: AdminInvoiceAPI }
    const { invoiceAPI, adminInvoiceAPI } = mocked
    const before = await withTimers(invoiceAPI.getInvoiceableOrders())
    const target = before.items.find((o) => !o.already_invoiced)!
    const added = await withTimers(adminInvoiceAPI.addExclusions([target.out_trade_no]))
    expect(added.added).toHaveLength(1)

    const wrapper = await mountPanel()
    await settle()

    const row = wrapper.get(`[data-testid="invoice-order-${target.id}"]`)
    expect(row.text()).toContain('已开票')
    expect(row.text()).toContain(target.out_trade_no)
    expect(row.get('input').attributes('disabled')).toBeDefined()

    const after = await withTimers(invoiceAPI.getInvoiceableOrders())
    expect(after.summary.invoiceable_count).toBe(before.summary.invoiceable_count - 1)
    const refused = invoiceAPI.createRequest({ invoice_type: 'individual', order_ids: [target.id], individual_name: '张三', receiver_email: 'a@b.cn' })
    const assertion = expect(refused).rejects.toMatchObject({ reason: 'ORDER_NOT_INVOICEABLE' })
    await vi.runAllTimersAsync()
    await assertion

    await withTimers(adminInvoiceAPI.removeExclusion(added.added[0].id))
  })

  it('sums only the paid amount and leaves bonus credit out of the invoice total', async () => {
    const wrapper = await mountPanel()
    await settle()

    await wrapper.get('[data-testid="invoice-order-320118"] input').trigger('change')
    await wrapper.get('[data-testid="invoice-order-320097"] input').trigger('change')
    await settle()

    const text = wrapper.text()
    expect(text).toContain('¥700.00')
    expect(text).toContain('赠送额度（不开票）')
    expect(text).toContain('$35.00')
    expect(nextButton(wrapper).attributes('disabled')).toBeUndefined()
  })

  it('select-all picks every open order but none of the claimed ones', async () => {
    const wrapper = await mountPanel()
    await settle()

    await wrapper.get('input[type="checkbox"]:not(.sr-only)').trigger('change')
    await settle()
    // 8 笔订单里 3 笔被占用，剩 5 笔：500 + 200 + 99 + 1000 + 2000
    expect(wrapper.text()).toContain('5 笔')
    expect(wrapper.text()).toContain('¥3,799.00')
    expect(wrapper.get('[data-testid="invoice-order-319755"]').classes()).not.toContain('inv-order-selected')
  })

  it('blocks the confirm step until the title form is valid', async () => {
    const wrapper = await mountPanel()
    await settle()
    await wrapper.get('[data-testid="invoice-order-320118"] input').trigger('change')
    await nextButton(wrapper).trigger('click')

    // 企业抬头为空：点下一步只会把错误全部亮出来
    await nextButton(wrapper).trigger('click')
    expect(wrapper.text()).toContain('请输入企业名称')
    expect(wrapper.text()).toContain('请输入纳税人识别号')
    expect(wrapper.text()).toContain('请选择开票类目')
    expect(wrapper.find('[data-testid="invoice-agree"]').exists()).toBe(false)

    await wrapper.get('#inv-enterprise-name').setValue('杭州云栈科技有限公司')
    await wrapper.get('#inv-tax').setValue('91330106ma2kxxxx7q')
    // 抬头填完但没选类目：仍然进不了确认页
    await nextButton(wrapper).trigger('click')
    expect(wrapper.find('[data-testid="invoice-agree"]').exists()).toBe(false)

    await wrapper.get('[data-testid="invoice-category-select"]').setValue('1')
    await nextButton(wrapper).trigger('click')
    expect(wrapper.find('[data-testid="invoice-agree"]').exists()).toBe(true)
    expect(wrapper.text()).toContain('技术服务费')
    // 确认页展示大写后的税号
    expect(wrapper.text()).toContain('91330106MA2KXXXX7Q')
  })

  it('submits after the user agrees, then shows success and frees nothing it claimed', async () => {
    const wrapper = await mountPanel()
    await settle()
    await wrapper.get('[data-testid="invoice-order-320118"] input').trigger('change')
    await nextButton(wrapper).trigger('click')
    await wrapper.get('[data-testid="invoice-type-individual"]').trigger('click')
    await wrapper.get('#inv-name').setValue('张三')
    await wrapper.get('[data-testid="invoice-category-select"]').setValue('2')
    await nextButton(wrapper).trigger('click')
    expect(wrapper.text()).toContain('软件使用费')

    expect(nextButton(wrapper).attributes('disabled')).toBeDefined()
    await wrapper.get('[data-testid="invoice-agree"]').setValue(true)
    await nextButton(wrapper).trigger('click')
    await settle()

    const success = wrapper.get('[data-testid="invoice-success"]')
    expect(success.text()).toContain('申请编号 #1009')
    expect(success.text()).toContain('me@example.com')
    expect(success.text()).toContain('¥500.00')
    expect(showError).not.toHaveBeenCalled()

    // 刚提交的订单在列表里变成已申请
    await wrapper.get('[data-testid="invoice-view-apply"]').trigger('click')
    await settle()
    expect(wrapper.get('[data-testid="invoice-order-320118"] input').attributes('disabled')).toBeDefined()
  })

  it('reapplies a rejected request with its title prefilled', async () => {
    const wrapper = await mountPanel()
    await settle()
    await wrapper.get('[data-testid="invoice-view-records"]').trigger('click')
    await settle()

    const rejected = wrapper.findAll('li button').find((b) => b.text().includes('#1001'))
    expect(rejected).toBeDefined()
    await rejected!.trigger('click')
    await settle()

    const dialog = wrapper.get('[data-testid="dialog"]')
    expect(dialog.text()).toContain('抬头与税号不一致')
    await dialog.findAll('button').find((b) => b.text() === '修改后重新申请')!.trigger('click')
    await settle()

    expect(wrapper.find('[data-testid="dialog"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="invoice-view-apply"]').attributes('aria-selected')).toBe('true')
    // resetModules 不会重建 vi.mock 的模块，假数据跨用例保留：上一个用例已占用 320118
    await wrapper.get('[data-testid="invoice-order-320097"] input').trigger('change')
    await nextButton(wrapper).trigger('click')
    expect((wrapper.get('#inv-name').element as HTMLInputElement).value).toBe('张三')
  })

  it('refuses a request without a usable category and stores the name snapshot on success', async () => {
    const mocked = (await import('@/api/invoice')) as unknown as { invoiceAPI: InvoiceAPI }
    const { invoiceAPI } = mocked
    const categories = await withTimers(invoiceAPI.getCategories())
    expect(categories.map((c) => c.name)).toEqual(['技术服务费', '软件使用费'])

    const open = (await withTimers(invoiceAPI.getInvoiceableOrders())).items.find((o) => !o.already_invoiced)!
    const base = { invoice_type: 'individual' as const, order_ids: [open.id], individual_name: '张三', receiver_email: 'a@b.cn' }
    for (const [extra, reason] of [[{}, 'INVOICE_CATEGORY_REQUIRED'], [{ category_id: 99 }, 'INVALID_INVOICE_CATEGORY']] as const) {
      const refused = invoiceAPI.createRequest({ ...base, ...extra })
      const assertion = expect(refused).rejects.toMatchObject({ reason })
      await vi.runAllTimersAsync()
      await assertion
    }

    const created = await withTimers(invoiceAPI.createRequest({ ...base, category_id: 1 }))
    expect(created.category_id).toBe(1)
    expect(created.category_name).toBe('技术服务费')
  })
})
