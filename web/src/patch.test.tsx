// @vitest-environment jsdom
import { render } from 'preact'
import { act } from 'preact/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { api } from './api'
import { LocalTab } from './local'
import { RunView } from './run'
import type { Draft, LocalView, Patch, PatchDetail, Run } from './types'

vi.mock('./api', () => ({ api: vi.fn(), post: vi.fn(), bindings: vi.fn() }))
const root = document.createElement('div')
document.body.append(root)
afterEach(() => { act(() => render(null, root)); vi.resetAllMocks() })
const settle = async () => { for (let i = 0; i < 5; i++) await act(async () => { await Promise.resolve() }) }
const button = (text: string, within: Element = root) => [...within.querySelectorAll('button')].find((b) => b.textContent === text)

const fix = 'f55c45591234f55c45591234f55c45591234f55c'
const big = 'b16b16b16b16b16b16b16b16b16b16b16b16b16b'
const patches: Patch[] = [
  { sha: fix, subject: 'Check the count', by: 'editor', at: '' },
  { sha: big, subject: 'Vendor the table', by: 'person', too_large: true, files: ['table.go', 'table_test.go'], at: '' },
]
const detail: PatchDetail = { ...patches[0], patch: 'From f55c\nSubject: [PATCH] Check the count\n', diff: [
  { old_path: 'calc.go', new_path: 'calc.go', patch: '@@ -1 +1,2 @@\n-a\n+b\n+c\n', new: false, deleted: false, renamed: false, omitted: false } as PatchDetail['diff'][number],
] }
const draft = (id: string, over: Partial<Draft>): Draft =>
  ({ id, severity: 'high', title: `Finding ${id}`, description: 'd', status: 'pending', revision: 1, session: 's1', drafted_by: 'Fake', updated: '', decision: 'kept', ...over }) as Draft
const run: Run = {
  id: 'r1', kind: 'gitlab', ref: { host: 'h', project: 'p', number: 7 }, title: 't', web_url: 'u', head: 'abcdef1234', base: 'b',
  created: '', status: 'idle', cost_usd: 0, sessions: [], patches,
  drafts: [draft('d1', { fixed_in: fix, fixed_subject: 'Check the count', fixed_by: 'editor' }), draft('d2', { fixed_in: big, fixed_subject: 'Vendor the table', fixed_by: 'person' })],
  waves: [{ session: 's2', branch: 'vuoro/fix-7', base: 'abcdef1234', started: '', pushed: { branch: 'vuoro/fix-7', sha: fix, target: 'feature', at: '' },
    commits: [{ sha: fix, subject: 'Check the count', findings: ['d1'] }] }],
}

describe('a saved fix', () => {
  it('shows its diff again from a fixed finding and from an earlier wave, and copies the patch', async () => {
    vi.mocked(api).mockResolvedValue(detail)
    const write = vi.fn(async () => {})
    Object.assign(navigator, { clipboard: { writeText: write } })
    act(() => render(<RunView run={run} url="u" onClose={() => {}} onPick={() => {}} onRun={() => {}} />, root))
    const group = root.querySelector('details.fixed-findings')!
    const fixed = [...group.querySelectorAll('.fix-link')]
    expect(fixed[1].textContent).toContain('its patch was too large to keep; it touched table.go, table_test.go')
    expect(button('Show the diff', fixed[1])).toBeUndefined()
    await act(async () => button('Show the diff', fixed[0])!.click())
    await settle()
    expect(api).toHaveBeenCalledWith(`/api/runs/r1/patches/${fix}`)
    const shown = fixed[0].parentElement!.querySelector('.patch-diff')!
    expect(shown.querySelector('.file-list .file-link')!.textContent).toBe('calc.go+2 −1')
    await act(async () => button('Copy the patch', shown)!.click())
    expect(write).toHaveBeenCalledWith(detail.patch)
    expect(button('Copied', shown)).toBeDefined()

    // The earlier wave's commit offers the same diff.
    const wave = root.querySelector('details.editor-waves')!
    expect(button('Show the diff', wave)).toBeDefined()
  })

  it('shows nothing for a commit the run did not save', () => {
    const r = { ...run, patches: [], drafts: [draft('d1', { fixed_in: fix, fixed_by: 'editor' })] }
    act(() => render(<RunView run={r} url="u" onClose={() => {}} onPick={() => {}} onRun={() => {}} />, root))
    expect(root.querySelector('.saved-patch')).toBeNull()
  })

  it('shows a Local fix branch\'s commits as its review saved them', async () => {
    const view: LocalView = {
      roots: [], added: ['/w/p'], scanned: '', built: new Date().toISOString(), hidden: [], unreadable: [], needs: 1, quiet_days: 14,
      projects: [{ key: '/w/p/.git', name: 'p', path: '/w/p', base: 'main', added: true, base_chosen: false, remotes: [], worktrees: [],
        branches: [{ name: 'feature', tip: 'abc', ahead: 1, committed: new Date().toISOString(), worktrees: [], quiet: false, needs: ['vuoro/fix-r1 waits to be merged'],
          fixes: [{ branch: 'vuoro/fix-r1', waiting: true, run: 'r1', patches: [patches[0]] }, { branch: 'vuoro/fix-r0', waiting: false, run: 'r0', patches: [] }] }] }],
    }
    vi.mocked(api).mockImplementation(async (path: string) => (path === '/api/local' ? view : detail) as never)
    await act(async () => render(<LocalTab />, root))
    await settle()
    const tags = [...root.querySelectorAll('.local-branch-facts .local-tag')]
    expect(button('Show the fix', tags[1])).toBeUndefined() // nothing saved
    await act(async () => button('Show the fix', tags[0])!.click())
    await settle()
    const modal = root.querySelector('.modal.fix-patches')!
    expect(modal.textContent).toContain('The fix on vuoro/fix-r1')
    expect(api).toHaveBeenCalledWith(`/api/runs/r1/patches/${fix}`)
    expect(modal.querySelector('.patch-diff .file-link')!.textContent).toBe('calc.go+2 −1')
  })
})
