// @vitest-environment jsdom
import { render, type ComponentChildren } from 'preact'
import { useState } from 'preact/hooks'
import { act } from 'preact/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { api, post } from './api'
import { GuideContext, useGuideState } from './guide'
import { Inbox, type InboxView } from './inbox'
import { LocalTab } from './local'
import { ProjectsTab } from './projects'
import { RunPanel } from './run'
import type { Change, HintID, LocalView, Run, Setup } from './types'

vi.mock('./api', () => ({ api: vi.fn(), post: vi.fn(), bindings: vi.fn() }))

const background = { refresh_minutes: 15, notify: true, autostart: false, login: true, dock: false, dock_here: false, dock_restart: false }
const setupWith = (over: Partial<Setup> = {}): Setup => ({
  editable: true, base: '', sources: [], forges: [], tools: [{ name: 'fake', tool: 'Fake Tool', state: 'ok' }],
  presets: [{ name: 'default', reviewers: [{ profile: 'code' }] }], profiles: [], waived: [], background, scheme: '', tab: '', hints: [], tour: { state: '' }, ...over,
})
// No agent tool answered, and no ticket store, which no hint reads.
const noTool = setupWith({ tools: [{ name: 'fake', tool: 'Fake Tool', state: 'missing', error: 'not installed' }] })
const account = { key: 'gl', name: 'Work', kind: 'gitlab', host: 'lab.test', username: 'me', last_attempt: '', last_success: '' }
const emptyInbox = (accounts: InboxView['accounts']): InboxView => ({ accounts, mine: [], theirs: [], snoozed: [] })
const emptyLocal: LocalView = { roots: [], added: [], scanned: '', built: new Date().toISOString(), hidden: [], unreadable: [], needs: 0, quiet_days: 14, projects: [] }
const change = { kind: 'gitlab', ref: { host: 'lab.test', project: 'team/repo', number: 2 }, web_url: 'https://lab.test/team/repo/-/merge_requests/2', author: { username: 'author' }, diff: { head: 'head', base: 'base', start: 'base' }, files: [], threads: [], drafts: [] } as unknown as Change
const draft = { id: 'd1', severity: 'high', title: 'A finding', description: 'It is wrong.', status: 'pending', revision: 1, session: 's1', drafted_by: 'Fake', updated: '' } as const
const idle: Run = { id: 'r1', kind: 'gitlab', ref: change.ref, title: 'A change', web_url: change.web_url, head: 'head', base: 'base', created: '', status: 'idle', cost_usd: 0, sessions: [], drafts: [draft] }

const root = document.createElement('div')
document.body.append(root)
const settle = async () => { for (let i = 0; i < 6; i++) await act(async () => { await Promise.resolve() }) }
const hint = (id: HintID) => root.querySelector<HTMLElement>(`[data-hint="${id}"]`)
const buttons = () => [...root.querySelectorAll('button')].map((b) => b.textContent)
afterEach(() => { act(() => render(null, root)); vi.resetAllMocks() })

function serve(routes: Record<string, unknown>) {
  vi.mocked(api).mockImplementation(async (path: string) => {
    for (const [prefix, v] of Object.entries(routes)) if (path.startsWith(prefix)) return v as never
    throw new Error(`unexpected ${path}`)
  })
  vi.mocked(post).mockResolvedValue({})
}

// Keeps the guide in a setup of its own, as App does.
function Guided({ setup, children }: { setup: Setup; children: ComponentChildren }) {
  const [s, setS] = useState<Setup | undefined>(setup)
  return <GuideContext.Provider value={useGuideState(s, setS)}>{children}</GuideContext.Provider>
}

// Each empty or first-time screen, with what it renders inside.
const screens: { id: HintID; routes: Record<string, unknown>; view: (s: Setup) => ComponentChildren }[] = [
  { id: 'reviews-no-forge', routes: { '/api/inbox': emptyInbox([]) }, view: () => <Inbox onOpen={() => {}} /> },
  { id: 'reviews-quiet', routes: { '/api/inbox': emptyInbox([account]) }, view: () => <Inbox onOpen={() => {}} /> },
  { id: 'local-empty', routes: { '/api/local': emptyLocal }, view: (s) => <LocalTab setup={s} /> },
  { id: 'projects-empty', routes: { '/api/projects': { projects: [], accounts: [] } }, view: () => <ProjectsTab onOpen={() => {}} /> },
  { id: 'change-no-runs', routes: { '/api/runs': [] }, view: (s) => <RunPanel change={change} setup={s} onPick={() => {}} onDrafts={() => {}} /> },
  { id: 'first-drafts', routes: { '/api/runs/': idle, '/api/runs': [idle] }, view: (s) => <RunPanel change={change} setup={s} onPick={() => {}} onDrafts={() => {}} /> },
]

describe('getting-started hints', () => {
  for (const screen of screens) {
    it(`shows ${screen.id} on its screen, and keeps it dismissed`, async () => {
      serve(screen.routes)
      await act(async () => render(<Guided setup={setupWith()}>{screen.view(setupWith())}</Guided>, root))
      await settle()
      expect(hint(screen.id)).not.toBeNull()
      await act(async () => [...hint(screen.id)!.querySelectorAll('button')].find((b) => b.textContent === 'Got it')!.click())
      expect(hint(screen.id)).toBeNull()
      expect(post).toHaveBeenCalledWith('/api/prefs/hint', { hint: screen.id, dismissed: true })

      // After a restart setup carries it, and it stays away.
      act(() => render(null, root))
      const restarted = setupWith({ hints: [screen.id] })
      await act(async () => render(<Guided setup={restarted}>{screen.view(restarted)}</Guided>, root))
      await settle()
      expect(hint(screen.id)).toBeNull()
    })

    it(`renders ${screen.id} with no agent tool, and points at nothing that fails`, async () => {
      serve(screen.routes)
      await act(async () => render(<Guided setup={noTool}>{screen.view(noTool)}</Guided>, root))
      await settle()
      expect(hint(screen.id)).not.toBeNull()
      // A run cannot start, so neither the hint nor the page offers to.
      expect(buttons()).not.toContain('Start a review')
    })
  }

  it('names the missing tool where the screen needs one', async () => {
    for (const screen of screens.filter((s) => s.id === 'change-no-runs' || s.id === 'local-empty')) {
      serve(screen.routes)
      await act(async () => render(<Guided setup={noTool}>{screen.view(noTool)}</Guided>, root))
      await settle()
      expect(hint(screen.id)!.textContent).toContain('needs Fake Tool installed and signed in')
      act(() => render(null, root))
    }
  })

  it('shows none before setup says which were dismissed', async () => {
    serve({ '/api/local': emptyLocal })
    function Unloaded() {
      const [s, setS] = useState<Setup>()
      return <GuideContext.Provider value={useGuideState(s, setS)}><LocalTab /></GuideContext.Provider>
    }
    await act(async () => render(<Unloaded />, root))
    await settle()
    expect(hint('local-empty')).toBeNull()
    expect(root.textContent).toContain('No local projects yet')
  })
})
