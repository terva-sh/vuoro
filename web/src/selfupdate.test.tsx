// @vitest-environment jsdom
import { render } from 'preact'
import { act } from 'preact/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { api, post } from './api'
import { SelfUpdate } from './selfupdate'
import type { SelfUpdateState } from './types'

vi.mock('./api', () => ({ api: vi.fn(), post: vi.fn() }))

const root = document.createElement('div')
afterEach(() => {
  act(() => render(null, root))
  vi.resetAllMocks()
})

const shown = async (state: SelfUpdateState) => {
  vi.mocked(api).mockResolvedValue(state)
  await act(async () => render(<SelfUpdate />, root))
}

describe('the self-update notice', () => {
  it('says nothing when there is no newer release', async () => {
    await shown({ current: 'v0.1.1', release: true, latest: 'v0.1.1', newer: false })
    expect(root.textContent).toBe('')
    await shown({ current: 'dev', release: false, newer: false })
    expect(root.textContent).toBe('')
  })

  it('offers a newer release, and updates only when asked', async () => {
    await shown({ current: 'v0.1.1', release: true, latest: 'v0.2.0', newer: true })
    const button = root.querySelector('button')!
    expect(button.textContent).toBe('↑ Update to vuoro v0.2.0')
    expect(post).not.toHaveBeenCalled()
    vi.mocked(post).mockResolvedValue({ current: 'v0.1.1', release: true, latest: 'v0.2.0', newer: false, applied: 'v0.2.0' })
    await act(async () => button.click())
    expect(post).toHaveBeenCalledWith('/api/self-update')
    expect(root.textContent).toBe('✓ vuoro v0.2.0 is installed. Quit and reopen vuoro to use it.')
  })

  it('says why an update failed, and offers it again', async () => {
    await shown({ current: 'v0.1.1', release: true, latest: 'v0.2.0', newer: true })
    vi.mocked(post).mockRejectedValue(new Error('vuoro_0.2.0_darwin_arm64.tar.gz does not match checksums.txt'))
    await act(async () => root.querySelector('button')!.click())
    expect(root.querySelector('[role="alert"]')!.textContent).toContain('does not match checksums.txt')
    expect(root.querySelector('button')!.textContent).toBe('↑ Update to vuoro v0.2.0')
  })
})
