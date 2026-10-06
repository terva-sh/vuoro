// @vitest-environment jsdom
import { render } from 'preact'
import { act } from 'preact/test-utils'
import { describe, expect, it } from 'vitest'
import { SubmitBar } from './confirm'
import type { Change } from './types'

const change = { kind: 'gitlab', capabilities: { drafts: true, reply: true, resolve: true, approve: true, unapprove: true, request_changes: true, suggestions: true }, ref: { host: 'gitlab.com', project: 'g/p', number: 7 }, author: { username: 'author' } } as unknown as Change

function bar(waived: string[] = [], approved = false) {
  const posted: [string, Record<string, unknown>][] = []
  const waivedNow: string[][] = []
  const root = document.createElement('div')
  document.body.append(root)
  act(() => render(
    <SubmitBar change={change} me="me" pending={2} approved={approved} waived={waived}
      act={(p, b) => posted.push([p, b])} onWaive={(a) => waivedNow.push(a)} />, root))
  const click = (label: string) => act(() => [...root.querySelectorAll('button')].find((b) => b.textContent === label)!.click())
  return { root, posted, waivedNow, click }
}

describe('SubmitBar', () => {
  for (const [label, path, body] of [
    ['Submit', '/api/changes/submit', {}],
    ['Submit and approve', '/api/changes/submit', { verdict: 'approved' }],
    ['Request changes', '/api/changes/submit', { verdict: 'changes_requested' }],
    ['Approve', '/api/changes/approve', { approve: true }],
  ] as const) {
    it(`asks before ${label}, and Cancel posts nothing`, () => {
      const b = bar()
      b.click(label)
      expect(b.posted).toEqual([])
      expect(b.root.querySelector('[role=dialog]')?.textContent).toContain('!7')
      expect(document.activeElement?.textContent).toBe('Cancel')
      b.click('Cancel')
      expect(b.root.querySelector('[role=dialog]')).toBeNull()
      expect(b.posted).toEqual([])
      b.click(label)
      act(() => (b.root.querySelector('.modal .primary') as HTMLButtonElement).click())
      expect(b.posted).toEqual([[path, body]])
    })
  }

  it('asks before revoking an approval', () => {
    const b = bar([], true)
    b.click('Revoke approval')
    expect(b.posted).toEqual([])
    expect(b.root.querySelector('[role=dialog]')).not.toBeNull()
  })

  it('Escape cancels', () => {
    const b = bar()
    b.click('Approve')
    act(() => { window.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' })) })
    expect(b.root.querySelector('[role=dialog]')).toBeNull()
    expect(b.posted).toEqual([])
  })

  it('runs a waived action at once, and asks while any of its actions is not waived', () => {
    const b = bar(['approve'])
    b.click('Approve')
    expect(b.posted).toEqual([['/api/changes/approve', { approve: true }]])
    b.click('Submit and approve')
    expect(b.root.querySelector('[role=dialog]')).not.toBeNull()
    expect(b.posted.length).toBe(1)
  })

  it('waives what the person ticked', () => {
    const b = bar()
    b.click('Submit')
    act(() => (b.root.querySelector('.modal input[type=checkbox]') as HTMLInputElement).click())
    act(() => (b.root.querySelector('.modal .primary') as HTMLButtonElement).click())
    expect(b.waivedNow).toEqual([['submit']])
    expect(b.posted).toEqual([['/api/changes/submit', {}]])
  })
})
