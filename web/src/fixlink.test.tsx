// @vitest-environment jsdom
import { render } from 'preact'
import { act } from 'preact/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { post } from './api'
import { FixLink } from './findings'
import type { Draft, Run } from './types'

vi.mock('./api', () => ({ api: vi.fn(), post: vi.fn() }))
const root = document.createElement('div')
document.body.append(root)
afterEach(() => { act(() => render(null, root)); vi.resetAllMocks() })
const settle = async () => { for (let i = 0; i < 5; i++) await act(async () => { await Promise.resolve() }) }
const button = (name: string) => [...root.querySelectorAll('button')].find((b) => b.textContent === name)!
const draft = (over: Partial<Draft> = {}) =>
  ({ id: 'd9', severity: 'medium', title: 'README says to ignore fw-compute', description: 'd', status: 'pending', revision: 1, session: 's1', updated: '', ...over }) as unknown as Draft

describe('FixLink', () => {
  it("shows the commit an editor's trailer names, which the person cannot take back", () => {
    act(() => render(<FixLink draft={draft({ fixed_in: 'f55c45591234', fixed_subject: 'Describe the selector in the NGC README', fixed_by: 'editor' })} runID="r1" url="u" onRun={() => {}} />, root))
    expect(root.textContent).toBe("✓ Fixed in f55c4559 · Describe the selector in the NGC README · named in the commit's trailer")
    expect(root.querySelector('button')).toBeNull()
  })

  it('links a fix by hand from the review commits, and takes back only that link', async () => {
    const onRun = vi.fn()
    vi.mocked(post).mockImplementation(async (path: string) => {
      if (path.endsWith('/fixes')) return [{ sha: 'aaaa1111bbbb', subject: 'Earlier' }, { sha: '1e5a6a531234', subject: 'Warn in variable descriptions' }]
      return { id: 'r1' } as Run
    })
    act(() => render(<FixLink draft={draft()} runID="r1" url="u" onRun={onRun} />, root))
    await act(async () => button('Mark fixed…').click())
    await settle()
    expect(post).toHaveBeenCalledWith('/api/runs/r1/fixes', { url: 'u' })
    // The newest commit is chosen first.
    expect((root.querySelector('select') as HTMLSelectElement).value).toBe('1e5a6a531234')
    await act(async () => button('Mark fixed').click())
    await settle()
    expect(post).toHaveBeenLastCalledWith('/api/runs/r1/drafts/d9/fixed', { url: 'u', sha: '1e5a6a531234' })
    expect(onRun).toHaveBeenCalled()

    act(() => render(<FixLink draft={draft({ fixed_in: '1e5a6a531234', fixed_subject: 'Warn', fixed_by: 'person' })} runID="r1" url="u" onRun={onRun} />, root))
    expect(root.textContent).toContain('linked by you')
    await act(async () => button('Not fixed').click())
    expect(post).toHaveBeenLastCalledWith('/api/runs/r1/drafts/d9/fixed', { url: 'u', sha: '' })
  })
})
