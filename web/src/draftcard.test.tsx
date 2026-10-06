// @vitest-environment jsdom
import { render } from 'preact'
import { act } from 'preact/test-utils'
import { describe, expect, it, vi } from 'vitest'
import { DraftCard } from './diffview'
import { WriteContext, type Writer } from './write'
import type { Draft } from './types'

const draft = { id: 'd1', severity: 'high', title: 'A finding', description: 'It is wrong.', status: 'pending', revision: 1, session: 's1', drafted_by: 'Fake', updated: '' } as unknown as Draft

describe('DraftCard', () => {
  it('keeps once however often Keep is pressed, and says it is keeping', async () => {
    let finish!: () => void
    const act_ = vi.fn(() => new Promise<void>((resolve) => { finish = resolve }))
    const w: Writer = { url: 'u', caps: { drafts: true, reply: true, resolve: true, approve: true, unapprove: true, request_changes: true, suggestions: true }, act: act_ }
    const root = document.createElement('div')
    act(() => render(<WriteContext.Provider value={w}><DraftCard draft={draft} runID="r1" /></WriteContext.Provider>, root))
    const keep = () => [...root.querySelectorAll('button')].find((b) => /Keep/.test(b.textContent!) && !/Edit/.test(b.textContent!))!
    act(() => keep().click())
    act(() => keep().click())
    expect(act_).toHaveBeenCalledTimes(1)
    expect(keep().textContent).toContain('Keeping')
    expect(keep().disabled).toBe(true)
    expect([...root.querySelectorAll('button')].every((b) => b.disabled || /suggestion/.test(b.textContent!))).toBe(true)
    await act(async () => finish())
    // Until the run shows the decision, the buttons stay off.
    expect(keep().disabled).toBe(true)
  })

  it('lets the person try again after a failed Keep', async () => {
    const w: Writer = { url: 'u', caps: { drafts: true, reply: true, resolve: true, approve: true, unapprove: true, request_changes: true, suggestions: true }, act: vi.fn(() => Promise.reject(new Error('forge down'))) }
    const root = document.createElement('div')
    act(() => render(<WriteContext.Provider value={w}><DraftCard draft={draft} runID="r1" /></WriteContext.Provider>, root))
    const keep = () => [...root.querySelectorAll('button')].find((b) => b.textContent === 'Keep')!
    await act(async () => keep().click())
    expect(keep().disabled).toBe(false)
  })
})
