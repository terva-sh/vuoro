// @vitest-environment jsdom
import { render } from 'preact'
import { act } from 'preact/test-utils'
import { describe, expect, it, vi } from 'vitest'
import { post } from './api'
import { applyScheme, SchemePicker } from './scheme'
import type { Scheme } from './types'

vi.mock('./api', () => ({ post: vi.fn() }))

describe('applyScheme', () => {
  it('names a pinned scheme on the root, and takes it off to follow the system', () => {
    const root = document.createElement('html')
    applyScheme('dark', root)
    expect(root.dataset.scheme).toBe('dark')
    applyScheme('', root)
    expect(root.hasAttribute('data-scheme')).toBe(false)
  })
})

describe('SchemePicker', () => {
  const pick = async (value: Scheme, saved: Promise<unknown>) => {
    vi.mocked(post).mockReset().mockReturnValue(saved)
    const picked: Scheme[] = []
    const root = document.createElement('div')
    act(() => render(<SchemePicker scheme="" onScheme={(s) => picked.push(s)} />, root))
    const select = root.querySelector('select')!
    expect([...select.options].map((o) => o.text)).toEqual(['System', 'Light', 'Dark'])
    await act(async () => {
      select.value = value
      select.dispatchEvent(new Event('change'))
      await saved.catch(() => undefined)
    })
    return picked
  }

  it('applies the choice at once and asks the server to keep it', async () => {
    expect(await pick('dark', Promise.resolve({}))).toEqual(['dark'])
    expect(post).toHaveBeenCalledWith('/api/prefs/scheme', { scheme: 'dark' })
  })

  it('puts the old scheme back when the server does not keep the new one', async () => {
    expect(await pick('light', Promise.reject(new Error('no')))).toEqual(['light', ''])
  })
})
