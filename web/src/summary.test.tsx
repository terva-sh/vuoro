// @vitest-environment jsdom
import { render } from 'preact'
import { act } from 'preact/test-utils'
import { afterEach, expect, it, vi } from 'vitest'
import { api, post } from './api'
import { SummaryPanel } from './summary'
import type { Change, Run, Setup } from './types'

vi.mock('./api', () => ({ api: vi.fn(), post: vi.fn() }))
const change = { kind: 'gitlab', ref: { host: 'lab.test', project: 'team/repo', number: 1 }, web_url: 'https://lab.test/team/repo/-/merge_requests/1', diff: { head: 'head', base: 'base' } } as Change
const setup = { tools: [{ name: 'fake', tool: 'Fake tool' }] } as Setup
const summary: Run = { title: 'Fixture', web_url: change.web_url, id: 'summary1', mode: 'summary', kind: change.kind, ref: change.ref, head: 'head', base: 'base', created: '', status: 'working', cost_usd: 0, summary: { scope: 'change', from: 'base', to: 'head' }, drafts: [], sessions: [{ id: 's1', role: 'summary', profile: 'summary', tool: 'fake', status: 'working', usage: { input: 0, output: 0, cache_read: 0, cache_write: 0, cost_usd: 0 }, log: [{ kind: 'assistant', text: 'Partial streamed summary', at: '' }] }] }
const reviewed = { ...summary, id: 'review1', mode: undefined, head: 'old-head', review_complete: true } as Run
let root: HTMLDivElement
async function mount(c = change, s = setup) { root = document.createElement('div'); await act(async () => render(<SummaryPanel change={c} setup={s} />, root)); await act(async () => {}); await act(async () => {}) }
async function click(text: string) { const b = [...root.querySelectorAll('button')].find((b) => b.textContent === text)!; expect(b).toBeDefined(); await act(async () => b.click()); await act(async () => {}) }
async function select(label: string, value: string) { const s = root.querySelector(`select[aria-label="${label}"]`)! as HTMLSelectElement; await act(async () => { s.value = value; s.dispatchEvent(new Event('change', { bubbles: true })) }) }
function serve(list = [summary, reviewed]) { vi.mocked(api).mockImplementation(async (path) => path === '/api/runs' ? list : summary) }
afterEach(() => { if (root) act(() => render(null, root)); vi.clearAllMocks(); vi.useRealTimers() })

it('shows partial output, exact range and explicit Stop without posting', async () => {
  serve(); await mount()
  expect(root.textContent).toContain('Partial streamed summary')
  expect(root.textContent).toContain('base..head')
  expect(post).not.toHaveBeenCalled()
  vi.mocked(post).mockResolvedValue({ ...summary, status: 'cancelled' })
  await click('■ Stop summary')
  expect(post).toHaveBeenCalledWith('/api/runs/summary1/sessions/s1/stop')
  expect(root.textContent).toContain('Partial output is retained')
  expect(root.textContent).toContain('Partial streamed summary')
  expect(root.querySelector('.summary-output')?.getAttribute('aria-busy')).toBe('false')
})
it('uses the selected completed review ID, not an inferred timestamp or supplied SHA', async () => {
  serve(); await mount(); await select('Summary scope', 'since_review')
  vi.mocked(post).mockResolvedValue({ ...summary, summary: { scope: 'since_review', from: 'old-head', to: 'head', review_id: 'review1' } })
  await click('Summarise')
  expect(post).toHaveBeenCalledWith('/api/summaries', { url: change.web_url, tool: 'fake', review_id: 'review1' })
  expect(root.textContent).toContain('old-head..head')
})
it('explains absent baseline evidence and reads history without an installed tool', async () => {
  serve([summary, { ...reviewed, review_complete: false }]); await mount(change, { ...setup, tools: [] })
  expect(root.textContent).toContain('Older reviews without a recorded completion')
  expect(root.textContent).toContain('Saved summaries are still readable')
  expect(root.querySelector('option[value="since_review"]')?.hasAttribute('disabled')).toBe(true)
  expect([...root.querySelectorAll('button')].find((b) => b.textContent === 'Summarise')?.disabled).toBe(true)
  expect(root.textContent).toContain('Partial streamed summary')
  expect(post).not.toHaveBeenCalled()
})
it('refuses a known empty incremental range', async () => {
  serve([summary, { ...reviewed, head: 'head' }]); await mount(); await select('Summary scope', 'since_review')
  expect(root.textContent).toContain('No new commits')
  expect([...root.querySelectorAll('button')].find((b) => b.textContent === 'Summarise')?.disabled).toBe(true)
})
it('ignores a poll started before Stop and aborts polling on unmount', async () => {
  vi.useFakeTimers(); serve(); await mount()
  let finish!: (r: Run) => void
  vi.mocked(api).mockImplementationOnce(() => new Promise((resolve) => { finish = resolve as typeof finish }))
  await act(async () => { await vi.advanceTimersByTimeAsync(1000) })
  vi.mocked(post).mockResolvedValue({ ...summary, status: 'cancelled' }); await click('■ Stop summary')
  await act(async () => finish(summary))
  expect(root.textContent).toContain('Partial output is retained')
  const signal = vi.mocked(api).mock.calls.find((c) => c[0] === '/api/runs/summary1')![1]?.signal
  act(() => render(null, root)); expect(signal?.aborted).toBe(true)
  const count = vi.mocked(api).mock.calls.length
  await vi.advanceTimersByTimeAsync(5000); expect(api).toHaveBeenCalledTimes(count)
})
it('isolates history across changes and ignores an old start response', async () => {
  serve(); await mount()
  let finish!: (r: Run) => void
  vi.mocked(post).mockImplementationOnce(() => new Promise((resolve) => { finish = resolve as typeof finish }))
  await click('Summarise')
  const other = { ...change, ref: { ...change.ref, number: 2 } }
  const otherRun = { ...summary, id: 'summary2', ref: other.ref, sessions: [] }
  vi.mocked(api).mockImplementation(async (path) => path === '/api/runs' ? [summary, otherRun] : otherRun)
  await act(async () => render(<SummaryPanel change={other} setup={setup} />, root)); await act(async () => {})
  await act(async () => finish(summary))
  expect(root.textContent).not.toContain('Partial streamed summary')
  expect((root.querySelector('select[aria-label="Summary history"]') as HTMLSelectElement)?.value).toBe('summary2')
})
