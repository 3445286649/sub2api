import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { effectScope, nextTick, ref } from 'vue'
import { useCountUp } from '../useCountUp'

/**
 * 自己搭一个可控的 rAF 队列。
 *
 * 不用 vi.useFakeTimers()：它默认不接管 requestAnimationFrame，jsdom 的 rAF
 * 走自己的时钟，advanceTimersByTime 推不动它，动画帧永远不会执行。
 */
let rafCallbacks: Map<number, FrameRequestCallback>
let rafId: number
let rafNow: number

function advance(ms: number) {
  rafNow += ms
  const pending = [...rafCallbacks.values()]
  rafCallbacks.clear()
  for (const cb of pending) cb(rafNow)
}

/** 在独立作用域里跑，结束后能一起清掉动画帧 */
function withScope<T>(fn: () => T) {
  const scope = effectScope()
  const result = scope.run(fn)!
  return { result, stop: () => scope.stop() }
}

function stubMatchMedia(reducedMotion: boolean) {
  vi.stubGlobal('matchMedia', (query: string) => ({
    matches: query.includes('prefers-reduced-motion') ? reducedMotion : false,
    media: query,
    addEventListener: () => {},
    removeEventListener: () => {},
    addListener: () => {},
    removeListener: () => {},
    dispatchEvent: () => false,
    onchange: null,
  }))
}

describe('useCountUp', () => {
  beforeEach(() => {
    rafCallbacks = new Map()
    rafId = 0
    rafNow = 0
    vi.stubGlobal('requestAnimationFrame', (cb: FrameRequestCallback) => {
      rafCallbacks.set(++rafId, cb)
      return rafId
    })
    vi.stubGlobal('cancelAnimationFrame', (id: number) => {
      rafCallbacks.delete(id)
    })
    stubMatchMedia(false)
  })

  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('starts at the current source value without animating', () => {
    const source = ref(100)
    const { result, stop } = withScope(() => useCountUp(source, 400))

    // 成功页依赖这一点：挂载时显示的是充值前的旧余额，等 refreshUser 回来才滚过去
    expect(result.display.value).toBe(100)
    stop()
  })

  it('animates to the new value and lands exactly on it', async () => {
    const source = ref(100)
    const { result, stop } = withScope(() => useCountUp(source, 400))

    source.value = 200
    await nextTick()

    // 首帧只确定起点，还不该跳
    advance(0)
    expect(result.display.value).toBe(100)

    advance(200)
    // 中途应在两端之间，而不是瞬间跳到终点
    expect(result.display.value).toBeGreaterThan(100)
    expect(result.display.value).toBeLessThan(200)

    advance(200)
    expect(result.display.value).toBe(200)
    stop()
  })

  it('jumps straight to the target when reduced motion is requested', async () => {
    stubMatchMedia(true)
    const source = ref(100)
    const { result, stop } = withScope(() => useCountUp(source, 400))

    source.value = 250
    await nextTick()

    // 不做动画，直接是终值
    expect(result.display.value).toBe(250)
    expect(rafCallbacks.size).toBe(0)
    stop()
  })

  it('falls back to the target value when requestAnimationFrame is unavailable', async () => {
    vi.stubGlobal('requestAnimationFrame', undefined)
    const source = ref(10)
    const { result, stop } = withScope(() => useCountUp(source, 400))

    source.value = 30
    await nextTick()

    expect(result.display.value).toBe(30)
    stop()
  })

  it('ignores a non-finite source value instead of rendering NaN', async () => {
    const source = ref(50)
    const { result, stop } = withScope(() => useCountUp(source, 400))

    source.value = Number.NaN
    await nextTick()
    advance(0)
    advance(500)

    expect(Number.isFinite(result.display.value)).toBe(true)
    expect(result.display.value).toBe(0)
    stop()
  })

  it('does not restart the animation when the value is set to what it already targets', async () => {
    const source = ref(100)
    const { result, stop } = withScope(() => useCountUp(source, 400))

    source.value = 200
    await nextTick()
    advance(0)
    advance(400)
    expect(result.display.value).toBe(200)

    // 再次赋同一个值不应触发新的动画
    source.value = 200
    await nextTick()
    expect(rafCallbacks.size).toBe(0)
    expect(result.display.value).toBe(200)
    stop()
  })

  it('restarts from the currently displayed value when the target changes mid-flight', async () => {
    const source = ref(0)
    const { result, stop } = withScope(() => useCountUp(source, 400))

    source.value = 100
    await nextTick()
    advance(0)
    advance(200)
    const midway = result.display.value
    expect(midway).toBeGreaterThan(0)
    expect(midway).toBeLessThan(100)

    // 动画未结束时改目标：应从当前显示值接着滚，而不是回跳
    source.value = 300
    await nextTick()
    advance(0)
    expect(result.display.value).toBeCloseTo(midway, 5)

    advance(400)
    expect(result.display.value).toBe(300)
    stop()
  })
})
