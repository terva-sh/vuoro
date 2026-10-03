// @vitest-environment jsdom
import { render } from 'preact'
import { act } from 'preact/test-utils'
import { describe, expect, it, vi } from 'vitest'
import { ChangeView } from './change'
import { api, post } from './api'
import type { Change, Run, Setup } from './types'

vi.mock('./api', () => ({ api: vi.fn(async () => []), post: vi.fn() }))

const change: Change = {
  kind: 'forgejo', capabilities: { drafts: false, resolve: false, approve: false },
  ref: { host: 'forge.example', project: 'o/r', number: 2 },
  title: 'A PR', description: 'See #1 and !7.', author: { username: 'author', bot: false },
  web_url: 'https://forge.example/o/r/pulls/2', source_branch: 'feature', target_branch: 'main',
  diff: { head: 'head', base: 'base', start: 'base' }, state: 'open', draft: false, mergeability: '',
  created: '', updated: '', reviewers: [], assignees: [], participants: [], requests: [], reviews: [], pushes: [],
  files: [{ old_path: 'a.go', new_path: 'a.go', patch: '@@ -1 +1 @@\n-old\n+new\n', new: false, deleted: false, renamed: false, omitted: false, rebuilt: false }],
  threads: [{ id: '1', resolvable: true, resolved: false, anchor: { path: 'a.go', old_path: 'a.go', new_line: 1, old_line: 0, diff: { head: 'head', base: 'base', start: 'base' } },
    notes: [{ id: '1', author: { username: 'reviewer', bot: false }, body: 'Comment', at: '', system: false }] }], drafts: [],
}
const setup: Setup = {
  forges: [{ name: 'Forgejo', url: 'https://forge.example', username: 'me' }],
  tools: [{ name: 'fake', tool: 'Fake Tool' }], presets: [{ name: 'default', reviewers: [{ profile: 'code' }] }], profiles: [], waived: [],
}

describe('forge capabilities', () => {
  it('shows old findings separately from today\'s diff with their verified distance', async () => {
    const root = document.createElement('div')
    const r: Run = { id:'r1',kind:'gitlab',ref:change.ref,title:'A PR',web_url:change.web_url,head:'old',base:'base',created:'',status:'idle',cost_usd:0,sessions:[],drafts:[{id:'d1',severity:'low',title:'Old finding',description:'Saved wording',anchor:{path:'a.go',side:'head',start_line:1,end_line:1},status:'pending',revision:1,session:'s1',drafted_by:'Fake',updated:''}] }
    vi.mocked(api).mockImplementation(async (path) => path === '/api/runs' ? [r] : path.endsWith('/head') ? {reviewed_head:'old',current_head:'head',relation:'ahead',commits_behind:2} : r)
    await act(async () => render(<ChangeView change={{...change,kind:'gitlab',capabilities:{drafts:true,resolve:true,approve:true}}} setup={setup} onChange={() => {}} onSetup={() => {}} />,root))
    for (let i=0;i<5;i++) await act(async () => {})
    expect(root.textContent).toContain('reviewed at old, 2 commits behind')
    expect(root.querySelector('.files .draft')).toBeNull()
    expect(root.querySelector('section .draft')?.textContent).toContain('Saved wording')
    expect([...root.querySelectorAll('section .draft button')].find((b)=>b.textContent==='Keep')?.hasAttribute('disabled')).toBe(true)
    expect(post).not.toHaveBeenCalled()
    act(() => render(null,root))
    vi.mocked(api).mockResolvedValue([])
  })
  it('a configured Forgejo account can review but cannot post', async () => {
    const root = document.createElement('div')
    await act(async () => render(<ChangeView change={change} setup={setup} onChange={() => {}} onSetup={() => {}} />, root))
    expect(root.textContent).toContain('o/r#2')
    expect(root.textContent).toContain('Forgejo posting is not available yet')
    const buttons = [...root.querySelectorAll('button')].map((b) => b.textContent)
    expect(buttons).toContain('Start a review')
    expect(buttons).not.toContain('Submit')
    expect(buttons).not.toContain('Approve')
    expect(buttons).not.toContain('Reply')
    expect(buttons).not.toContain('Resolve')
    expect(root.querySelector('[aria-label="Comment on line 1"]')).toBeNull()
    act(() => render(null, root))
  })
  it('the same view preserves GitLab human-directed writes', async () => {
    const root = document.createElement('div')
    const gl = { ...change, kind: 'gitlab', capabilities: { drafts: true, resolve: true, approve: true }, web_url: 'https://forge.example/o/r/-/merge_requests/2' }
    await act(async () => render(<ChangeView change={gl} setup={setup} onChange={() => {}} onSetup={() => {}} />, root))
    const buttons = [...root.querySelectorAll('button')].map((b) => b.textContent)
    expect(buttons).toContain('Submit')
    expect(buttons).toContain('Approve')
    expect(buttons).toContain('Reply')
    expect(buttons).toContain('Resolve')
    expect(root.querySelector('[aria-label="Comment on line 1"]')).not.toBeNull()
    act(() => render(null, root))
  })
})
