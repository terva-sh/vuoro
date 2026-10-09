// @vitest-environment jsdom
import { render } from 'preact'
import { act } from 'preact/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { api, bindings, post, type Bindings } from './api'
import { LocalTab } from './local'
import { standing } from './localproject'
import type { LocalBranch, LocalChange, LocalView, Setup } from './types'

vi.mock('./api', () => ({ api: vi.fn(), post: vi.fn(), bindings: vi.fn() }))
const root = document.createElement('div')
document.body.append(root) // a form submits only from the document
afterEach(() => { act(() => render(null, root)); vi.resetAllMocks() })
const settle = async () => { for (let i = 0; i < 5; i++) await act(async () => { await Promise.resolve() }) }
const button = (name: string) => [...root.querySelectorAll('button')].find((b) => b.textContent === name)!
const empty = (over: Partial<LocalView> = {}): LocalView => ({
  roots: [], added: [], scanned: '', built: new Date().toISOString(), hidden: [], unreadable: [], needs: 0, quiet_days: 14, projects: [], ...over,
})
const project = (key: string, name: string) => ({ key, name, path: `/w/${name}`, base: 'main', added: true, base_chosen: false, remotes: [], worktrees: [], branches: [] })
const window_ = (chooseFolder: Bindings['chooseFolder']) =>
  vi.mocked(bindings).mockReturnValue({ token: async () => 't', version: async () => 'v', report: async () => {}, chooseFolder })

const row = (name: string) => root.querySelector(`li.local-branch[aria-label="${name}"]`) as HTMLElement
const rowNames = () => [...root.querySelectorAll('li.local-branch')].map((li) => li.getAttribute('aria-label'))

const branch = (name: string, over: Partial<LocalBranch> = {}): LocalBranch => ({
  name, tip: 'abc', ahead: 1, committed: new Date().toISOString(), worktrees: [], fixes: [], needs: [], quiet: false, ...over,
})

describe('the Local tab', () => {
  it('lists what needs the person first, and folds the quiet branches until asked', async () => {
    const view = empty({ added: ['/w/p'], needs: 2, projects: [{ ...project('/w/p/.git', 'p'), branches: [
      branch('reviewed'),
      branch('fresh', { needs: ['never reviewed'] }),
      branch('left-open', { quiet: true, needs: ['1 finding undecided'] }),
      branch('old-1', { quiet: true }),
      branch('old-2', { quiet: true }),
    ] }] })
    vi.mocked(api).mockResolvedValue(view)
    const onCount = vi.fn()
    await act(async () => render(<LocalTab onCount={onCount} />, root))
    await settle()
    expect(onCount).toHaveBeenCalledWith(2)
    expect(rowNames()).toEqual(['fresh', 'left-open', 'reviewed'])
    expect(row('fresh').textContent).toContain('Needs you: never reviewed')
    expect(row('reviewed').textContent).not.toContain('Needs you')
    expect(root.querySelector('.local-more')!.textContent).toContain('Show 2 quiet branches, with no commit in 14 days')
    await act(async () => button('Show 2 quiet branches').click())
    expect(rowNames()).toEqual(['fresh', 'left-open', 'reviewed', 'old-1', 'old-2'])
  })

  it('adds a folder with the native chooser in the window, and nothing when it is cancelled', async () => {
    vi.mocked(api).mockResolvedValue(empty())
    const choose = vi.fn().mockResolvedValueOnce('/w/p').mockResolvedValueOnce('')
    window_(choose)
    vi.mocked(post).mockResolvedValue(empty({ added: ['/w/p'], projects: [project('/w/p/.git', 'p')] }))
    await act(async () => render(<LocalTab />, root))
    await settle()
    expect(root.textContent).toContain('No local projects yet')
    await act(async () => button('Add a folder').click())
    await settle()
    expect(choose).toHaveBeenCalledWith('Add a folder')
    expect(post).toHaveBeenCalledWith('/api/local/projects', { path: '/w/p' })
    expect(root.querySelector('[aria-label="p"]')).not.toBeNull()
    vi.mocked(post).mockClear()
    await act(async () => button('Add a folder to scan').click())
    await settle()
    expect(choose).toHaveBeenLastCalledWith('Add a folder to scan')
    expect(post).not.toHaveBeenCalled()
    expect(root.querySelector('input')).toBeNull()
  })

  it('takes a typed path in a browser, which has no chooser, and keeps the form when the path is refused', async () => {
    vi.mocked(api).mockResolvedValue(empty())
    vi.mocked(bindings).mockReturnValue(undefined)
    await act(async () => render(<LocalTab />, root))
    await settle()
    await act(async () => button('Add a folder to scan').click())
    const input = root.querySelector('input[aria-label="Add a folder to scan, as an absolute path"]') as HTMLInputElement
    expect(input).not.toBeNull()
    vi.mocked(post).mockRejectedValueOnce(new Error('local: relative is not an absolute path'))
    await act(async () => { input.value = 'relative'; input.dispatchEvent(new Event('input')) })
    await act(async () => button('Add').click())
    await settle()
    expect(root.querySelector('[role="alert"]')!.textContent).toContain('not an absolute path')
    expect(root.querySelector('form.local-path')).not.toBeNull()
    vi.mocked(post).mockResolvedValueOnce(empty({ roots: ['/w'] }))
    await act(async () => { input.value = '~/w'; input.dispatchEvent(new Event('input')) })
    await act(async () => button('Add').click())
    await settle()
    expect(post).toHaveBeenLastCalledWith('/api/local/roots', { path: '~/w' })
    expect(root.querySelector('form.local-path')).toBeNull()
    expect(root.textContent).toContain('No checkouts to show')
  })

  it('lists the folders given, each with a way back, and hides a project', async () => {
    const view = empty({ added: ['/w/p', '/w/gone'], roots: ['/w'], projects: [project('/w/p/.git', 'p')], hidden: [project('/w/q/.git', 'q')],
      unreadable: [{ path: '/w/gone', error: 'not a git checkout' }] })
    vi.mocked(api).mockResolvedValue(view)
    vi.mocked(post).mockResolvedValue(view)
    await act(async () => render(<LocalTab />, root))
    await settle()
    const folders = root.querySelector('.local-folders') as HTMLDetailsElement
    expect(folders.querySelector('summary')!.textContent).toBe('Folders · 2 added, 1 scanned, 1 hidden')
    // A folder that broke opens the list, so the person sees why.
    expect(folders.open).toBe(true)
    expect(folders.textContent).toContain('not a git checkout')
    const inFolders = (name: string, n = 0) => [...folders.querySelectorAll('button')].filter((b) => b.textContent === name)[n]
    await act(async () => inFolders('Remove', 1).click())
    expect(post).toHaveBeenLastCalledWith('/api/local/projects/remove', { path: '/w/gone' })
    await settle()
    await act(async () => inFolders('Remove', 2).click())
    expect(post).toHaveBeenLastCalledWith('/api/local/roots/remove', { path: '/w' })
    await settle()
    await act(async () => inFolders('Show').click())
    expect(post).toHaveBeenLastCalledWith('/api/local/hide', { key: '/w/q/.git', hidden: false })
    await settle()
    await act(async () => [...root.querySelector('section[aria-label="p"] h3')!.querySelectorAll('button')].find((b) => b.textContent === 'Hide')!.click())
    expect(post).toHaveBeenLastCalledWith('/api/local/hide', { key: '/w/p/.git', hidden: true })
  })
})

