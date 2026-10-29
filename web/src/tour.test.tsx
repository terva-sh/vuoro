// @vitest-environment jsdom
import { render } from 'preact'
import { useState } from 'preact/hooks'
import { act } from 'preact/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { api, post } from './api'
import { App } from './app'
import { ChangeView } from './change'
import { GuideContext, useGuideState } from './guide'
import { nextStep, tourSteps } from './tour'
import type { Change, Setup, Tour } from './types'

vi.mock('./api', () => ({ api: vi.fn(), post: vi.fn(), bindings: vi.fn() }))

const background = { refresh_minutes: 15, notify: true, autostart: false, login: true, dock: false, dock_here: false, dock_restart: false }
const setupWith = (over: Partial<Setup> = {}): Setup => ({
  editable: true, base: 'b1', config_path: '/c.json', sources: [], tools: [], presets: [{ name: 'default', reviewers: [{ profile: 'code' }] }], profiles: [], waived: [], background,
  scheme: '', tab: '', hints: [], tour: { state: '' },
  forges: [{ name: 'Work', url: 'https://lab.test', username: 'me', kind: 'gitlab' }], ...over,
})
const change: Change = {
  kind: 'gitlab', capabilities: { drafts: true, reply: true, resolve: true, approve: true, unapprove: true, request_changes: true, suggestions: true },
  ref: { host: 'lab.test', project: 'team/repo', number: 2 }, title: 'A change', description: '', author: { username: 'author', bot: false },
  web_url: 'https://lab.test/team/repo/-/merge_requests/2', source_branch: 'feature', target_branch: 'main',
  diff: { head: 'head', base: 'base', start: 'base' }, state: 'open', draft: false, mergeability: '',
  created: '', updated: '', reviewers: [], assignees: [], participants: [], requests: [], reviews: [], pushes: [], files: [], threads: [], drafts: [],
}

const root = document.createElement('div')
document.body.append(root)
const settle = async () => { for (let i = 0; i < 6; i++) await act(async () => { await Promise.resolve() }) }
const popover = () => document.querySelector<HTMLElement>('.tour')
const step = () => popover()?.dataset.step
const button = (label: string) => [...document.querySelectorAll('button')].find((b) => b.textContent === label) as HTMLButtonElement
const press = async (label: string) => { await act(async () => button(label).click()); await settle() }
afterEach(() => { act(() => render(null, root)); vi.resetAllMocks() })

// serve answers the window's reads; every write answers what prefs would.
function serve(setup: Setup) {
  vi.mocked(api).mockImplementation(async (path: string, init?: RequestInit) => {
    if (init?.method && init.method !== 'GET') throw new Error(`api wrote ${path}`)
    if (path === '/api/hello') return { version: 'test' } as never
    if (path === '/api/setup') return setup as never
    if (path === '/api/runs') return [] as never
    if (path.startsWith('/api/inbox')) return { accounts: [], mine: [], theirs: [], snoozed: [] } as never
    if (path.startsWith('/api/local')) return null as never
    throw new Error(`unexpected ${path}`)
  })
  vi.mocked(post).mockResolvedValue({})
}

// Every write the tour made. The tour's only write is the person's
// progress, so each must be to /api/prefs/tour.
const writes = () => vi.mocked(post).mock.calls.map(([path]) => path)
const tourWrites = () => vi.mocked(post).mock.calls.filter(([path]) => path === '/api/prefs/tour').map(([, body]) => body as Tour)

function GuidedChange({ setup }: { setup: Setup }) {
  const [s, setS] = useState<Setup | undefined>(setup)
  return <GuideContext.Provider value={useGuideState(s, setS)}><ChangeView change={change} setup={s} onChange={() => {}} onSetup={() => {}} /></GuideContext.Provider>
}

