// @vitest-environment jsdom
import { render } from 'preact'
import { act } from 'preact/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { api, post } from './api'
import { ProjectTriage } from './projecttriage'
import type { ProjectRow } from './projects'
import type { Run } from './types'

vi.mock('./api', () => ({ api: vi.fn(), post: vi.fn() }))

const row = (n: number, edit: Partial<ProjectRow> = {}): ProjectRow => ({
  ref: { host: 'lab.test', project: 'team/repo', number: n }, kind: 'gitlab', title: `Change ${n}`,
  web_url: `https://lab.test/team/repo/-/merge_requests/${n}`, author: 'ann', draft: false, source_branch: `b${n}`, target_branch: 'main',
  opened: '2026-10-20T00:00:00Z', place: 'unreviewed', since: null, reasons: [], holder: [],
  reviewers: [], assignees: [], labels: [], threads: { total: 0, unresolved: 0, author: 0, reviewers: 0 }, ...edit,
})
const proposal = { id: 'd1', change: 1, reviewers: ['bob', 'cat'], labels: ['backend'], unblock: 'Needs a reviewer', reason: 'Bob owns api/',
  severity: 'medium', title: '', description: '', status: 'pending', revision: 1, session: 's1', drafted_by: 'Fake', updated: '2026-10-27T00:00:00Z' }
const run = { id: 'r1', mode: 'project-triage', kind: 'gitlab', ref: { host: 'lab.test', project: 'team/repo', number: 0 }, title: 'Team / Repo', web_url: '',
  head: '', base: '', created: '2026-10-27T00:00:00Z', status: 'idle', cost_usd: 0, sessions: [], drafts: [proposal] } as unknown as Run
const root = document.body.appendChild(document.createElement('div'))
const button = (name: string) => [...root.querySelectorAll('button')].find((b) => b.textContent?.trim().startsWith(name))
const settle = async () => { for (let i = 0; i < 4; i++) await act(async () => { await Promise.resolve() }) }
afterEach(() => { act(() => render(null, root)); vi.resetAllMocks() })

describe('project agent triage', () => {
  it('starts on the changes shown, leaving drafts out', async () => {
    vi.mocked(api).mockImplementation(async (path: string) => (path === '/api/setup' ? { tools: [{ name: 'claude', tool: 'Claude Code', state: 'ready' }] } : null) as never)
    await act(async () => render(<ProjectTriage projectKey="k1" kind="gitlab" rows={[row(1), row(2, { place: 'draft', draft: true })]} onView={() => {}} />, root))
    await settle()
    vi.mocked(post).mockResolvedValueOnce({ ...run, status: 'working', drafts: [] } as never)
    await act(async () => button('Triage 1 shown')!.click())
    expect(post).toHaveBeenCalledWith('/api/projects/k1/triage', { numbers: [1], tool: 'claude' })
  })
  it('accepts edited proposals after one confirmation, and shows a stale one waiting', async () => {
    vi.mocked(api).mockImplementation(async (path: string) => (path === '/api/setup' ? { tools: [] } : run) as never)
    const onView = vi.fn()
    await act(async () => render(<ProjectTriage projectKey="k1" kind="gitlab" rows={[row(1)]} onView={onView} />, root))
    await settle()
    expect(root.textContent).toContain('Needs a reviewer')
    expect(button('Accept')).toBeUndefined()
    act(() => (root.querySelector('[aria-label="Leave out cat"]') as HTMLButtonElement).click())
    act(() => { (root.querySelector('input[type="checkbox"]') as HTMLInputElement).click() })
    act(() => button('Accept 1 selected')!.click())
    // The dialog says what will be written, and nothing is sent until the person accepts.
    expect(root.querySelector('[role="dialog"]')!.textContent).toContain('!1: add reviewers bob; add labels backend')
    expect(post).not.toHaveBeenCalled()
    vi.mocked(post).mockResolvedValueOnce({ view: {}, run, outcomes: [{ id: 'd1', result: 'stale' }] } as never)
    await act(async () => { [...root.querySelectorAll('[role="dialog"] button')].find((b) => b.textContent === 'Accept')!.dispatchEvent(new MouseEvent('click', { bubbles: true })) })
    await settle()
    expect(post).toHaveBeenCalledWith('/api/projects/k1/triage/r1/accept', { proposals: [{ id: 'd1', reviewers: ['bob'] }] })
    expect(onView).toHaveBeenCalled()
    expect(root.textContent).toContain('The change moved since the agent read it')
    // Accepting it again, having seen it is stale, says so.
    act(() => { (root.querySelector('input[type="checkbox"]') as HTMLInputElement).click() })
    act(() => button('Accept 1 selected')!.click())
    vi.mocked(post).mockResolvedValueOnce({ view: {}, run: { ...run, drafts: [{ ...proposal, decision: 'kept' }] }, outcomes: [{ id: 'd1', result: 'accepted' }] } as never)
    await act(async () => { [...root.querySelectorAll('[role="dialog"] button')].find((b) => b.textContent === 'Accept')!.dispatchEvent(new MouseEvent('click', { bubbles: true })) })
    await settle()
    expect(post).toHaveBeenLastCalledWith('/api/projects/k1/triage/r1/accept', { proposals: [{ id: 'd1', reviewers: ['bob'], stale: true }] })
    expect(root.textContent).toContain('✓ accepted')
  })
})
