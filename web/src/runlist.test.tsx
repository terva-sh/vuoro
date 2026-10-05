// @vitest-environment jsdom
import { render } from 'preact'
import { act } from 'preact/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { RunList } from './app'
import type { Run } from './types'

const base: Run = {
  id: 'r1', kind: 'gitlab', ref: { host: 'gitlab.com', project: 'team/repo', number: 7 }, title: 'Forge change',
  web_url: 'https://gitlab.com/team/repo/-/merge_requests/7', head: 'abcdef0123', base: '0123abcdef',
  created: '2026-10-05T00:00:00Z', status: 'idle', cost_usd: 0.1, sessions: [], drafts: [],
}
const local: Run = { ...base, id: 'r2', kind: 'local', ref: { host: 'local', project: 'vuoro-1a2b3c4d', number: 0 }, title: 'Local commits', web_url: '' }
const root = document.createElement('div')
afterEach(() => act(() => render(null, root)))

describe('the run list', () => {
  it('opens a forge run, and lists a local one without a link', () => {
    const open = vi.fn()
    act(() => render(<RunList runs={[base, local]} onOpen={open} />, root))
    const buttons = [...root.querySelectorAll('button')]
    expect(buttons.map((b) => b.textContent)).toEqual(['Forge change'])
    expect(root.textContent).toContain('team/repo!7')
    expect(root.textContent).toContain('Local commits')
    expect(root.textContent).toContain('local commits of vuoro-1a2b3c4d')
    expect(root.textContent).not.toContain('!0')
    act(() => buttons[0].click())
    expect(open).toHaveBeenCalledWith(base.web_url)
  })
  it('shows nothing with no runs', () => {
    act(() => render(<RunList runs={[]} onOpen={() => {}} />, root))
    expect(root.textContent).toBe('')
  })
})
