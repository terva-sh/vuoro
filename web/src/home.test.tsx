// @vitest-environment jsdom
import { render } from 'preact'
import { act } from 'preact/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { api, post } from './api'
import { Home } from './app'
import type { InboxView } from './inbox'
import type { HomeTab, LocalView, Setup } from './types'

vi.mock('./api', () => ({ api: vi.fn(), post: vi.fn() }))

const setup = (tab: HomeTab): Setup => ({
  editable: false, base: 'b1', config_path: '/c.json', forges: [], tools: [], presets: [], profiles: [], waived: [], scheme: '', tab, sources: [],
})
const inbox: InboxView = {
  accounts: [],
  mine: [{ ref: { host: 'gitlab.example', project: 'team/repo', number: 1 }, kind: 'gitlab', account_key: 'gl', account: 'Work', title: 'Review the change', web_url: 'https://gitlab.example/team/repo/-/merge_requests/1', draft: false, roles: ['reviewer'], turn: { owner: 'mine', since: null, last_activity: null, reasons: [{ rule: 'R1', text: 'Your review is requested.', since: null }] } }],
  theirs: [], snoozed: [],
}
const local = (needs: number): LocalView => ({
  roots: [], added: ['/w/proj'], scanned: '', built: new Date().toISOString(), hidden: [], unreadable: [], needs,
  projects: [{ key: '/w/proj/.git', name: 'proj', path: '/w/proj', base: 'main', added: true, base_chosen: false, worktrees: [],
    branches: [{ name: 'feature', tip: 'abc', ahead: 2, committed: new Date().toISOString(), worktrees: [], fixes: [], needs: ['never reviewed'] }] }],
})

const root = document.createElement('div')
document.body.append(root)
const tab = (name: string) => [...root.querySelectorAll('[role="tab"]')].find((b) => b.textContent!.startsWith(name)) as HTMLButtonElement
const panel = (id: string) => root.querySelector(`#panel-${id}`) as HTMLElement
const settle = async () => { for (let i = 0; i < 5; i++) await act(async () => { await Promise.resolve() }) }
afterEach(() => { act(() => render(null, root)); vi.resetAllMocks() })

function serve(cached: LocalView | null, built: LocalView) {
  vi.mocked(api).mockImplementation(async (path: string) => {
    if (path.startsWith('/api/inbox')) return inbox
    if (path === '/api/local?cached=1') return cached
    if (path === '/api/local') return built
    throw new Error(`unexpected ${path}`)
  })
  vi.mocked(post).mockResolvedValue({})
}

describe('the home tabs', () => {
  it('opens on the tab used last, counts both, and keeps a switch', async () => {
    serve(local(3), local(1))
    const open = vi.fn()
    await act(async () => render(<Home setup={setup('local')} onSetup={() => {}} runs={[]} onOpen={open} onOpenRun={() => {}} opening={false} />, root))
    await settle()
    expect(tab('Local').getAttribute('aria-selected')).toBe('true')
    expect(panel('reviews').hidden).toBe(true)
    // Local's count is the one it built when shown, the cached one before.
    expect(tab('Local').textContent).toBe('Local 1')
    expect(panel('local').textContent).toContain('never reviewed')
    // The inbox polls behind its tab, so Reviews counts too.
    expect(tab('Reviews').textContent).toBe('Reviews 1')

    // The inbox's keys are not Local's.
    act(() => { window.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter' })) })
    expect(open).not.toHaveBeenCalled()

    await act(async () => tab('Reviews').click())
    expect(post).toHaveBeenCalledWith('/api/prefs/tab', { tab: 'reviews' })
    expect(panel('reviews').hidden).toBe(false)
    expect(panel('local').hidden).toBe(true)
    act(() => { window.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter' })) })
    expect(open).toHaveBeenCalledWith(inbox.mine[0].web_url)
  })

  it('reads the local view only once Local is shown, and counts it from the cache before', async () => {
    serve(local(4), local(4))
    await act(async () => render(<Home setup={setup('')} onSetup={() => {}} runs={[]} onOpen={() => {}} onOpenRun={() => {}} opening={false} />, root))
    await settle()
    expect(tab('Reviews').getAttribute('aria-selected')).toBe('true')
    expect(tab('Local').textContent).toBe('Local 4')
    expect(api).not.toHaveBeenCalledWith('/api/local')
    // An arrow key moves between the tabs.
    await act(async () => { tab('Reviews').dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowRight', bubbles: true })) })
    await settle()
    expect(tab('Local').getAttribute('aria-selected')).toBe('true')
    expect(document.activeElement).toBe(tab('Local'))
    expect(api).toHaveBeenCalledWith('/api/local')
    expect(post).toHaveBeenCalledWith('/api/prefs/tab', { tab: 'local' })
  })

  it('shows no count for Local before anything was built', async () => {
    serve(null, local(0))
    await act(async () => render(<Home setup={setup('reviews')} onSetup={() => {}} runs={[]} onOpen={() => {}} onOpenRun={() => {}} opening={false} />, root))
    await settle()
    expect(tab('Local').textContent).toBe('Local')
  })
})
