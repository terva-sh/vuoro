// @vitest-environment jsdom
import { render } from 'preact'
import { act } from 'preact/test-utils'
import { describe, expect, it } from 'vitest'
import { errorText, useAction, type Action } from './action'

describe('errorText', () => {
  it("says an Error's message, and anything else as it is", () => {
    expect(errorText(new Error('refused'))).toBe('refused')
    expect(errorText('offline')).toBe('offline')
  })
})

describe('useAction', () => {
  const mount = () => {
    let a!: Action
    const Probe = () => { a = useAction(); return null }
    const root = document.createElement('div')
    act(() => render(<Probe />, root))
    return { get: () => a, root }
  }

  it('is busy until the request settles, and answers with its result', async () => {
    const p = mount()
    let finish!: (v: number) => void
    let result: Promise<number | undefined>
    act(() => { result = p.get().run(() => new Promise<number>((r) => { finish = r })) })
    expect(p.get().busy).toBe(true)
    await act(async () => { finish(7); await result })
    expect(await result!).toBe(7)
    expect(p.get().busy).toBe(false)
    expect(p.get().error).toBeUndefined()
  })

  it('keeps what the request threw, and clears it on the next run', async () => {
    const p = mount()
    await act(async () => { expect(await p.get().run(() => Promise.reject(new Error('refused')))).toBeUndefined() })
    expect(p.get().error).toBe('refused')
    expect(p.get().busy).toBe(false)
    await act(async () => { await p.get().run(async () => 1) })
    expect(p.get().error).toBeUndefined()
  })

  it('sets nothing once the component is gone', async () => {
    const p = mount()
    let fail!: (e: Error) => void
    let result: Promise<unknown>
    act(() => { result = p.get().run(() => new Promise((_, r) => { fail = r })) })
    act(() => render(null, p.root))
    await act(async () => { fail(new Error('late')); await result })
    expect(p.get().error).toBeUndefined()
  })
})
