// @vitest-environment jsdom
import { render } from 'preact'
import { act } from 'preact/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { api, post } from './api'
import { Editor, PushDialog, RunView } from './run'
import type { Draft, Run, Session } from './types'

const draft = (id: string, decision?: 'kept' | 'dropped') =>
  ({ id, severity: 'high', title: `Finding ${id}`, description: 'd', status: 'pending', revision: 1, session: 's1', updated: '', decision }) as unknown as Draft

const run = (over: Partial<Run> = {}): Run => ({
  id: 'r1', kind: 'gitlab', ref: { host: 'h', project: 'p', number: 7 }, title: 't', web_url: 'u', head: 'abcdef1234', base: 'b',
  created: '', status: 'idle', cost_usd: 0, sessions: [], drafts: [draft('d1', 'kept'), draft('d2', 'kept'), draft('d3')], ...over,
}) as Run

vi.mock('./api', () => ({ api: vi.fn(), post: vi.fn() }))

afterEach(() => vi.resetAllMocks())

describe('Editor', () => {
  it('asks for a kept finding before it starts', () => {
    const root = document.createElement('div')
    act(() => render(<Editor run={run({ drafts: [draft('d1')] })} url="u" onRun={() => {}} />, root))
    const b = root.querySelector('button')!
    expect(b.disabled).toBe(true)
    expect(b.title).toBe('Keep a finding first')
  })

  it('offers a closed review to the editor', () => {
    const root = document.createElement('div')
    act(() => render(<Editor run={run({ status: 'closed' })} url="u" onRun={() => {}} />, root))
    expect(root.querySelector('button')!.disabled).toBe(false)
  })

  it('shows the commits beside their findings and names the ones left alone', async () => {
    vi.mocked(api).mockResolvedValue({
      session: 's2', branch: 'vuoro/fix-7-abc123', base: 'abcdef1234', started: '',
      commits: [{ sha: '0123456789', subject: 'Add adds', findings: ['d1'] }],
      files: [{ old_path: 'pkg/calc.go', new_path: 'pkg/calc.go', patch: '@@ -1 +1 @@\n-a\n+b\n' }],
    })
    const session = { id: 's2', profile: 'editor', tool: 'claude', role: 'editor', status: 'idle', usage: { cost_usd: 0 }, log: [] } as unknown as Session
    const r = run({ editor: { session: 's2', branch: 'vuoro/fix-7-abc123', base: 'abcdef1234', started: '' }, sessions: [session] })
    const root = document.createElement('div')
    await act(async () => render(<Editor run={r} url="u" onRun={() => {}} />, root))
    await act(async () => {})
    expect(root.textContent).toContain('Add adds')
    expect(root.textContent).toContain('answers d1 Finding d1')
    expect(root.textContent).toContain('No commit names d2 Finding d2')
    expect(root.textContent).not.toContain('d3')
    // The editor's files are listed beside its diff, as the change's are.
    expect(root.querySelector('.file-list .file-link')?.textContent).toBe('calc.gopkg')
    expect(root.querySelector('#editor-file-pkg_calc_go')).not.toBeNull()
    act(() => [...root.querySelectorAll('button')].find((b) => b.textContent === 'Discard…')!.click())
    expect(root.textContent).toContain('Delete the branch')
  })

  it('pushes only from the dialog, with what the person saw and edited', async () => {
    const preview = { branch: 'vuoro/fix-7-abc', target: 'fix/total', sha: 'f00d', commits: 2, files: ['.gitlab-ci.yml', 'calc.go'], ci: ['.gitlab-ci.yml'], title: 'Fix review findings', description: 'd' }
    vi.mocked(post).mockResolvedValue(run())
    const closed = vi.fn()
    const root = document.createElement('div')
    document.body.append(root)
    act(() => render(<PushDialog run={run()} url="u" preview={preview} onClose={closed} onRun={() => {}} />, root))
    expect(root.textContent).toContain('Changes CI configuration')
    expect(document.activeElement?.textContent).toBe('Cancel')
    expect(post).not.toHaveBeenCalled()
    const title = root.querySelector('input')!
    act(() => { title.value = 'My title'; title.dispatchEvent(new Event('input')) })
    await act(async () => { [...root.querySelectorAll('button')].find((b) => b.textContent === 'Push and open')!.click() })
    expect(post).toHaveBeenCalledWith('/api/runs/r1/editor/push', { url: 'u', confirm: 'push', sha: 'f00d', title: 'My title', description: 'd' })
    expect(closed).toHaveBeenCalled()
    root.remove()
  })

  it('hands the picked threads of the person\'s own change to the editor', async () => {
    vi.mocked(post).mockResolvedValue(run())
    const thread = (id: string) => ({ ...draft(id), thread_id: `t-${id}`, category: 'must_fix' }) as Draft
    const own = run({ mode: 'triage', drafts: [thread('d1'), thread('d2')] })
    const root = document.createElement('div')
    act(() => render(<Editor run={own} url="u" onRun={() => {}} picked={[thread('d2')]} describe={() => 'x'} />, root))
    const b = root.querySelector('button')!
    expect(b.textContent).toContain('Fix the picked threads in a branch (1)')
    await act(async () => b.click())
    expect(post).toHaveBeenCalledWith('/api/runs/r1/editor', { url: 'u', items: ['d2'] })
  })

  it('on the person\'s own review, picks findings for the editor and posts nothing', async () => {
    vi.mocked(api).mockResolvedValue({})
    vi.mocked(post).mockResolvedValue(run())
    const low = { ...draft('d3'), severity: 'low' } as Draft
    const own = run({ drafts: [draft('d1'), draft('d2'), low] })
    const root = document.createElement('div')
    act(() => render(<RunView run={own} url="u" own onClose={() => {}} onPick={() => {}} onRun={() => {}} />, root))
    expect(root.textContent).not.toContain('Keep')
    expect(root.textContent).not.toContain('Copy the hand-off report')
    const boxes = [...root.querySelectorAll<HTMLInputElement>('.finding-row > .pick input')]
    expect(boxes.map((b) => b.checked)).toEqual([true, true, false]) // low is not picked by default
    act(() => { boxes[1].checked = false; boxes[1].dispatchEvent(new Event('change', { bubbles: true })) })
    const start = [...root.querySelectorAll('button')].find((b) => b.textContent!.includes('Fix the picked findings'))!
    expect(start.textContent).toContain('(1)')
    await act(async () => start.click())
    expect(post).toHaveBeenCalledWith('/api/runs/r1/editor', { url: 'u', items: ['d1'] })
  })
})
