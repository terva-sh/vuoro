// @vitest-environment jsdom
import { render } from 'preact'
import { act } from 'preact/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { usePoll } from './poll'

const settle = () => act(async () => { for (let i = 0; i < 5; i++) await Promise.resolve() })

function mount(tick: (signal: AbortSignal) => Promise<number | undefined>, dep = 'a') {
  const Probe = ({ d }: { d: string }) => { usePoll(tick, [d]); return null }
  const root = document.createElement('div')
  act(() => render(<Probe d={dep} />, root))
  return { root, rerender: (d: string) => act(() => render(<Probe d={d} />, root)), unmount: () => act(() => render(null, root)) }
}

afterEach(() => vi.useRealTimers())

describe('usePoll', () => {
  it('ticks at once, then again after the wait each tick answers', async () => {
    vi.useFakeTimers()
    const waits = [100, 300]
    const tick = vi.fn(async () => waits.shift() ?? 1000)
    mount(tick)
    await settle()
    expect(tick).toHaveBeenCalledTimes(1)
    await act(async () => { vi.advanceTimersByTime(99) })
    expect(tick).toHaveBeenCalledTimes(1)
    await act(async () => { vi.advanceTimersByTime(1) })
    await settle()
    expect(tick).toHaveBeenCalledTimes(2)
    await act(async () => { vi.advanceTimersByTime(299) })
    expect(tick).toHaveBeenCalledTimes(2)
    await act(async () => { vi.advanceTimersByTime(1) })
    expect(tick).toHaveBeenCalledTimes(3)
  })

  it('never overlaps a tick still running', async () => {
    vi.useFakeTimers()
    let finish!: () => void
    const tick = vi.fn(() => new Promise<number>((r) => { finish = () => r(10) }))
    mount(tick)
    await act(async () => { vi.advanceTimersByTime(1000) })
    expect(tick).toHaveBeenCalledTimes(1)
    finish()
    await settle()
    await act(async () => { vi.advanceTimersByTime(10) })
    expect(tick).toHaveBeenCalledTimes(2)
  })

  it('stops on an undefined answer or a throw', async () => {
    vi.useFakeTimers()
    for (const tick of [vi.fn(async () => undefined), vi.fn(async (): Promise<number> => { throw new Error('gone') })]) {
      mount(tick)
      await settle()
      await act(async () => { vi.advanceTimersByTime(60_000) })
      expect(tick).toHaveBeenCalledTimes(1)
    }
  })

  it('aborts and stops on unmount, and starts again when its deps change', async () => {
    vi.useFakeTimers()
    const signals: AbortSignal[] = []
    const tick = vi.fn(async (s: AbortSignal) => { signals.push(s); return 100 })
    const p = mount(tick)
    await settle()
    p.rerender('b')
    await settle()
    expect(tick).toHaveBeenCalledTimes(2)
    expect(signals[0].aborted).toBe(true)
    p.unmount()
    expect(signals[1].aborted).toBe(true)
    await act(async () => { vi.advanceTimersByTime(10_000) })
    expect(tick).toHaveBeenCalledTimes(2)
  })
})
