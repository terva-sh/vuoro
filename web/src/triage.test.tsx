// @vitest-environment jsdom
import { render } from 'preact'
import { act } from 'preact/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { api, post } from './api'
import { TriagePanel } from './triage'
import { WriteContext } from './write'
import type { Change, Run, Setup } from './types'

vi.mock('./api', () => ({ api: vi.fn(), post: vi.fn() }))
const thread = { id: 't1', anchor: null, resolvable: true, resolved: false, notes: [{ id: 'n1', author: { username: 'reviewer', bot: false }, body: 'Could this race?', system: false, at: '2026-10-01T00:00:00Z' }] }
const change: Change = { kind: 'gitlab', capabilities: { drafts: true, reply: true, resolve: true, approve: true, unapprove: true, request_changes: true, suggestions: true }, ref: { host: 'lab.test', project: 'team/repo', number: 1 }, web_url: 'https://lab.test/team/repo/-/merge_requests/1', title: 'My change', description: '', author: { username: 'me', bot: false }, diff: { head: 'head', base: 'base', start: 'base' }, source_branch: 'feature', target_branch: 'main', state: 'open', draft: false, mergeability: '', created: '', updated: '', reviewers: [], assignees: [], participants: [], requests: [], reviews: [], pushes: [], files: [], drafts: [], threads: [thread] }
const run: Run = { id: 'r1', mode: 'triage', kind: 'gitlab', ref: change.ref, title: 'My change', web_url: change.web_url, head: 'head', base: 'base', status: 'idle', cost_usd: 0, created: '2026-10-01T00:00:00Z', triage_threads: [thread], sessions: [], drafts: [{ id: 'd1', thread_id: 't1', category: 'discuss', reason: 'Need reproduction evidence', description: 'Could you share an example?', title: '', severity: 'info', status: 'pending', revision: 1, session: 's1', drafted_by: 'Fake agent', updated: '' }] }
const setup: Setup = { editable: true, base: '', sources: [], forges: [{ name: 'Work', url: 'https://lab.test', username: 'me', kind: 'gitlab' }], tools: [{ name: 'fake', tool: 'Fake tool', state: 'ok' }], presets: [], profiles: [], waived: [], scheme: '' }
let root: HTMLDivElement
const mount = async (c = change, writable = true, available = setup) => {
  root = document.createElement('div')
  await act(async () => render(<WriteContext.Provider value={writable ? { url: c.web_url, caps: c.capabilities, act: vi.fn(async () => {}) } : undefined}><TriagePanel change={c} setup={available} onPick={() => {}} /></WriteContext.Provider>, root))
  await act(async () => {})
  await act(async () => {})
}
const click = async (name: string) => { const b = [...root.querySelectorAll('button')].find((b) => b.textContent === name); expect(b).toBeDefined(); await act(async () => b!.click()); await act(async () => {}) }
afterEach(() => { if (root) act(() => render(null, root)); vi.clearAllMocks(); vi.useRealTimers() })
const serve = () => vi.mocked(api).mockImplementation(async (path) => path === '/api/runs' ? [run] : run)

