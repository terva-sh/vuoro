// @vitest-environment jsdom
import { render } from 'preact'
import { act } from 'preact/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { api } from './api'
import { About, aboutText, type AboutInfo } from './about'
import type { Setup } from './types'

vi.mock('./api', () => ({ api: vi.fn(), post: vi.fn() }))
afterEach(() => vi.resetAllMocks())

const info: AboutInfo = {
  version: '0.2.0', commit: 'f0031cec3fc107525af7ac46', modified: true, go: 'go1.26.2', platform: 'linux/amd64',
  modules: [{ path: 'github.com/terva-sh/tuohi', version: 'v0.1.0' }, { path: 'github.com/godbus/dbus/v5', version: 'v5.2.3', replaced_by: 'example.com/fork/dbus' }],
  config_path: '/home/pat/.config/vuoro/config.json', state_dir: '/home/pat/.local/state/vuoro',
}
const setup = { tools: [{ name: 'claude', tool: 'claude', version: '2.1.300', state: 'ok' }, { name: 'terva', tool: 'terva', state: 'missing' }] } as unknown as Setup

describe('About', () => {
  it('lists the build, what it is built with, the agent tools, and the files', async () => {
    vi.mocked(api).mockResolvedValue(info)
    const root = document.createElement('div')
    act(() => render(<About setup={setup} onClose={() => {}} />, root))
    await act(async () => { await new Promise((r) => setTimeout(r, 0)) })
    expect(api).toHaveBeenCalledWith('/api/about')
    const text = root.textContent!
    for (const want of ['0.2.0', 'f0031cec3fc1 (with uncommitted changes)', 'linux/amd64', 'github.com/terva-sh/tuohi', 'v0.1.0', 'Claude Code', '2.1.300', '/home/pat/.config/vuoro/config.json']) {
      expect(text).toContain(want)
    }
    act(() => render(null, root))
  })

  it('copies, and shows the text to copy by hand when the clipboard is missing or refuses', async () => {
    vi.mocked(api).mockResolvedValue(info)
    const root = document.createElement('div')
    act(() => render(<About setup={setup} onClose={() => {}} />, root))
    await act(async () => { await new Promise((r) => setTimeout(r, 0)) })
    const press = async () => {
      const b = [...root.querySelectorAll('button')].find((x) => x.textContent!.startsWith('Cop'))!
      await act(async () => { b.click(); await new Promise((r) => setTimeout(r, 0)) })
    }
    const writeText = vi.fn().mockResolvedValue(undefined)
    Object.defineProperty(navigator, 'clipboard', { value: { writeText }, configurable: true })
    await press()
    expect(writeText).toHaveBeenCalledWith(aboutText(info, setup))
    expect(root.textContent).toContain('Copied')
    expect(root.querySelector('textarea')).toBeNull()
    writeText.mockRejectedValueOnce(new Error('denied'))
    await press()
    expect(root.querySelector('textarea')!.value).toBe(aboutText(info, setup))
    Object.defineProperty(navigator, 'clipboard', { value: undefined, configurable: true })
    await press()
    expect(root.textContent).toContain('select the text and copy it')
    act(() => render(null, root))
  })

  it('copies the same facts as text for a bug report', () => {
    const text = aboutText(info, setup)
    expect(text).toContain('vuoro\n  Version: 0.2.0\n  Commit: f0031cec3fc1 (with uncommitted changes)')
    expect(text).toContain('  terva: unknown version, missing')
    expect(text).toContain('  github.com/godbus/dbus/v5: v5.2.3, replaced by example.com/fork/dbus')
  })
})
