// @vitest-environment jsdom
import { render } from 'preact'
import { act } from 'preact/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { api } from './api'
import { LocalTab } from './local'
import type { LocalBranch, LocalView } from './types'

vi.mock('./api', () => ({ api: vi.fn(), post: vi.fn() }))
const root = document.createElement('div')
afterEach(() => { act(() => render(null, root)); vi.resetAllMocks() })

const branch = (name: string, over: Partial<LocalBranch> = {}): LocalBranch => ({
  name, tip: 'abc', ahead: 1, committed: new Date().toISOString(), worktrees: [], fixes: [], needs: [], quiet: false, ...over,
})

describe('the Local tab', () => {
  it('lists what needs the person, and counts the rest apart from the quiet branches', async () => {
    const view: LocalView = {
      roots: [], added: ['/w/p'], scanned: '', built: new Date().toISOString(), hidden: [], unreadable: [], needs: 2, quiet_days: 14,
      projects: [{ key: 'k', name: 'p', path: '/w/p', base: 'main', added: true, base_chosen: false, worktrees: [], branches: [
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
})
