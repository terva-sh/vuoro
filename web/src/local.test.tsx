// @vitest-environment jsdom
import { render } from 'preact'
import { act } from 'preact/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { api, bindings, post, type Bindings } from './api'
import { LocalTab, standing } from './local'
import type { LocalBranch, LocalChange, LocalView } from './types'

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

const branch = (name: string, over: Partial<LocalBranch> = {}): LocalBranch => ({
  name, tip: 'abc', ahead: 1, committed: new Date().toISOString(), worktrees: [], fixes: [], needs: [], quiet: false, ...over,
})

describe('the Local tab', () => {
  it('lists what needs the person, and counts the rest apart from the quiet branches', async () => {
    const view: LocalView = {
      roots: [], added: ['/w/p'], scanned: '', built: new Date().toISOString(), hidden: [], unreadable: [], needs: 2, quiet_days: 14,
      projects: [{ key: 'k', name: 'p', path: '/w/p', base: 'main', added: true, base_chosen: false, remotes: [], worktrees: [], branches: [
        branch('fresh', { needs: ['never reviewed'] }),
        branch('left-open', { quiet: true, needs: ['1 finding undecided'] }),
        branch('reviewed'),
        branch('old-1', { quiet: true }),
        branch('old-2', { quiet: true }),
      ] }],
    }
    vi.mocked(api).mockResolvedValue(view)
    const onCount = vi.fn()
    await act(async () => render(<LocalTab onCount={onCount} />, root))
    for (let i = 0; i < 5; i++) await act(async () => { await Promise.resolve() })
    expect(onCount).toHaveBeenCalledWith(2)
    const text = root.textContent!
    expect(text).toContain('fresh')
    expect(text).toContain('1 finding undecided')
    expect(text).toContain('1 more branch needs nothing')
    expect(text).toContain('2 quiet branches, with no commit in 14 days')
    expect(text).not.toContain('old-1')
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
    await act(async () => (root.querySelector('[aria-label="p"] button') as HTMLButtonElement).click())
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
    const rows = [...root.querySelectorAll('.local-branches li')].map((li) => li.textContent)
    // A tied branch shows, even quiet and needing nothing, so its change can be opened.
    expect(rows[0]).toContain('!2 open')
    expect(rows[0]).toContain('in step')
    expect(rows[1]).toContain('quiet-tied')
    expect(rows[2]).toBe('1 more branch needs nothing')
    await act(async () => button('!2 open').click())
    expect(open).toHaveBeenCalledWith('https://gitlab.example/g/p/-/merge_requests/2')
  })
})
