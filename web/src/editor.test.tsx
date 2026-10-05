// @vitest-environment jsdom
import { render } from 'preact'
import { act } from 'preact/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { api } from './api'
import { Editor } from './run'
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

  it('shows the commits beside their findings and names the ones left alone', async () => {
    vi.mocked(api).mockResolvedValue({
      session: 's2', branch: 'vuoro/fix-7-abc123', base: 'abcdef1234', started: '',
      commits: [{ sha: '0123456789', subject: 'Add adds', findings: ['d1'] }], files: [],
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
    act(() => [...root.querySelectorAll('button')].find((b) => b.textContent === 'Discard…')!.click())
    expect(root.textContent).toContain('Delete the branch')
  })
})
