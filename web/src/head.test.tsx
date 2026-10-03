// @vitest-environment jsdom
import { render } from 'preact'
import { act } from 'preact/test-utils'
import { afterEach, expect, it, vi } from 'vitest'
import { api, post } from './api'
import { HeadBadge, useRunHead, type HeadState } from './head'
import type { Change } from './types'

vi.mock('./api', () => ({ api: vi.fn(), post: vi.fn() }))
let root: HTMLDivElement
const change = (head: string) => ({ diff: { head }, web_url: 'https://lab.test/o/r/-/merge_requests/1' }) as Change
function Harness({ head }: { head: string }) { return <HeadBadge state={useRunHead('r1', 'old', change(head))} /> }
afterEach(() => { if (root) act(() => render(null,root)); vi.clearAllMocks() })

it('binds the count to its exact head pair and aborts old comparisons', async () => {
  let finishOld!: (s: HeadState) => void
  vi.mocked(api).mockImplementationOnce(() => new Promise((resolve) => { finishOld = resolve as typeof finishOld }))
  root = document.createElement('div')
  await act(async () => render(<Harness head="first" />,root)); await act(async () => {})
  const signal = vi.mocked(api).mock.calls[0][1]?.signal
  vi.mocked(api).mockResolvedValue({ reviewed_head: 'old', current_head: 'second', relation: 'rewritten' })
  await act(async () => render(<Harness head="second" />,root)); await act(async () => {})
  expect(signal?.aborted).toBe(true)
  await act(async () => { finishOld({ reviewed_head:'old',current_head:'first',relation:'ahead',commits_behind:2 }) })
  expect(root.textContent).toContain('history rewritten')
  expect(root.textContent).not.toContain('2 commits behind')
  expect(post).not.toHaveBeenCalled()
})
it('shows an unavailable count on read failure and preserves the reviewed SHA', async () => {
  vi.mocked(api).mockRejectedValue(new Error('offline'))
  root = document.createElement('div')
  await act(async () => render(<Harness head="new" />,root)); await act(async () => {})
  expect(root.textContent).toContain('reviewed at old, head changed · commit distance unavailable')
  expect(post).not.toHaveBeenCalled()
})

it('finishes a comparison when the forge has advanced past the workspace snapshot', async () => {
  vi.mocked(api).mockResolvedValue({ reviewed_head: 'old', current_head: 'newest', relation: 'ahead', commits_behind: 3 })
  root = document.createElement('div')
  await act(async () => render(<Harness head="new" />, root)); await act(async () => {})
  expect(root.textContent).toContain('refresh to compare')
  expect(root.textContent).not.toContain('checking')
  expect(root.textContent).not.toContain('3 commits behind')
  expect(post).not.toHaveBeenCalled()
})
