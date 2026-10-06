// @vitest-environment jsdom
import { render } from 'preact'
import { act } from 'preact/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { post } from './api'
import { FirstRun, ForgeEditor, Forges, hostOf, joinCommand, splitCommand, Tools } from './setup'
import type { Setup, Tool } from './types'

vi.mock('./api', () => ({ api: vi.fn(), post: vi.fn() }))
afterEach(() => vi.resetAllMocks())

const setup = (over: Partial<Setup> = {}): Setup => ({
  editable: true, base: 'b1', config_path: '/home/pat/.config/vuoro/config.json', forges: [], tools: [], presets: [], profiles: [], waived: [], scheme: '', tab: '',
  sources: [
    { id: 'secret-service', label: 'Secret Service', read: ['secret-tool', 'lookup', 'service', 'vuoro', 'host', '{host}'], store: ['secret-tool', 'store', '--label=vuoro {host}', 'service', 'vuoro', 'host', '{host}'] },
    { id: 'env', label: 'An environment variable' },
    { id: 'command', label: 'A command of your own' },
  ],
  ...over,
})

const input = (root: Element, label: string) =>
  [...root.querySelectorAll('label')].find((l) => l.textContent!.startsWith(label))!.querySelector('input, select') as HTMLInputElement
const type = (el: HTMLInputElement, value: string) => act(() => { el.value = value; el.dispatchEvent(new Event(el.tagName === 'SELECT' ? 'change' : 'input')) })
const button = (root: Element, text: string) => [...root.querySelectorAll('button')].find((b) => b.textContent === text)!
// A click whose request settles: the shared action takes a few microtasks
// between the answer and the state it sets.
const click = (root: Element, text: string) => act(async () => { button(root, text).click(); for (let i = 0; i < 5; i++) await Promise.resolve() })

describe('splitCommand', () => {
  it('splits as a shell would, and expands nothing', () => {
    expect(splitCommand(`op read "op://Private/vuoro forge.example/credential"`)).toEqual(['op', 'read', 'op://Private/vuoro forge.example/credential'])
    expect(splitCommand(`~/bin/get 'a b' c\\ d "e\\"f" $HOME ''`)).toEqual(['~/bin/get', 'a b', 'c d', 'e"f', '$HOME', ''])
    expect(() => splitCommand(`pass show 'vuoro`)).toThrow('single quote')
  })
  it('round-trips through joinCommand', () => {
    const argv = ['security', 'find-generic-password', '-a', "pat's forge", '-w', '']
    expect(splitCommand(joinCommand(argv))).toEqual(argv)
  })
})

describe('hostOf', () => {
  it('takes a plain host name and port, and nothing a shell could read', () => {
    expect(hostOf('https://git.example.com:2222/')).toBe('git.example.com:2222')
    expect(hostOf('https://gitlab.com')).toBe('gitlab.com')
    for (const bad of ['http://gitlab.com', 'https://$(id).example', 'https://a`b`.example', 'https://user@host.example', 'https://host.example/path', 'not a url']) {
      expect(hostOf(bad), bad).toBe('')
    }
  })
})

describe('ForgeEditor', () => {
  it('tests the source, fills the username, and saves the source but never a token', async () => {
    vi.mocked(post).mockResolvedValueOnce({ username: 'pat', matches: true }).mockResolvedValueOnce(setup())
    const saved = vi.fn()
    const root = document.createElement('div')
    document.body.append(root) // a form submits only from the document
    act(() => render(<ForgeEditor setup={setup()} onSaved={saved} />, root))
    expect(root.querySelector('input[type="password"]')).toBeNull()
    expect(button(root, 'Add this forge').disabled).toBe(true)
    type(input(root, 'Address'), 'https://forge.example')
    // The ready-made source fills its command in for this host, and says how
    // to store the token.
    expect(input(root, 'vuoro reads it with').value).toBe('secret-tool lookup service vuoro host forge.example')
    expect(root.textContent).toContain("secret-tool store '--label=vuoro forge.example' service vuoro host forge.example")
    expect(root.querySelector('a')!.getAttribute('href')).toBe('https://forge.example/user/settings/applications')
    await click(root, 'Test')
    const form = { name: 'forge.example', kind: 'forgejo', url: 'https://forge.example', username: '', clone: 'ssh', token_command: ['secret-tool', 'lookup', 'service', 'vuoro', 'host', 'forge.example'] }
    expect(post).toHaveBeenCalledWith('/api/setup/test', form)
    expect(root.textContent).toContain('✓ Signed in as @pat')
    expect(input(root, 'Your username').value).toBe('pat')
    await click(root, 'Add this forge')
    expect(post).toHaveBeenLastCalledWith('/api/setup/forges', { base: 'b1', original: '', forge: { ...form, username: 'pat' } })
    expect(saved).toHaveBeenCalled()
    root.remove()
  })

  it('asks for a new test when the source changes, and names a token that is someone else\'s', async () => {
    vi.mocked(post).mockResolvedValue({ username: 'robot', matches: false })
    const root = document.createElement('div')
    act(() => render(<ForgeEditor setup={setup()} onSaved={() => {}} />, root))
    type(input(root, 'Address'), 'https://forge.example')
    type(input(root, 'Your username'), 'pat')
    await click(root, 'Test')
    expect(root.textContent).toContain("The token is @robot's, not @pat's.")
    expect(button(root, 'Add this forge').disabled).toBe(true)
    type(input(root, 'Keep it in'), 'env')
    expect(root.textContent).not.toContain('Signed in')
    expect(input(root, 'Variable').placeholder).toBe('VUORO_FORGE_EXAMPLE_TOKEN')
  })
})