describe('author triage interactions', () => {
  it('shows reason beside the thread and saves edits locally without posting', async () => {
    serve(); vi.mocked(post).mockResolvedValue({ ...run, drafts: [{ ...run.drafts[0], description: 'My wording', category: 'must_fix', edited: true }] })
    await mount()
    expect(root.querySelector('#triage-discuss')?.textContent).toContain('Could this race?')
    expect(root.querySelector('#triage-discuss')?.textContent).toContain('Need reproduction evidence')
    expect(post).not.toHaveBeenCalled()
    await click('Edit reply and category')
    const textarea = root.querySelector('textarea')!
    await act(async () => { textarea.value = 'My wording'; textarea.dispatchEvent(new Event('input', { bubbles: true })) })
    await act(async () => { root.querySelector('.composer')!.dispatchEvent(new Event('submit', { bubbles: true, cancelable: true })) })
    await act(async () => {})
    expect(post).toHaveBeenCalledWith('/api/runs/r1/drafts/d1/edit-reply', { body: 'My wording', category: 'discuss' })
    expect(root.querySelector('#triage-must_fix')?.textContent).toContain('My wording')
    expect(root.textContent).toContain('edited by you')
  })
  it('disables acceptance for changed undated discussion or moved head', async () => {
    serve()
    await mount({ ...change, threads: [{ ...thread, notes: [{ ...thread.notes[0], body: 'Changed after triage' }] }] })
    const accept = [...root.querySelectorAll('button')].find((b) => b.textContent === 'Add to pending replies')!
    expect(accept.disabled).toBe(true)
    expect(root.textContent).toContain('discussion or head changed')
    expect(post).not.toHaveBeenCalled()
  })
  it('supports local read-only replies and absent tools without exposing posting', async () => {
    serve(); await mount({ ...change, kind: 'forgejo', capabilities: { drafts: false, reply: false, resolve: false, approve: false, unapprove: false, request_changes: false, suggestions: false } }, false, { ...setup, tools: [] })
    expect(root.textContent).toContain('No agent tool is available')
    expect([...root.querySelectorAll('button')].find((b) => b.textContent === 'Run triage again')?.disabled).toBe(true)
    expect(root.textContent).toContain('Posting on this forge is unavailable')
    expect(root.textContent).toContain('Edit reply and category')
    expect(root.textContent).not.toContain('Add to pending replies')
    expect(post).not.toHaveBeenCalled()
  })
  it.each(['resolved', 'removed', 'closed'])('retains saved replies for %s threads but refuses acceptance', async (state) => {
    serve()
    await mount({ ...change, state: state === 'closed' ? 'closed' : 'open', threads: state === 'removed' ? [] : [{ ...thread, resolved: state === 'resolved' }] })
    expect(root.textContent).toContain('Could you share an example?')
    expect([...root.querySelectorAll('button')].find((b) => b.textContent === 'Add to pending replies')?.disabled).toBe(true)
    expect([...root.querySelectorAll('button')].find((b) => b.textContent === 'Copy reply')?.disabled).toBe(false)
    if (state !== 'closed') expect(root.textContent).toContain('no longer open')
    expect(post).not.toHaveBeenCalled()
  })
  it('does not overwrite a local edit with an older in-flight poll', async () => {
    vi.useFakeTimers(); serve(); await mount()
    let resolveOld!: (value: Run) => void
    vi.mocked(api).mockImplementationOnce(() => new Promise((resolve) => { resolveOld = resolve as typeof resolveOld }))
    await act(async () => { await vi.advanceTimersByTimeAsync(1500) })
    vi.mocked(post).mockResolvedValue({ ...run, drafts: [{ ...run.drafts[0], description: 'My newer wording', edited: true }] })
    await click('Edit reply and category')
    await act(async () => { root.querySelector('.composer')!.dispatchEvent(new Event('submit', { bubbles: true, cancelable: true })) })
    await act(async () => {})
    await act(async () => { resolveOld(run) })
    expect(root.textContent).toContain('My newer wording')
    expect(root.textContent).not.toContain('Could you share an example?')
  })
  it('resets selection when reused for another change and ignores the previous edit response', async () => {
    const otherChange = { ...change, ref: { ...change.ref, number: 2 }, web_url: 'https://lab.test/team/repo/-/merge_requests/2' }
    const otherRun = { ...run, id: 'r2', ref: otherChange.ref, web_url: otherChange.web_url, drafts: [{ ...run.drafts[0], description: 'The second change reply' }] }
    vi.mocked(api).mockImplementation(async (path) => path === '/api/runs' ? [run, otherRun] : path === '/api/runs/r2' ? otherRun : run)
    await mount()
    const previousPoll = vi.mocked(api).mock.calls.find((c) => c[0] === '/api/runs/r1')!
    let finishEdit!: (value: Run) => void
    vi.mocked(post).mockImplementationOnce(() => new Promise((resolve) => { finishEdit = resolve as typeof finishEdit }))
    await click('Edit reply and category')
    await act(async () => { root.querySelector('.composer')!.dispatchEvent(new Event('submit', { bubbles: true, cancelable: true })) })
    await act(async () => render(<WriteContext.Provider value={{ url: otherChange.web_url, caps: otherChange.capabilities, act: vi.fn(async () => {}) }}><TriagePanel change={otherChange} setup={setup} onPick={() => {}} /></WriteContext.Provider>, root))
    await act(async () => {})
    await act(async () => {})
    expect(previousPoll[1]?.signal?.aborted).toBe(true)
    expect(root.textContent).toContain('The second change reply')
    await act(async () => { finishEdit({ ...run, drafts: [{ ...run.drafts[0], description: 'Late first-change edit' }] }) })
    expect(root.textContent).toContain('The second change reply')
    expect(root.textContent).not.toContain('Late first-change edit')
    await click('Edit reply and category')
    vi.mocked(post).mockResolvedValue(otherRun)
    await act(async () => { root.querySelector('.composer')!.dispatchEvent(new Event('submit', { bubbles: true, cancelable: true })) })
    expect(vi.mocked(post).mock.lastCall?.[0]).toBe('/api/runs/r2/drafts/d1/edit-reply')
  })
  it('keeps a newly started run when the earlier run-list request arrives late', async () => {
    let finishList!: (value: Run[]) => void
    vi.mocked(api).mockImplementation((path) => path === '/api/runs' ? new Promise((resolve) => { finishList = resolve as typeof finishList }) : Promise.resolve(run) as ReturnType<typeof api>)
    vi.mocked(post).mockResolvedValue(run)
    await mount()
    await click('Triage open threads')
    await act(async () => { finishList([]) })
    expect(root.textContent).toContain('Could you share an example?')
    expect(root.querySelector('label select')?.children.length).toBe(1)
  })
  it('aborts polling when leaving and keeps old drafts on polling failure', async () => {
    vi.useFakeTimers(); serve(); await mount()
    vi.mocked(api).mockRejectedValueOnce(new Error('Run could not load'))
    await act(async () => { await vi.advanceTimersByTimeAsync(1500) })
    expect(root.textContent).toContain('Run could not load')
    expect(root.textContent).toContain('Could you share an example?')
    const call = vi.mocked(api).mock.calls.find((c) => c[0] === '/api/runs/r1')!
    act(() => render(null, root))
    expect(call[1]?.signal?.aborted).toBe(true)
    const count = vi.mocked(api).mock.calls.length
    await vi.advanceTimersByTimeAsync(5000)
    expect(vi.mocked(api).mock.calls.length).toBe(count)
  })
})
