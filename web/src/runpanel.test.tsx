// @vitest-environment jsdom
import { render } from 'preact'
import { act } from 'preact/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { api } from './api'
import { RunPanel } from './run'
import type { Change, Run, Setup } from './types'

vi.mock('./api', () => ({ api: vi.fn(), post: vi.fn() }))

const change = { kind: 'gitlab', ref: { host: 'lab.test', project: 'team/repo', number: 2 }, web_url: 'https://lab.test/team/repo/-/merge_requests/2', diff: { head: 'head', base: 'base', start: 'base' }, files: [], threads: [], drafts: [] } as unknown as Change
const setup: Setup = { editable: true, base: '', sources: [], forges: [], tools: [{ name: 'fake', tool: 'Fake Tool', state: 'ok' }], presets: [{ name: 'default', reviewers: [{ profile: 'code' }] }], profiles: [], waived: [], background: { refresh_minutes: 15, notify: true, autostart: false, login: true }, scheme: '', tab: '' }
const draft = { id: 'd1', severity: 'high', title: 'A finding', description: 'It is wrong.', status: 'pending', revision: 1, session: 's1', drafted_by: 'Fake', updated: '' } as const
const preparing: Run = { id: 'r1', kind: 'gitlab', ref: change.ref, title: 'A change', web_url: change.web_url, head: 'head', base: 'base', created: '2026-10-05T00:00:00Z', status: 'preparing', cost_usd: 0, sessions: [], drafts: [] }
const idle: Run = { ...preparing, status: 'idle', drafts: [draft] }
const root = document.createElement('div')
afterEach(() => { act(() => render(null, root)); vi.resetAllMocks() })

describe('the run panel', () => {
  it('relabels the selected run as it settles, and an undecided finding is not styled as the header form', async () => {
    vi.mocked(api).mockImplementation(async (path: string) => (path === '/api/runs' ? [preparing] : idle) as never)
    await act(async () => render(<RunPanel change={change} setup={setup} onPick={() => {}} onDrafts={() => {}} />, root))
    for (let i = 0; i < 5; i++) await act(async () => {})
    const runSelect = [...root.querySelectorAll('select')].find((s) => [...s.options].some((o) => o.value === 'r1'))!
    expect(runSelect.options[runSelect.selectedIndex].textContent).toContain('idle')
    const finding = root.querySelector('details.finding')!
    expect(finding.classList.contains('undecided')).toBe(true)
    expect(finding.classList.contains('open')).toBe(false)
  })
})