describe('an editor opened before the file changed', () => {
  it('saves against the file it was opened from, so the save is refused', async () => {
    vi.mocked(post).mockResolvedValueOnce({ username: 'pat', matches: true }).mockRejectedValueOnce(new Error('config: the file changed since vuoro read it'))
    const root = document.createElement('div')
    document.body.append(root)
    const forge = { name: 'internal', kind: 'forgejo', url: 'https://forge.example', username: 'pat', token_env: 'T' }
    act(() => render(<ForgeEditor setup={setup({ base: 'old' })} initial={forge} original="internal" onSaved={() => {}} />, root))
    // The window reads the file again after a hand edit, and the open editor
    // gets the new snapshot.
    act(() => render(<ForgeEditor setup={setup({ base: 'new' })} initial={forge} original="internal" onSaved={() => {}} />, root))
    await click(root, 'Test')
    await click(root, 'Save')
    expect(post).toHaveBeenLastCalledWith('/api/setup/forges', expect.objectContaining({ base: 'old' }))
    expect(root.textContent).toContain('the file changed')
    root.remove()
  })
})

describe('Tools', () => {
  it('says what to do about each tool, and checks again', async () => {
    const tools: Tool[] = [
      { name: 'claude', tool: 'claude', version: '2.1.300', state: 'signed_out' },
    ]
    vi.mocked(post).mockResolvedValue(setup({ tools: [{ name: 'claude', tool: 'claude', version: '2.1.300', account: 'claude.ai max', state: 'ok' }] }))
    const onSetup = vi.fn()
    const root = document.createElement('div')
    act(() => render(<Tools setup={setup({ tools })} onSetup={onSetup} />, root))
    expect(root.textContent).toContain('sign in with /login')
    await click(root, 'Check again')
    expect(post).toHaveBeenCalledWith('/api/setup/tools', {})
    expect(onSetup).toHaveBeenCalled()
  })
})

describe('Tools when the check fails', () => {
  it('says so, rather than showing the old results as the new ones', async () => {
    vi.mocked(post).mockRejectedValue(new Error('forbidden'))
    const onSetup = vi.fn()
    const root = document.createElement('div')
    act(() => render(<Tools setup={setup({ tools: [{ name: 'claude', tool: 'claude', version: '2.1.300', state: 'ok', account: 'max' }] })} onSetup={onSetup} />, root))
    await click(root, 'Check again')
    expect(onSetup).not.toHaveBeenCalled()
    expect(root.querySelector('[role="alert"]')!.textContent).toContain('The check did not run')
    expect(button(root, 'Check again').disabled).toBe(false)
  })
})

describe('FirstRun', () => {
  it('starts with the forge form, and can be skipped', () => {
    const done = vi.fn()
    const root = document.createElement('div')
    act(() => render(<FirstRun setup={setup()} onSetup={() => {}} onDone={done} />, root))
    expect(root.textContent).toContain('Set up vuoro')
    expect(root.querySelector('.forge-editor')).not.toBeNull()
    act(() => button(root, 'Skip for now').click())
    expect(done).toHaveBeenCalled()
  })
})

describe('the accounts panel', () => {
  it('reads setup again each time it opens, to pick up a hand edit', async () => {
    const { api } = await import('./api')
    const { Home } = await import('./app')
    const fresh = setup({ base: 'b2', forges: [{ name: 'edited by hand', kind: 'gitlab', url: 'https://gitlab.com', username: 'pat', token_env: 'T' }] })
    vi.mocked(api).mockImplementation(async (path: string) => (path === '/api/setup' ? fresh : { accounts: [], mine: [], theirs: [], snoozed: [], unread: [], dormant: [] }) as never)
    const onSetup = vi.fn()
    const root = document.createElement('div')
    const existing = setup({ forges: [{ name: 'internal', kind: 'forgejo', url: 'https://forge.example', username: 'pat', token_env: 'T' }] })
    await act(async () => render(<Home setup={existing} onSetup={onSetup} runs={[]} onOpen={() => {}} onOpenRun={() => {}} opening={false} />, root))
    const panel = root.querySelector('details.inbox-setup') as HTMLDetailsElement
    // The browser fires toggle itself, after open changes.
    await act(async () => { panel.open = true; await new Promise((r) => setTimeout(r, 0)) })
    expect(api).toHaveBeenCalledWith('/api/setup')
    expect(onSetup).toHaveBeenCalledWith(fresh)
    act(() => render(null, root))
  })
})

describe('Forges', () => {
  it('holds Remove and Keep while a removal runs', async () => {
    let finish!: (s: Setup) => void
    vi.mocked(post).mockImplementation(() => new Promise((r) => { finish = r as typeof finish }))
    const onSetup = vi.fn()
    const root = document.createElement('div')
    const forge = { name: 'Work', kind: 'gitlab', url: 'https://lab.example', username: 'pat', clone: 'ssh', token_env: 'VUORO_WORK_TOKEN', env_set: true }
    act(() => render(<Forges setup={setup({ forges: [forge] as unknown as Setup['forges'] })} onSetup={onSetup} />, root))
    act(() => button(root, 'Remove').click())
    await click(root, 'Remove')
    expect(button(root, 'Removing…').disabled).toBe(true)
    expect(button(root, 'Keep').disabled).toBe(true)
    expect(post).toHaveBeenCalledTimes(1)
    await act(async () => { finish(setup()); for (let i = 0; i < 5; i++) await Promise.resolve() })
    expect(onSetup).toHaveBeenCalledTimes(1)
  })
})
