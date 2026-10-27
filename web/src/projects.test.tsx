// @vitest-environment jsdom
import { render } from 'preact'
import { act } from 'preact/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { api, post } from './api'
import { ProjectsTab, type ProjectList, type ProjectRow, type ProjectView } from './projects'

vi.mock('./api', () => ({ api: vi.fn(), post: vi.fn() }))

const entry = { key: 'k1', kind: 'gitlab', host: 'lab.test', path: 'team/repo', name: 'Team / Repo', web_url: 'https://lab.test/team/repo' }
const list: ProjectList = { projects: [{ ...entry, open: 4 }], accounts: ['Work'] }
const row = (n: number, edit: Partial<ProjectRow>): ProjectRow => ({
  ref: { host: 'lab.test', project: 'team/repo', number: n }, kind: 'gitlab', title: `Change ${n}`,
  web_url: `https://lab.test/team/repo/-/merge_requests/${n}`, author: 'ann', draft: false, source_branch: `b${n}`, target_branch: 'main',
  opened: '2026-10-20T00:00:00Z', place: 'review', since: '2026-10-21T00:00:00Z', reasons: [], holder: [],
  reviewers: [], assignees: [], labels: [], threads: { total: 0, unresolved: 0, author: 0, reviewers: 0 }, ...edit,
})
const view: ProjectView = {
  project: entry, last_attempt: '2026-10-27T00:00:00Z', last_success: '2026-10-27T00:00:00Z', dependencies: false, open: 4,
  groups: [
    { place: 'blocked', rows: [row(3, { place: 'blocked', since: null, holder: [], target_branch: 'b2', stack: { root: 2, depth: 1, on: [2], above: [] }, reasons: [{ place: 'blocked', text: 'Stacked on !2, which is still open.', since: null }] })] },
    { place: 'review', rows: [row(2, { holder: ['bob'], reviewers: [{ username: 'bob', state: 'unreviewed' }], labels: ['backend'], stack: { root: 2, depth: 0, on: [], above: [3] }, pipeline: 'failed' })] },
    { place: 'unreviewed', rows: [row(1, { place: 'unreviewed', since: null, labels: ['docs'], author: 'cat' })] },
  ],
  unread: [row(9, { error: 'Reading this change timed out.' })],
}
const root = document.createElement('div')
const button = (name: string) => [...root.querySelectorAll('button')].find((b) => b.textContent?.trim().startsWith(name))!
const rows = () => [...root.querySelectorAll('.inbox-row')].map((b) => b.querySelector('.inbox-title')!.textContent)
async function mount() {
  vi.mocked(api).mockImplementation(async (path: string) => (path === '/api/projects' ? list : view) as never)
  const open = vi.fn()
  await act(async () => render(<ProjectsTab onOpen={open} />, root))
  for (let i = 0; i < 4; i++) await act(async () => { await Promise.resolve() })
  return open
}
afterEach(() => { act(() => render(null, root)); vi.resetAllMocks() })

describe('project triage', () => {
  it('groups changes by where they sit and explains the selected one', async () => {
    const open = await mount()
    expect(api).toHaveBeenCalledWith('/api/projects/k1', expect.anything())
    expect(api).toHaveBeenCalledWith('/api/projects/k1/refresh', expect.objectContaining({ method: 'POST' }))
    expect([...root.querySelectorAll('.inbox-lane h3')].map((h) => h.textContent)).toEqual(['Blocked 1', 'Waiting on review 1', 'No reviewers 1'])
    // An unknown age shows ?, never a colour alone, with when it was opened.
    const blocked = root.querySelector('[aria-label="Blocked"] .inbox-row')!
    expect(blocked.querySelector('.age')!.textContent).toBe('?')
    expect(blocked.textContent).toContain('opened')
    // The top of a stack says reviewing the change beneath unblocks it.
    const pane = root.querySelector('[aria-label="Selected change"]')!
    expect(pane.textContent).toContain('It waits only on !2 beneath it')
    expect([...pane.querySelectorAll('.project-stack li')].map((li) => li.textContent?.split(' · ')[0])).toEqual(['!2 Change 2', '!3 Change 3'])
    act(() => button('Open in vuoro').click())
    expect(open).toHaveBeenCalledWith(view.groups[0].rows[0].web_url)
    expect(root.textContent).toContain('stacks come from target branches alone')
    expect(root.textContent).toContain('Reading this change timed out.')
  })
  it('marks states with a symbol and a word', async () => {
    await mount()
    const review = root.querySelector('[aria-label="Waiting on review"] .inbox-row')!
    expect(review.textContent).toContain('● bob')
    expect(review.textContent).toContain('✗ pipeline failed')
    act(() => (review as HTMLButtonElement).click())
    expect(root.querySelector('[aria-label="Selected change"]')!.textContent).toContain('bob pending')
  })
  it('filters by place, label, author and stack', async () => {
    await mount()
    act(() => button('1 no reviewers').click())
    expect(rows()).toEqual(['Change 1'])
    act(() => button('Show all').click())
    const choose = (name: string, value: string) => act(() => {
      const s = root.querySelector(`select[aria-label="${name}"]`) as HTMLSelectElement
      s.value = value
      s.dispatchEvent(new Event('change'))
    })
    choose('Label', 'backend')
    expect(rows()).toEqual(['Change 2'])
    choose('Label', '')
    choose('Author', 'cat')
    expect(rows()).toEqual(['Change 1'])
    choose('Author', '')
    choose('Stack', '2')
    expect(rows()).toEqual(['Change 3', 'Change 2'])
  })
  it('adds a project from a search', async () => {
    vi.mocked(api).mockResolvedValue({ projects: [], accounts: ['Work'] } as never)
    await act(async () => render(<ProjectsTab onOpen={() => {}} />, root))
    await act(async () => { await Promise.resolve() })
    vi.mocked(post).mockResolvedValueOnce({ projects: [{ ...entry, kind: 'gitlab', account: 'Work', added: false }] } as never)
    await act(async () => { root.querySelector('.project-add form')!.dispatchEvent(new Event('submit', { cancelable: true })) })
    expect(post).toHaveBeenLastCalledWith('/api/projects/search', { query: '' })
    vi.mocked(api).mockResolvedValue(view as never)
    vi.mocked(post).mockResolvedValueOnce({ project: entry, list } as never)
    for (let i = 0; i < 3; i++) await act(async () => { await Promise.resolve() })
    await act(async () => (root.querySelector('.project-found button') as HTMLButtonElement).click())
    expect(post).toHaveBeenLastCalledWith('/api/projects/add', { host: 'lab.test', path: 'team/repo' })
    for (let i = 0; i < 4; i++) await act(async () => { await Promise.resolve() })
    expect(root.querySelector('.project-picker')?.textContent).toContain('repo')
  })
})
