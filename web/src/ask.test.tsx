// @vitest-environment jsdom
import { render } from 'preact'
import { act } from 'preact/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { api, post } from './api'
import { AskPanel } from './ask'
import type { Change, Prompts, Run, Session, Setup } from './types'

vi.mock('./api', () => ({ api: vi.fn(), post: vi.fn() }))

afterEach(() => vi.resetAllMocks())

// jsdom scrolls nothing; a session pane scrolls to its last entry.
Element.prototype.scrollIntoView = () => {}

const change = { kind: 'gitlab', ref: { host: 'h', project: 'p', number: 7 }, web_url: 'https://h/p/-/merge_requests/7', state: 'open', diff: { head: 'head2222222', base: 'base000000' } } as unknown as Change
const setup = { tools: [{ name: 'claude', tool: 'Claude Code' }] } as unknown as Setup
const prompts: Prompts = {
  dir: '/cfg/prompts',
  prompts: [
    { name: 'deps', title: 'Dependency impact', text: 'What do the new versions change?' },
    { name: 'explain', title: 'Explain this change', text: 'Explain it.', builtin: true },
  ],
  errors: [{ file: 'broken.md', reason: 'scope is change or since_review, not "all"' }],
}
const session = (over: Partial<Session> = {}) => ({ id: 's1', profile: 'ask', tool: 'claude', role: 'summary', status: 'idle', usage: { cost_usd: 0.02, input: 1, output: 1, cache_read: 0, cache_write: 0 },
  log: [{ kind: 'change', text: 'the diff' }, { kind: 'assistant', text: 'It subtracts on purpose.' }], ...over }) as unknown as Session
const run = (over: Partial<Run> = {}): Run => ({
  id: 'ask-1', kind: 'gitlab', ref: change.ref, title: 't', web_url: change.web_url, head: 'head2222222', base: 'base000000', created: new Date().toISOString(),
  status: 'idle', cost_usd: 0.02, sessions: [session()], drafts: [], mode: 'ask', ask: { prompt: 'deps', title: 'Dependency impact', question: 'What do the new versions change?' },
  summary: { scope: 'change', from: 'base000000', to: 'head2222222' }, ...over,
}) as Run

function serve(list: Run[], selected?: Run) {
  vi.mocked(api).mockImplementation(async (url: string) => {
    if (url === '/api/prompts') return prompts
    if (url === '/api/changes/asks') return list
    if (url === '/api/runs') return []
    if (url.startsWith('/api/runs/') && url.endsWith('/head')) return { reviewed_head: 'head1111111', current_head: change.diff.head, relation: 'ahead', commits_behind: 1 }
    if (url.startsWith('/api/runs/')) return selected ?? list.find((r) => url.endsWith(r.id))
    throw new Error(`unexpected ${url}`)
  })
}

const button = (root: Element, text: string) => [...root.querySelectorAll('button')].find((b) => b.textContent?.startsWith(text))
const settle = async () => { for (let i = 0; i < 6; i++) await act(async () => { await Promise.resolve() }) }