describe('a branch tied to its change', () => {
  it('names the change, opens its page, and says how the branch stands', async () => {
    const change = (standing: LocalChange['standing'], ahead = 0, behind = 0): LocalChange =>
      ({ ref: { host: 'gitlab.example', project: 'g/p', number: 2 }, kind: 'gitlab', title: 'Extend viewers', web_url: 'https://gitlab.example/g/p/-/merge_requests/2', head: 'abc', draft: false, standing, ahead, behind })
    expect(standing(change('in-step'))).toBe('in step')
    expect(standing(change('ahead', 2))).toBe('2 not pushed')
    expect(standing(change('behind', 0, 3))).toBe('3 behind')
    expect(standing(change('diverged', 1, 4))).toBe('1 not pushed, 4 behind')
    expect(standing(change('unfetched'))).toBe('has commits not fetched here')

    const view = empty({ added: ['/w/p'], projects: [{ ...project('/w/p/.git', 'p'), branches: [
      branch('in-step', { change: change('in-step') }),
      branch('quiet-tied', { quiet: true, change: change('in-step') }),
      branch('done'),
    ] }] })
    vi.mocked(api).mockResolvedValue(view)
    const open = vi.fn()
    await act(async () => render(<LocalTab onOpen={open} />, root))
    await settle()
    // A tied branch shows, even quiet and needing nothing, so its change can be opened.
    expect(rowNames()).toEqual(['in-step', 'quiet-tied', 'done'])
    expect(row('in-step').textContent).toContain('!2 open in step')
    await act(async () => button('!2 open').click())
    expect(open).toHaveBeenCalledWith('https://gitlab.example/g/p/-/merge_requests/2')
    // A tied row offers both reviews side by side; an untied one, local commits alone.
    const actions = (name: string) => [...row(name).querySelectorAll('.local-branch-actions button')].map((b) => b.textContent)
    expect(actions('in-step')).toEqual(['Review the change', 'Review local commits'])
    expect(actions('done')).toEqual(['Review local commits'])
    open.mockClear()
    await act(async () => (row('in-step').querySelector('.local-branch-actions button') as HTMLButtonElement).click())
    expect(open).toHaveBeenCalledWith('https://gitlab.example/g/p/-/merge_requests/2')
  })
})

