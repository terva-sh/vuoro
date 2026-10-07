// @vitest-environment jsdom
import { render } from 'preact'
import { act } from 'preact/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { api, post } from './api'
import { RunPanel, RunView } from './run'
import type { Change, Draft, Run, Setup } from './types'

vi.mock('./api', () => ({ api: vi.fn(), post: vi.fn() }))

const change = {
  kind: 'gitlab', ref: { host: 'gitlab.example', project: 'g/p', number: 7 }, title: 'A change', web_url: 'https://gitlab.example/g/p/-/merge_requests/7',
  author: { username: 'someone', bot: false }, diff: { head: 'newhead123', base: 'b', start: 'b' },
} as unknown as Change
const setup = {
  forges: [], tools: [{ name: 'fake', tool: 'Fake Tool', state: 'ok' }], presets: [{ name: 'default', reviewers: [{ profile: 'code' }] }],
} as unknown as Setup
const draft = (id: string, over: Partial<Draft> = {}): Draft =>
  ({ id, severity: 'high', title: `Finding ${id}`, description: 'd', status: 'pending', revision: 1, session: 's1', drafted_by: 'Fake', updated: '', ...over }) as Draft
const run = (over: Partial<Run> = {}): Run => ({
  id: 'r1', kind: 'gitlab', ref: change.ref, title: 't', web_url: change.web_url, head: 'oldhead123', base: 'b',
  created: new Date().toISOString(), status: 'idle', cost_usd: 0, sessions: [], drafts: [draft('d1', { decision: 'dropped' })], ...over,
})
const settle = async () => { for (let i = 0; i < 5; i++) await act(async () => { await Promise.resolve() }) }
const button = (root: Element, text: string) => [...root.querySelectorAll('button')].find((b) => b.textContent === text)
const root = document.createElement('div')
afterEach(() => { act(() => render(null, root)); vi.resetAllMocks() })

describe('reviewing again', () => {
  it('offers a review again of a moved change, following the run shown', async () => {
    const r1 = run()
    vi.mocked(api).mockImplementation(async (path: string) => (path === '/api/runs' ? [r1] : path.endsWith('/head') ? {} : r1) as never)
    vi.mocked(post).mockResolvedValue(run({ id: 'r2', head: 'newhead123', follows: 'r1' }))
    await act(async () => render(<RunPanel change={change} setup={setup} onPick={() => {}} onDrafts={() => {}} />, root))
    await settle()
    await act(async () => button(root, 'Review again')!.click())
    expect(post).toHaveBeenCalledWith('/api/runs', { url: change.web_url, preset: 'default', follows: 'r1' })
  })

  it('offers no review again while the run is at the change\'s head', async () => {
    const r1 = run({ head: 'newhead123' })
    vi.mocked(api).mockImplementation(async (path: string) => (path === '/api/runs' ? [r1] : path.endsWith('/head') ? {} : r1) as never)
    await act(async () => render(<RunPanel change={change} setup={setup} onPick={() => {}} onDrafts={() => {}} />, root))
    await settle()
    expect(button(root, 'Start a review')).toBeDefined()
    expect(button(root, 'Review again')).toBeUndefined()
  })

  it('links the runs, folds the earlier findings, and marks a carried decision', () => {
    const select = vi.fn()
    const r = run({
      id: 'r2', follows: 'r1', followed_by: ['r3'],
      drafts: [draft('d1', { earlier: 'd4', decision: 'dropped', carried: true }), draft('d2')],
      earlier: [
        { id: 'd4', severity: 'low', title: 'Name the sum', standing: 'dismissed', decision: 'dropped' },
        { id: 'd5', severity: 'high', title: 'Add subtracts', standing: 'fixed', fixed_in: 'f55c45591234' },
      ],
    })
    act(() => render(<RunView run={r} url="u" onClose={() => {}} onPick={() => {}} onRun={() => {}} onSelect={select} />, root))
    expect(root.querySelector('.run-follows')!.textContent).toBe('↺ This review follows an earlier review, and was told its findings. ↻ This change was reviewed again since.')
    act(() => button(root, 'an earlier review')!.click())
    expect(select).toHaveBeenCalledWith('r1')
    act(() => button(root, 'reviewed again')!.click())
    expect(select).toHaveBeenCalledWith('r3')

    const group = root.querySelector('details.earlier-findings')!
    expect(group.querySelector('summary')!.textContent).toBe('Earlier findings · 2')
    const rows = [...group.querySelectorAll('li')].map((li) => li.textContent)
    expect(rows[0]).toContain('Name the sum')
    expect(rows[0]).toContain('✗ dismissed')
    expect(rows[0]).toContain('restated as d1')
    expect(rows[1]).toContain('✓ fixed in f55c4559')
    expect(rows[1]).toContain('not raised again')

    const restated = root.querySelector('.finding')!.querySelector('summary')!.textContent!
    expect(restated).toContain('✗ dropped earlier')
    expect(restated).toContain('↺ earlier d4')
    expect(root.querySelector('.finding .draft')!.textContent).toContain('restates earlier finding d4 · ✗ dropped by you in the earlier review')
  })
})