describe('AskPanel', () => {
  it('offers the shipped prompts and the person\'s own, and names a file it could not use', async () => {
    serve([], run())
    vi.mocked(post).mockResolvedValue(run())
    const root = document.createElement('div')
    await act(async () => render(<AskPanel change={change} setup={setup} />, root))
    await settle()
    expect(button(root, 'Dependency impact')?.className).toBe('own')
    expect(button(root, 'Explain this change')?.className).toBe('')
    expect(root.textContent).toContain('1 prompt file could not be used')
    expect(root.textContent).toContain('broken.md')
    expect(root.textContent).toContain('/cfg/prompts')
    expect(post).not.toHaveBeenCalled()
    await act(async () => button(root, 'Dependency impact')!.click())
    expect(post).toHaveBeenCalledWith('/api/asks', expect.objectContaining({ url: change.web_url, prompt: 'deps', tool: 'claude' }))
    await settle()
    expect(root.textContent).toContain('It subtracts on purpose.')
  })

  it('asks the person\'s own question, and takes follow-ups in the conversation', async () => {
    const mine = run({ ask: { title: 'Why subtract?', question: 'Why subtract?' } })
    serve([], mine)
    vi.mocked(post).mockResolvedValue(mine)
    const root = document.createElement('div')
    document.body.appendChild(root) // a form submits only from the document
    await act(async () => render(<AskPanel change={change} setup={setup} />, root))
    await settle()
    const box = root.querySelector('textarea[aria-label="Your question"]') as HTMLTextAreaElement
    await act(async () => { box.value = 'Why subtract?'; box.dispatchEvent(new Event('input', { bubbles: true })) })
    await act(async () => button(root, 'Ask')!.click())
    expect(post).toHaveBeenCalledWith('/api/asks', expect.objectContaining({ question: 'Why subtract?', prompt: undefined }))
    serve([mine])
    await settle()
    expect(root.querySelector('textarea[aria-label="Ask a follow-up question"]')).not.toBeNull()
    root.remove()
  })

  it('marks a conversation on an older head as stale, and asks the same again on the new one', async () => {
    const old = run({ head: 'head1111111' })
    serve([old])
    vi.mocked(post).mockResolvedValue(run({ id: 'ask-2' }))
    const root = document.createElement('div')
    await act(async () => render(<AskPanel change={change} setup={setup} />, root))
    await settle()
    expect(root.querySelector('.ask-list')?.textContent).toContain('stale')
    expect(root.textContent).toContain('the change is now at head222')
    await act(async () => button(root, 'ask the same on the new head')!.click())
    expect(post).toHaveBeenCalledWith('/api/asks', expect.objectContaining({ prompt: 'deps' }))
  })

  it('follows a prompt\'s scope: whole change, or since a review only when there is one', async () => {
    const scoped: Prompts = { dir: '/cfg/prompts', errors: [], prompts: [
      { name: 'whole', title: 'Whole change', text: 'Q', scope: 'change' },
      { name: 'since', title: 'Since review', text: 'Q', scope: 'since_review' },
    ] }
    vi.mocked(api).mockImplementation(async (url: string) => {
      if (url === '/api/prompts') return scoped
      if (url === '/api/changes/asks') return []
      if (url === '/api/runs') return [] // no completed review of this change
      return run()
    })
    vi.mocked(post).mockResolvedValue(run())
    const root = document.createElement('div')
    await act(async () => render(<AskPanel change={change} setup={setup} />, root))
    await settle()
    expect(button(root, 'Since review')?.disabled).toBe(true)
    expect(button(root, 'Since review')?.title).toContain('since the chosen review')
    await act(async () => button(root, 'Whole change')!.click())
    expect(post).toHaveBeenCalledWith('/api/asks', expect.objectContaining({ prompt: 'whole', review_id: undefined }))
  })

  it('reads a summary from before Ask, with no follow-up, and asks it again as explain', async () => {
    const summary = run({ id: 'sum-1', mode: 'summary', head: 'head1111111', ask: { prompt: 'explain', title: 'Explain this change', question: '' }, sessions: [session({ profile: 'summary', status: 'ended' })] })
    serve([summary], { ...summary, ask: undefined }) // the single-run read names no prompt
    const root = document.createElement('div')
    await act(async () => render(<AskPanel change={change} setup={setup} />, root))
    await settle()
    expect(root.querySelector('.ask-list')?.textContent).toContain('Explain this change')
    expect(root.textContent).toContain('It subtracts on purpose.')
    expect(root.textContent).toContain('takes no follow-up')
    expect(root.querySelector('textarea[aria-label="Ask a follow-up question"]')).toBeNull()
    vi.mocked(post).mockResolvedValue(run({ id: 'ask-3' }))
    await act(async () => button(root, 'ask the same on the new head')!.click())
    expect(post).toHaveBeenCalledWith('/api/asks', expect.objectContaining({ prompt: 'explain', review_id: undefined }))
  })

  it('asks again over the conversation\'s own scope and review, not the tab\'s', async () => {
    const since = run({ head: 'head1111111', ask: { title: 'What changed?', question: 'What changed?' }, summary: { scope: 'since_review', from: 'rev0000000', to: 'head1111111', review_id: 'review-9' } })
    serve([since])
    vi.mocked(post).mockResolvedValue(run({ id: 'ask-4' }))
    const root = document.createElement('div')
    await act(async () => render(<AskPanel change={change} setup={setup} />, root))
    await settle()
    await act(async () => button(root, 'ask the same on the new head')!.click())
    expect(post).toHaveBeenCalledWith('/api/asks', expect.objectContaining({ question: 'What changed?', review_id: 'review-9' }))
  })

  it('sends nothing when the question\'s form is submitted past a disabled Ask', async () => {
    serve([])
    const root = document.createElement('div')
    document.body.appendChild(root)
    await act(async () => render(<AskPanel change={change} setup={{ tools: [] } as unknown as Setup} />, root))
    await settle()
    const box = root.querySelector('textarea[aria-label="Your question"]') as HTMLTextAreaElement
    await act(async () => { box.value = 'Why?'; box.dispatchEvent(new Event('input', { bubbles: true })) })
    // Cmd+Enter submits the form without its button.
    await act(async () => { box.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', metaKey: true, bubbles: true })) })
    expect(post).not.toHaveBeenCalled()
    root.remove()
  })
})
