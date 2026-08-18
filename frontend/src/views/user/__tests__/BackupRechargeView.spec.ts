import { beforeEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { defineComponent, h } from 'vue'
import BackupRechargeView from '../BackupRechargeView.vue'

const routeState = vi.hoisted(() => ({ params: { channelId: 'backup-1' } as Record<string, string> }))
const routerPush = vi.hoisted(() => vi.fn())
const routerReplace = vi.hoisted(() => vi.fn())
const fetchPublicSettings = vi.hoisted(() => vi.fn())
const appState = vi.hoisted(() => ({
  publicSettingsLoaded: true,
  cachedPublicSettings: {
    recharge_storefront_enabled: true,
    recharge_storefront_channels: [
      { id: 'backup-1', name: '备用一', url: 'https://shop.example.com/', enabled: true, sort_order: 1 },
      { id: 'backup-2', name: '备用二', url: 'https://pay.example.com/shop/2', enabled: true, sort_order: 2 },
    ],
  },
}))

vi.mock('vue-router', () => ({
  useRoute: () => routeState,
  useRouter: () => ({ push: routerPush, replace: routerReplace }),
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key }),
  }
})

vi.mock('@/stores', () => ({
  useAppStore: () => ({ ...appState, fetchPublicSettings }),
}))

const AppLayoutStub = defineComponent({
  setup(_, { slots }) {
    return () => h('main', slots.default?.())
  },
})

const IconStub = defineComponent({
  props: { name: String },
  setup(props) {
    return () => h('span', { 'data-icon': props.name })
  },
})

function mountView() {
  return mount(BackupRechargeView, {
    global: {
      stubs: {
        AppLayout: AppLayoutStub,
        Icon: IconStub,
      },
    },
  })
}

describe('BackupRechargeView', () => {
  beforeEach(() => {
    routeState.params.channelId = 'backup-1'
    appState.publicSettingsLoaded = true
    appState.cachedPublicSettings.recharge_storefront_enabled = true
    routerPush.mockReset()
    routerReplace.mockReset()
    fetchPublicSettings.mockReset()
  })

  it('keeps enabled channel iframes mounted and uses configured URLs unchanged', () => {
    const wrapper = mountView()
    const frames = wrapper.findAll('iframe')

    expect(frames).toHaveLength(2)
    expect(frames.map((frame) => frame.attributes('src'))).toEqual([
      'https://shop.example.com/',
      'https://pay.example.com/shop/2',
    ])
    expect(frames.every((frame) => !frame.attributes('src')?.includes('token'))).toBe(true)
    expect(frames[1].element.parentElement?.style.display).toBe('none')
  })

  it('navigates through the named route when another channel is selected', async () => {
    const wrapper = mountView()
    await wrapper.findAll('.backup-recharge-tab')[1].trigger('click')

    expect(routerPush).toHaveBeenCalledWith({ name: 'BackupRecharge', params: { channelId: 'backup-2' } })
  })

  it('shows the unavailable state when the global switch is off', () => {
    appState.cachedPublicSettings.recharge_storefront_enabled = false
    const wrapper = mountView()

    expect(wrapper.findAll('iframe')).toHaveLength(0)
    expect(wrapper.text()).toContain('backupRecharge.unavailable')
  })
})
