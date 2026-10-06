// @vitest-environment jsdom
import { render } from 'preact'
import { act } from 'preact/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { groupRuns, RunList } from './runs'
import type { Run } from './types'

const base: Run = {
  id: 'r1', kind: 'gitlab', ref: { host: 'gitlab.com', project: 'team/repo', number: 7 }, title: 'Forge change',
  web_url: 'https://gitlab.com/team/repo/-/merge_requests/7', head: 'abcdef0123', base: '0123abcdef',
  created: '2026-10-05T00:00:00Z', status: 'idle', cost_usd: 0.1, sessions: [], drafts: [],
}
const local: Run = { ...base, id: 'r2', kind: 'local', ref: { host: 'local', project: 'vuoro-1a2b3c4d', number: 0 }, title: 'Local commits', web_url: '' }
const root = document.createElement('div')
afterEach(() => act(() => render(null, root)))

describe('the run list', () => {
  it('opens a forge run, and lists a local one without a link', () => {
    const open = vi.fn()
    act(() => render(<RunList runs={[base, local]} onOpen={open} />, root))
    const buttons = [...root.querySelectorAll('button')]
    expect(buttons.map((b) => b.textContent)).toEqual(['Forge change'])
    expect(root.textContent).toContain('team/repo!7')
    expect(root.textContent).toContain('Local commits')
    expect(root.textContent).toContain('local commits of vuoro-1a2b3c4d')
    expect(root.textContent).not.toContain('!0')
    act(() => buttons[0].click())
    expect(open).toHaveBeenCalledWith(base.web_url)
  })
  it('shows nothing with no runs', () => {
    act(() => render(<RunList runs={[]} onOpen={() => {}} />, root))
    expect(root.textContent).toBe('')
  })
})

describe('the runs card', () => {
  const again = { ...base, id: 'r3', created: '2026-10-06T00:00:00Z', head: 'fedcba9876', status: 'closed' as const, cost_usd: 0.25 }
  const other = { ...base, id: 'r4', ref: { ...base.ref, number: 8 }, title: 'Other change', web_url: 'u8', created: '2026-10-04T00:00:00Z' }

  it('groups runs by change, newest first, and local runs by repository and title', () => {
    const groups = groupRuns([base, other, again, local, { ...local, id: 'r5' }])
    expect(groups.map((g) => g.runs.map((r) => r.id))).toEqual([['r3', 'r1'], ['r2', 'r5'], ['r4']])
  })

  it('starts folded, and its header says what it holds', () => {
    act(() => render(<RunList runs={[base, other, again]} onOpen={() => {}} />, root))
    const card = root.querySelector('details.runs-card') as HTMLDetailsElement
    expect(card.open).toBe(false)
    expect(card.querySelector('summary')!.textContent).toBe('Runs3across 2 changes · 2 not closed · $0.45')
  })

  it("shows one row per change, and a change's earlier runs on demand", () => {
    const opened: string[] = []
    act(() => render(<RunList runs={[base, again, local, { ...local, id: 'r5' }]} onOpen={() => {}} onOpenRun={(id) => opened.push(id)} />, root))
    expect(root.querySelectorAll('tr.run-group').length).toBe(2)
    expect(root.querySelectorAll('tr.run-earlier').length).toBe(0)
    const toggle = [...root.querySelectorAll('button')].find((b) => b.getAttribute('aria-expanded') === 'false')!
    act(() => toggle.click())
    expect(root.querySelectorAll('tr.run-earlier').length).toBe(2)
    expect(root.querySelector('tr.run-earlier')!.textContent).toContain('fedcba98')
    // A local run's earlier runs open by their own id.
    const localToggle = [...root.querySelectorAll('button')].find((b) => b.getAttribute('aria-expanded') === 'false')!
    act(() => localToggle.click())
    const earlierLocal = [...root.querySelectorAll('tr.run-earlier button')].at(-1) as HTMLButtonElement
    act(() => earlierLocal.click())
    expect(opened).toEqual(['r5'])
  })
})