describe('the tour', () => {
  it('is offered when setup finishes, and declining it is kept', async () => {
    serve(setupWith({ forges: [] }))
    await act(async () => render(<App />, root))
    await settle()
    await press('Skip setup for now')
    expect(root.textContent).toContain('New to vuoro?')
    await press('No thanks')
    expect(tourWrites()).toEqual([{ state: 'off' }])
    expect(root.textContent).not.toContain('New to vuoro?')
    expect(popover()).toBeNull()
  })

  it('walks the home and change parts without starting, opening, or posting anything', async () => {
    serve(setupWith({ forges: [] }))
    await act(async () => render(<App />, root))
    await settle()
    await press('Skip setup for now')
    await press('Show me around')
    // Each step is anchored to its real element, which is outlined.
    for (const id of ['home-tabs', 'open-url', 'settings']) {
      expect(step()).toBe(id)
      expect(document.querySelector(`[data-tour="${id}"]`)!.classList.contains('tour-target')).toBe(true)
      await press('Next')
    }
    // The change part waits for a change the person opens.
    expect(popover()).toBeNull()
    expect(tourWrites().at(-1)).toEqual({ state: 'on', step: 'settings' })
    act(() => render(null, root))

    await act(async () => render(<GuidedChange setup={setupWith({ tour: { state: 'on', step: 'settings' } })} />, root))
    await settle()
    for (const id of ['change-tabs', 'review-tab']) {
      expect(step()).toBe(id)
      await press('Next')
    }
    expect(step()).toBe('submit')
    await press('Finish')
    expect(popover()).toBeNull()
    expect(tourWrites().at(-1)).toEqual({ state: 'done', step: 'submit' })

    // Nothing but progress was written, and nothing was opened or started.
    expect(new Set(writes())).toEqual(new Set(['/api/prefs/tour']))
  })

  it('skips a step whose element is absent, and ends there when nothing follows', async () => {
    // A forge vuoro cannot post to shows no Submit.
    await act(async () => render(<GuidedChange setup={setupWith({ forges: [], tour: { state: 'on', step: 'change-tabs' } })} />, root))
    vi.mocked(api).mockResolvedValue([] as never)
    vi.mocked(post).mockResolvedValue({})
    await settle()
    expect(step()).toBe('review-tab')
    expect(button('Finish')).toBeTruthy()
    await press('Finish')
    expect(tourWrites()).toEqual([{ state: 'done', step: 'review-tab' }])
  })

  it('never anchors to an element inside a hidden panel', () => {
    root.innerHTML = '<div hidden><nav data-tour="home-tabs"></nav></div><form data-tour="open-url"></form>'
    expect(nextStep('home')?.step.id).toBe('open-url')
    expect(nextStep('home', 'open-url')).toBeUndefined()
    root.innerHTML = ''
  })

  it('ends at any step with Esc, and Settings replays it', async () => {
    serve(setupWith({ tour: { state: 'on', step: 'home-tabs' } }))
    await act(async () => render(<App />, root))
    await settle()
    expect(step()).toBe('open-url')
    await act(async () => { window.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' })) })
    await settle()
    expect(popover()).toBeNull()
    expect(tourWrites()).toEqual([{ state: 'off', step: 'open-url' }])

    await press('⚙ Settings')
    await press('Replay the tour')
    expect(tourWrites().at(-1)).toEqual({ state: 'on' })
    expect(step()).toBe('home-tabs')
  })

  it('is offered until answered to a person who never saw first run', async () => {
    // A config written by hand: forges, so no first run.
    serve(setupWith())
    await act(async () => render(<App />, root))
    await settle()
    expect(root.textContent).toContain('New to vuoro?')
    await press('Show me around')
    expect(tourWrites()).toEqual([{ state: 'on' }])
    expect(step()).toBe('home-tabs')
    expect(root.textContent).not.toContain('New to vuoro?')
  })

  it('keeps Enter on Next from opening the inbox\'s selected change', async () => {
    serve(setupWith({ tour: { state: 'on' } }))
    const inbox = { accounts: [{ key: 'gl', name: 'Work', kind: 'gitlab', host: 'lab.test', username: 'me', last_attempt: '', last_success: '' }],
      mine: [{ ref: change.ref, kind: 'gitlab', account_key: 'gl', account: 'Work', title: 'Review me', web_url: change.web_url, draft: false, roles: ['reviewer'],
        turn: { owner: 'mine', since: null, last_activity: null, reasons: [{ rule: 'R1', text: 'Your review is requested.', since: null }] } }], theirs: [], snoozed: [] }
    const base = vi.mocked(api).getMockImplementation()!
    vi.mocked(api).mockImplementation(async (path: string, init?: RequestInit) => path.startsWith('/api/inbox') ? inbox as never : base(path, init))
    await act(async () => render(<App />, root))
    await settle()
    expect(step()).toBe('home-tabs')
    expect(root.textContent).toContain('Review me')
    await act(async () => { button('Next').dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true })) })
    await settle()
    // Opening a change posts to /api/changes; nothing did.
    expect(writes()).not.toContain('/api/changes')
    // The same key outside the tour still opens it, so the test can fail.
    await act(async () => { window.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter' })) })
    await settle()
    expect(writes()).toContain('/api/changes')
  })

  it('answers the offer with Enter rather than opening the inbox\'s change', async () => {
    serve(setupWith())
    const inbox = { accounts: [{ key: 'gl', name: 'Work', kind: 'gitlab', host: 'lab.test', username: 'me', last_attempt: '', last_success: '' }],
      mine: [{ ref: change.ref, kind: 'gitlab', account_key: 'gl', account: 'Work', title: 'Review me', web_url: change.web_url, draft: false, roles: ['reviewer'],
        turn: { owner: 'mine', since: null, last_activity: null, reasons: [{ rule: 'R1', text: 'Your review is requested.', since: null }] } }], theirs: [], snoozed: [] }
    const base = vi.mocked(api).getMockImplementation()!
    vi.mocked(api).mockImplementation(async (path: string, init?: RequestInit) => path.startsWith('/api/inbox') ? inbox as never : base(path, init))
    await act(async () => render(<App />, root))
    await settle()
    await act(async () => { button('No thanks').dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true })) })
    await settle()
    expect(writes()).not.toContain('/api/changes')
  })

  it('replays from the home page when a change was open', async () => {
    serve(setupWith({ tour: { state: 'done', step: 'submit' } }))
    vi.mocked(post).mockImplementation(async (path: string) => (path === '/api/changes' ? change : {}) as never)
    await act(async () => render(<App />, root))
    await settle()
    const input = document.querySelector<HTMLInputElement>('input[type="url"]')!
    await act(async () => { input.value = change.web_url; input.dispatchEvent(new Event('input')) })
    await press('Open')
    expect(document.querySelector('[data-tour="change-tabs"]')).not.toBeNull()
    await press('⚙ Settings')
    await press('Replay the tour')
    expect(step()).toBe('home-tabs')
  })

  it('replays from the home page, not setup, for a person who skipped setup', async () => {
    serve(setupWith({ forges: [], tour: { state: 'done', step: 'submit' } }))
    await act(async () => render(<App />, root))
    await settle()
    await press('Skip setup for now')
    await press('⚙ Settings')
    await press('Replay the tour')
    expect(root.textContent).not.toContain('Set up vuoro')
    expect(step()).toBe('home-tabs')
  })

  it('lets an open modal take Esc first', async () => {
    serve(setupWith({ tour: { state: 'on', step: 'home-tabs' } }))
    await act(async () => render(<App />, root))
    await settle()
    expect(step()).toBe('open-url')
    await press('vuoro test')
    expect(document.querySelector('[aria-modal="true"]')).not.toBeNull()
    await act(async () => { window.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' })) })
    await settle()
    expect(document.querySelector('[aria-modal="true"]')).toBeNull()
    expect(step()).toBe('open-url')
    expect(tourWrites()).toEqual([])
  })

  it('lists the same steps as the server keeps', () => {
    // app.TourSteps in app/prefs.go.
    expect(tourSteps.map((s) => s.id)).toEqual(['home-tabs', 'open-url', 'settings', 'change-tabs', 'review-tab', 'submit'])
  })
})