describe('a branch row', () => {
  const setup = (over: Partial<Setup> = {}): Setup => ({
    editable: true, base: 'b', config_path: '', forges: [], tools: [{ tool: 'claude', version: '2', account: 'me' } as Setup['tools'][number]],
    presets: [{ name: 'quick', reviewers: [{ profile: 'code' }] }, { name: 'deep', reviewers: [{ profile: 'code' }, { profile: 'security' }] }],
    profiles: [], waived: [], background: { refresh_minutes: 15, notify: true, autostart: false, login: true, dock: false, dock_here: false, dock_restart: false }, scheme: '', tab: '', sources: [], ...over,
  })

  it('reviews local commits with the picked preset, opens the run, and says what is not reviewed', async () => {
    const p = { ...project('/w/p/.git', 'p'), worktrees: [{ path: '/w/p-feature', branch: 'feature', head: 'abc', dirty: true }],
      branches: [branch('feature', { worktrees: ['/w/p-feature'], run: { id: 'r0', status: 'closed', head: 'old', created: new Date().toISOString(), moved: true, undecided: 0 } })] }
    vi.mocked(api).mockResolvedValue(empty({ added: ['/w/p'], projects: [p] }))
    vi.mocked(post).mockResolvedValue({ run: { id: 'r1' }, uncommitted: ['/w/p-feature'] })
    const openRun = vi.fn()
    await act(async () => render(<LocalTab setup={setup()} onOpenRun={openRun} />, root))
    await settle()
    expect(row('feature').querySelector('.local-tag')!.textContent).toBe('p-feature · uncommitted')
    expect(row('feature').textContent).toContain('Uncommitted changes in p-feature are not in a review of local commits.')
    // The last review opens its run.
    await act(async () => button(`Reviewed just now`).click())
    expect(openRun).toHaveBeenLastCalledWith('r0')
    const select = root.querySelector('.local-actions select') as HTMLSelectElement
    await act(async () => { select.value = 'deep'; select.dispatchEvent(new Event('change')) })
    // A branch reviewed before is reviewed again, following that review (D50).
    expect(button('Review local commits').title).toContain('Follows the last review')
    await act(async () => button('Review local commits').click())
    await settle()
    expect(post).toHaveBeenCalledWith('/api/local/review', { key: '/w/p/.git', branch: 'feature', preset: 'deep' })
    expect(openRun).toHaveBeenLastCalledWith('r1')
  })

  it('cannot review local commits with no agent tool, and says why', async () => {
    vi.mocked(api).mockResolvedValue(empty({ added: ['/w/p'], projects: [{ ...project('/w/p/.git', 'p'), branches: [branch('feature')] }] }))
    await act(async () => render(<LocalTab setup={setup({ tools: [{ tool: 'claude', error: 'not signed in' } as Setup['tools'][number]] })} />, root))
    await settle()
    const review = button('Review local commits')
    expect(review.disabled).toBe(true)
    expect(review.title).toBe('claude: not signed in')
  })

  it('shows ten branches at a time', async () => {
    const many = Array.from({ length: 23 }, (_, i) => branch(`b${i}`))
    vi.mocked(api).mockResolvedValue(empty({ added: ['/w/p'], projects: [{ ...project('/w/p/.git', 'p'), branches: many }] }))
    await act(async () => render(<LocalTab />, root))
    await settle()
    expect(rowNames()).toHaveLength(10)
    await act(async () => button('Show 10 more of 13').click())
    expect(rowNames()).toHaveLength(20)
    await act(async () => button('Show 3 more of 3').click())
    expect(rowNames()).toHaveLength(23)
  })

  it('changes the base a project is compared with, and goes back to the default', async () => {
    const chosen = { ...project('/w/p/.git', 'p'), base: 'develop', base_chosen: true }
    vi.mocked(api).mockResolvedValue(empty({ added: ['/w/p'], projects: [{ ...project('/w/p/.git', 'p'), base: undefined, error: 'no origin/HEAD, main, or master to compare branches with; choose a base' }] }))
    vi.mocked(post).mockResolvedValueOnce(empty({ added: ['/w/p'], projects: [chosen] }))
    await act(async () => render(<LocalTab />, root))
    await settle()
    expect(root.textContent).toContain('choose a base')
    await act(async () => button('Change base').click())
    const input = root.querySelector('input[aria-label="The project\'s base branch"]') as HTMLInputElement
    await act(async () => { input.value = 'develop'; input.dispatchEvent(new Event('input')) })
    await act(async () => button('Save').click())
    await settle()
    expect(post).toHaveBeenLastCalledWith('/api/local/base', { key: '/w/p/.git', base: 'develop' })
    expect(root.textContent).toContain('against develop')
    expect(root.querySelector('input[aria-label="The project\'s base branch"]')).toBeNull()
    vi.mocked(post).mockResolvedValueOnce(empty({ added: ['/w/p'], projects: [project('/w/p/.git', 'p')] }))
    await act(async () => button('Change base').click())
    await act(async () => button('Use the default').click())
    await settle()
    expect(post).toHaveBeenLastCalledWith('/api/local/base', { key: '/w/p/.git', base: '' })
  })
})
