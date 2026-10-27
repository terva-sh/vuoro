// @vitest-environment jsdom
import { render } from 'preact'
import { act } from 'preact/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { api, post } from './api'
import { PushUpdateDialog, UpdatePanel, UpdateView } from './update'
import type { Change, FileDiff, Run, Session, Setup, Update, UpdateDetail } from './types'

vi.mock('./api', () => ({ api: vi.fn(), post: vi.fn() }))

afterEach(() => vi.resetAllMocks())

// jsdom scrolls nothing; a session pane scrolls to its last entry.
Element.prototype.scrollIntoView = () => {}

const update = (over: Partial<Update> = {}): Update => ({
  strategy: 'merge', source: 'feature', target: 'main', target_sha: 'cafe123456', own: true, tool: 'claude', stops: [], started: '', ...over,
})

const run = (u: Update, over: Partial<Run> = {}): Run => ({
  id: 'run-abc123', kind: 'gitlab', ref: { host: 'h', project: 'p', number: 7 }, title: 't', web_url: 'u', head: 'abcdef1234', base: 'b',
  created: '', status: 'idle', cost_usd: 0, sessions: [], drafts: [], mode: 'update', update: u, ...over,
}) as Run

const detail = (u: Update, over: Partial<UpdateDetail> = {}): UpdateDetail => ({ ...u, state: 'ready', resolutions: [], commits: [], ...over })

const button = (root: Element, text: string) => [...root.querySelectorAll('button')].find((b) => b.textContent?.startsWith(text))

const settle = async () => { for (let i = 0; i < 5; i++) await act(async () => { await Promise.resolve() }) }

describe('UpdateView', () => {
  it('shows a stop beside its resolution, and accepts it only when the editor is done', async () => {
    const stop = { subject: "Merge branch 'main' into feature", seed: 'seed1', onto: 'abcdef1234', conflicted: ['calc.go'], session: 's2', branch: 'vuoro/resolve-7-1' }
    const u = update({ stops: [stop] })
    const session = { id: 's2', profile: 'editor', tool: 'claude', role: 'editor', status: 'idle', usage: { cost_usd: 0 }, log: [] } as unknown as Session
    const patch = '@@ -1,5 +1 @@\n-<<<<<<< ours\n-a - b\n-||||||| base\n-a + b\n-=======\n-b + a\n->>>>>>> theirs\n+b - a\n'
    vi.mocked(api).mockResolvedValue(detail(u, { state: 'resolving', resolutions: [[{ old_path: 'calc.go', new_path: 'calc.go', patch } as FileDiff]] }))
    vi.mocked(post).mockResolvedValue(run(u))
    const root = document.createElement('div')
    await act(async () => render(<UpdateView run={run(u, { sessions: [session] })} url="u" onRun={() => {}} />, root))
    await settle()
    expect(api).toHaveBeenCalledWith('/api/runs/run-abc123/update')
    expect(root.textContent).toContain('Conflicts in calc.go')
    expect(root.textContent).toContain('b - a')
    expect(root.textContent).not.toContain('To push')
    await act(async () => button(root, 'Accept this resolution')!.click())
    expect(post).toHaveBeenCalledWith('/api/runs/run-abc123/update/accept')

    // While the editor works, nothing can be accepted.
    const working = { ...session, status: 'working' } as Session
    await act(async () => render(<UpdateView run={run(u, { sessions: [working] })} url="u" onRun={() => {}} />, root))
    await settle()
    expect(button(root, 'Accept this resolution')).toBeUndefined()
    expect(root.textContent).toContain('The editor is resolving the conflict')
  })

  it('lists what a ready update pushes, and asks before it pushes', async () => {
    const u = update({ result: 'f00d123456' })
    vi.mocked(api).mockResolvedValue(detail(u, { commits: [{ sha: 'f00d123456', subject: "Merge branch 'main' into feature" }] }))
    const root = document.createElement('div')
    document.body.appendChild(root)
    await act(async () => render(<UpdateView run={run(u)} url="u" onRun={() => {}} />, root))
    await settle()
    expect(root.textContent).toContain('No conflicts')
    expect(root.textContent).toContain('To push · 1 commit')
    act(() => button(root, '⇪ Push to feature')!.click())
    expect(post).not.toHaveBeenCalled()
    expect(root.textContent).toContain('Fast-forward only')
    root.remove()
  })
})

describe('PushUpdateDialog', () => {
  it('names a rebase, its branch, and its lease, and pushes only on confirmation', async () => {
    const u = update({ strategy: 'rebase', result: 'f00d123456' })
    vi.mocked(post).mockResolvedValue(run(u))
    const root = document.createElement('div')
    document.body.appendChild(root)
    const done = vi.fn()
    act(() => render(<PushUpdateDialog run={run(u)} url="u" sha="f00d123456" commits={2} onClose={() => {}} onRun={done} />, root))
    expect(root.textContent).toContain('Rewrites feature')
    expect(root.textContent).toContain('lease against abcdef1')
    expect(document.activeElement?.textContent).toBe('Cancel')
    expect(post).not.toHaveBeenCalled()
    await act(async () => button(root, 'Push')!.click())
    expect(post).toHaveBeenCalledWith('/api/runs/run-abc123/update/push', expect.objectContaining({ url: 'u', confirm: 'push', sha: 'f00d123456' }))
    expect(done).toHaveBeenCalled()
    root.remove()
  })

  it('sends a merge on somebody else\'s change to a branch of its own, as a draft change', () => {
    const u = update({ own: false, result: 'f00d123456' })
    const root = document.createElement('div')
    document.body.appendChild(root)
    act(() => render(<PushUpdateDialog run={run(u)} url="u" sha="f00d123456" commits={1} onClose={() => {}} onRun={() => {}} />, root))
    expect(root.textContent).toContain('vuoro/update-7-abc123')
    expect(root.textContent).toContain('Opens a draft merge request onto feature')
    expect(root.textContent).toContain('Their branch is not written')
    root.remove()
  })
})

describe('UpdatePanel', () => {
  const change = { kind: 'gitlab', ref: { host: 'h', project: 'p', number: 7 }, web_url: 'u', state: 'open', target_branch: 'main', source_branch: 'feature' } as unknown as Change
  const setup = { tools: [{ name: 'claude', tool: 'Claude Code' }] } as unknown as Setup

  it('merges by default, and rebases only the person\'s own change', async () => {
    vi.mocked(api).mockResolvedValue([])
    const root = document.createElement('div')
    await act(async () => render(<UpdatePanel change={change} setup={setup} own={false} />, root))
    const [merge, rebase] = [...root.querySelectorAll<HTMLInputElement>('input[name=update-strategy]')]
    expect(merge.checked).toBe(true)
    expect(rebase.disabled).toBe(true)
    expect(root.textContent).toContain('goes up as a draft merge request onto feature')
    vi.mocked(post).mockResolvedValue(run(update({ own: false }), { status: 'preparing' }))
    await act(async () => button(root, 'Merge main in')!.click())
    expect(post).toHaveBeenCalledWith('/api/changes/update', { url: 'u', strategy: 'merge', tool: 'claude' })

    await act(async () => render(<UpdatePanel change={{ ...change, ref: { ...change.ref, number: 8 } }} setup={setup} own />, root))
    expect([...root.querySelectorAll<HTMLInputElement>('input[name=update-strategy]')][1].disabled).toBe(false)
  })
})
