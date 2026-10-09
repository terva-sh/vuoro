// @vitest-environment jsdom
import { render } from 'preact'
import { act } from 'preact/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { post } from './api'
import { Background, FirstRun, ForgeEditor, Forges, hostOf, joinCommand, signedIn, splitCommand, Tools } from './setup'
import type { Setup, Tool } from './types'

vi.mock('./api', () => ({ api: vi.fn(), post: vi.fn() }))
afterEach(() => vi.resetAllMocks())

const setup = (over: Partial<Setup> = {}): Setup => ({
  editable: true, base: 'b1', config_path: '/home/pat/.config/vuoro/config.json', forges: [], tools: [], presets: [], profiles: [], waived: [], background: { refresh_minutes: 15, notify: true, autostart: false, login: true, dock: false, dock_here: false, dock_restart: false }, scheme: '', tab: '',
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
    await click(root, 'Check the token')
    const form = { name: 'forge.example', kind: 'forgejo', url: 'https://forge.example', username: '', clone: 'ssh', token_command: ['secret-tool', 'lookup', 'service', 'vuoro', 'host', 'forge.example'] }
    expect(post).toHaveBeenCalledWith('/api/setup/test', form)
    expect(root.textContent).toContain('✓ Signed in to forge.example as @pat')
    expect(input(root, 'Your username').value).toBe('pat')
    await click(root, 'Add this forge')
    expect(post).toHaveBeenLastCalledWith('/api/setup/forges', { base: 'b1', original: '', forge: { ...form, username: 'pat' } })
    expect(saved).toHaveBeenCalled()
    root.remove()
  })

  it('asks for a new test when the source changes, and names a token that is someone else\'s', async () => {
    vi.mocked(post).mockResolvedValue({ username: 'robot', matches: false })
    const root = document.createElement('div')
    document.body.append(root) // a radio fires change only in the document
    act(() => render(<ForgeEditor setup={setup()} onSaved={() => {}} />, root))
    type(input(root, 'Address'), 'https://forge.example')
    type(input(root, 'Your username'), 'pat')
    await click(root, 'Check the token')
    expect(root.textContent).toContain("The token is @robot's, not @pat's.")
    expect(button(root, 'Add this forge').disabled).toBe(true)
    act(() => (root.querySelector('input[type="radio"][value="env"]') as HTMLInputElement).click())
    expect(root.textContent).not.toContain('Signed in')
    expect(input(root, 'Variable').placeholder).toBe('VUORO_FORGE_EXAMPLE_TOKEN')
    root.remove()
  })
})

describe('the access gauge', () => {
  const check = async (access: object) => {
    vi.mocked(post).mockResolvedValueOnce({ username: 'pat', matches: true, access })
    const root = document.createElement('div')
    document.body.append(root)
    act(() => render(<ForgeEditor setup={setup()} onSaved={() => {}} />, root))
    type(input(root, 'Address'), 'https://forge.example')
    await click(root, 'Check the token')
    return root
  }

  it('refuses a token that cannot read reviews, and says what it needs', async () => {
    const root = await check({ level: 'none', missing: ['read:repository', 'read:issue'] })
    expect(root.querySelector('.gauge.none')).not.toBeNull()
    expect(root.textContent).toContain('This token cannot read your reviews. Make one with read:repository, read:issue.')
    expect(button(root, 'Add this forge').disabled).toBe(true)
    root.remove()
  })

  it('names what to leave out of a token that is broad and still cannot read', async () => {
    const root = await check({ level: 'none', missing: ['read:repository'], broad: ['write:admin'] })
    expect(root.textContent).toContain('Make one with read:repository. Leave out write:admin, which vuoro does not need.')
    expect(button(root, 'Add this forge').disabled).toBe(true)
    root.remove()
  })

  it('saves a token broader than vuoro needs, and warns about what it holds', async () => {
    const root = await check({ level: 'broad', scopes: ['all'], broad: ['all'] })
    expect(root.querySelector('.gauge.broad .zones [aria-current]')!.textContent).toBe('Too broad')
    expect(root.textContent).toContain('Broader than vuoro needs. It also holds all.')
    expect(button(root, 'Add this forge').disabled).toBe(false)
    root.remove()
  })

  it('says when the forge cannot tell, without a reading', async () => {
    const root = await check({ level: 'unknown', note: 'GitHub does not tell vuoro what a fine-grained token may do.' })
    expect(root.querySelector('.gauge.unknown .needle')).toBeNull()
    expect(root.textContent).toContain('fine-grained')
    expect(button(root, 'Add this forge').disabled).toBe(false)
    root.remove()
  })
})

describe('a GitHub forge', () => {
  it('links to the token page with the repo scope', () => {
    const root = document.createElement('div')
    act(() => render(<ForgeEditor setup={setup()} onSaved={() => {}} />, root))
    type(input(root, 'Forge'), 'github')
    expect(input(root, 'Address').placeholder).toBe('https://github.com')
    type(input(root, 'Address'), 'https://github.com')
    expect(root.querySelector('a')!.getAttribute('href')).toBe('https://github.com/settings/tokens/new?description=vuoro&scopes=repo')
    expect(root.textContent).toContain('the repo scope')
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
    await click(root, 'Check the token')
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
  it('walks from a forge to its token, the agent tools, and the background, one step at a time', async () => {
    const added = setup({ base: 'b2', forges: [{ name: 'github.com', kind: 'github', url: 'https://github.com', username: 'pat', clone: 'ssh', token_command: ['secret-tool', 'lookup', 'service', 'vuoro', 'host', 'github.com'] }] })
    vi.mocked(post).mockResolvedValueOnce({ username: 'pat', matches: true }).mockResolvedValueOnce(added)
    let current = setup()
    const done = vi.fn()
    const root = document.createElement('div')
    document.body.append(root)
    const draw = () => act(() => render(<FirstRun setup={current} onSetup={(s) => { current = s; draw() }} onDone={done} />, root))
    draw()
    const step = () => root.querySelector('[aria-current="step"]')!.textContent
    expect(step()).toContain('Forge')
    expect(button(root, 'Next').disabled).toBe(true)
    act(() => { [...root.querySelectorAll('button.preset')].find((b) => b.textContent!.startsWith('GitHub'))!.dispatchEvent(new MouseEvent('click', { bubbles: true })) })
    act(() => button(root, 'Next').click())
    expect(step()).toContain('Token')
    expect(root.querySelector('.stepper .done')!.textContent).toContain('Forge')
    expect(root.querySelector('h3')!.textContent).toBe('Connect to github.com')
    expect(root.querySelector('a.button')!.getAttribute('href')).toBe('https://github.com/settings/tokens/new?description=vuoro&scopes=repo')
    // The ready-made store's command is shown to copy, and how vuoro reads
    // it back waits under a disclosure.
    expect(root.querySelector('.copy code')!.textContent).toBe("secret-tool store '--label=vuoro github.com' service vuoro host github.com")
    expect(root.querySelector('details.advanced input')).not.toBeNull()
    expect(button(root, 'Save and continue').disabled).toBe(true)
    await click(root, 'Check the token')
    expect(root.textContent).toContain('✓ Signed in to github.com as @pat')
    await click(root, 'Save and continue')
    expect(post).toHaveBeenLastCalledWith('/api/setup/forges', expect.objectContaining({ base: 'b1', forge: expect.objectContaining({ kind: 'github', username: 'pat' }) }))
    expect(step()).toContain('Agent tools')
    act(() => button(root, 'Next').click())
    expect(step()).toContain('Background')
    await click(root, 'Show my inbox')
    expect(done).toHaveBeenCalled()
    act(() => render(null, root))
    root.remove()
  })

  it('goes on without a forge, and can be left at the start', () => {
    const done = vi.fn()
    const root = document.createElement('div')
    act(() => render(<FirstRun setup={setup()} onSetup={() => {}} onDone={done} />, root))
    expect(root.textContent).toContain('Set up vuoro')
    act(() => button(root, 'Continue without a forge').click())
    expect(root.querySelector('[aria-current="step"]')!.textContent).toContain('Agent tools')
    // The forge's steps were skipped, so they are not ticked.
    expect(root.querySelectorAll('.stepper .skipped')).toHaveLength(2)
    act(() => button(root, 'Back').click())
    act(() => button(root, 'Skip setup for now').click())
    expect(done).toHaveBeenCalled()
  })
})

describe('FirstRun leaving the background step', () => {
  it('waits for a save, stays once with its error when the save failed, then lets the person go on', async () => {
    let fail!: (e: Error) => void
    vi.mocked(post).mockImplementation(() => new Promise((_, reject) => { fail = reject }))
    const done = vi.fn()
    const root = document.createElement('div')
    document.body.append(root) // a checkbox fires change only in the document
    act(() => render(<FirstRun setup={setup()} onSetup={() => {}} onDone={done} />, root))
    act(() => button(root, 'Continue without a forge').click())
    act(() => button(root, 'Next').click())
    const notify = [...root.querySelectorAll('label')].find((l) => l.textContent!.includes('Notify me'))!.querySelector('input')!
    act(() => notify.click())
    await act(async () => { button(root, 'Finish').click(); await Promise.resolve() })
    expect(button(root, 'Saving…').disabled).toBe(true)
    await act(async () => { fail(new Error('config: the file changed since vuoro read it')); for (let i = 0; i < 5; i++) await Promise.resolve() })
    expect(done).not.toHaveBeenCalled()
    expect(root.querySelector('[role="alert"]')!.textContent).toContain('the file changed')
    expect(root.querySelector('[aria-current="step"]')!.textContent).toContain('Background')
    // The step shows what is saved, and the person can leave with it.
    await click(root, 'Finish')
    expect(done).toHaveBeenCalled()
    act(() => render(null, root))
    root.remove()
  })
})

describe('signedIn', () => {
  it('words how Claude Code says it is signed in', () => {
    expect(signedIn('oauth_token')).toBe('signed in with a Claude token')
    expect(signedIn('claude.ai max')).toBe('signed in with a Claude account (Max plan)')
    expect(signedIn('')).toBe('signed in')
  })
})

describe('Settings', () => {
  it('reads setup again as it opens, to pick up a hand edit, and shows each forge with its access', async () => {
    const { api } = await import('./api')
    const { Settings } = await import('./settings')
    const fresh = setup({ base: 'b2', forges: [{ name: 'edited by hand', kind: 'gitlab', url: 'https://gitlab.com', username: 'pat', token_env: 'T' }] })
    vi.mocked(api).mockResolvedValue(fresh as never)
    vi.mocked(post).mockResolvedValue({ level: 'full', scopes: ['api'] })
    const onSetup = vi.fn()
    const root = document.createElement('div')
    const existing = setup({ forges: [{ name: 'internal', kind: 'forgejo', url: 'https://forge.example', username: 'pat', token_env: 'T' }] })
    await act(async () => { render(<Settings setup={existing} onSetup={onSetup} scheme="" onScheme={() => {}} onAbout={() => {}} onClose={() => {}} />, root); await new Promise((r) => setTimeout(r, 0)) })
    expect(api).toHaveBeenCalledWith('/api/setup')
    expect(onSetup).toHaveBeenCalledWith(fresh)
    expect(post).toHaveBeenCalledWith('/api/setup/access', { name: 'internal' })
    await act(async () => { await new Promise((r) => setTimeout(r, 0)) })
    expect(root.querySelector('.forge-card .gauge.full')).not.toBeNull()
    for (const section of ['Forges', 'Agent tools', 'Background', 'Appearance', 'Confirmations', 'About']) {
      expect([...root.querySelectorAll('section > h2')].map((h) => h.textContent)).toContain(section)
    }
    act(() => render(null, root))
  })
})

describe('Forges', () => {
  it('holds Remove and Keep while a removal runs', async () => {
    let finish!: (s: Setup) => void
    vi.mocked(post).mockImplementation((path: string) => (path === '/api/setup/access'
      ? Promise.resolve({ level: 'full' })
      : new Promise((r) => { finish = r as typeof finish })) as never)
    const onSetup = vi.fn()
    const root = document.createElement('div')
    const forge = { name: 'Work', kind: 'gitlab', url: 'https://lab.example', username: 'pat', clone: 'ssh', token_env: 'VUORO_WORK_TOKEN', env_set: true }
    act(() => render(<Forges setup={setup({ forges: [forge] as unknown as Setup['forges'] })} onSetup={onSetup} />, root))
    act(() => button(root, 'Remove').click())
    await click(root, 'Remove')
    expect(button(root, 'Removing…').disabled).toBe(true)
    expect(button(root, 'Keep').disabled).toBe(true)
    expect(vi.mocked(post).mock.calls.filter(([p]) => p === '/api/setup/forges/remove')).toHaveLength(1)
    await act(async () => { finish(setup()); for (let i = 0; i < 5; i++) await Promise.resolve() })
    expect(onSetup).toHaveBeenCalledTimes(1)
  })
})

describe('Background', () => {
  const checkbox = (root: Element, label: string) =>
    [...root.querySelectorAll('label')].find((l) => l.textContent!.includes(label))!.querySelector('input') as HTMLInputElement

  it('saves the refresh interval once it is changed, and the notifications switch at once', async () => {
    vi.mocked(post).mockResolvedValue(setup())
    const onSetup = vi.fn()
    const root = document.createElement('div')
    document.body.append(root)
    act(() => render(<Background setup={setup()} onSetup={onSetup} />, root))
    expect(root.querySelector('button')).toBeNull() // nothing to press
    const minutes = root.querySelector('input[type="number"]') as HTMLInputElement
    expect(minutes.value).toBe('15')
    type(minutes, '0')
    expect(root.textContent).toContain('Choose from 1 to 1440 minutes.')
    act(() => { minutes.dispatchEvent(new Event('change')) })
    expect(post).not.toHaveBeenCalled()
    type(minutes, '5')
    await act(async () => { minutes.dispatchEvent(new Event('change')); for (let i = 0; i < 5; i++) await Promise.resolve() })
    expect(post).toHaveBeenLastCalledWith('/api/setup/background', { base: 'b1', refresh_minutes: 5, notify: true })
    await act(async () => { checkbox(root, 'Notify me').click(); for (let i = 0; i < 5; i++) await Promise.resolve() })
    expect(post).toHaveBeenLastCalledWith('/api/setup/background', { base: 'b1', refresh_minutes: 15, notify: false })
    expect(onSetup).toHaveBeenCalledTimes(2)
    root.remove()
  })

  it('shows the saved interval again when saving a new one failed', async () => {
    vi.mocked(post).mockRejectedValueOnce(new Error('config: the file changed since vuoro read it')).mockResolvedValueOnce(setup())
    const root = document.createElement('div')
    document.body.append(root)
    act(() => render(<Background setup={setup()} onSetup={vi.fn()} />, root))
    const minutes = root.querySelector('input[type="number"]') as HTMLInputElement
    type(minutes, '5')
    await act(async () => { minutes.dispatchEvent(new Event('change')); for (let i = 0; i < 5; i++) await Promise.resolve() })
    expect(root.textContent).toContain('the file changed')
    expect(minutes.value).toBe('15')
    // The next save carries the interval that is saved, which is the one shown.
    await act(async () => { checkbox(root, 'Notify me').click(); for (let i = 0; i < 5; i++) await Promise.resolve() })
    expect(post).toHaveBeenLastCalledWith('/api/setup/background', { base: 'b1', refresh_minutes: 15, notify: false })
    root.remove()
  })

  it('turns start at login on at once, and says so where the system cannot', async () => {
    vi.mocked(post).mockResolvedValueOnce(setup())
    const root = document.createElement('div')
    document.body.append(root) // a checkbox fires change only in the document
    act(() => render(<Background setup={setup()} onSetup={vi.fn()} />, root))
    await act(async () => { checkbox(root, 'Start vuoro').click(); for (let i = 0; i < 5; i++) await Promise.resolve() })
    expect(post).toHaveBeenCalledWith('/api/setup/autostart', { enabled: true })
    act(() => render(<Background setup={setup({ background: { refresh_minutes: 15, notify: true, autostart: false, login: false, dock: false, dock_here: false, dock_restart: false } })} onSetup={vi.fn()} />, root))
    expect(root.textContent).toContain('cannot start at login on this system')
    root.remove()
  })

  it('offers the Dock only on a Mac, and says the choice waits for a restart', async () => {
    const mac = { refresh_minutes: 15, notify: true, autostart: false, login: true, dock: false, dock_here: true, dock_restart: false }
    vi.mocked(post).mockResolvedValueOnce(setup({ background: { ...mac, dock: true, dock_restart: true } }))
    const root = document.createElement('div')
    document.body.append(root)
    act(() => render(<Background setup={setup()} onSetup={vi.fn()} />, root))
    expect(root.textContent).not.toContain('Dock')
    act(() => render(<Background setup={setup({ background: mac })} onSetup={vi.fn()} />, root))
    await act(async () => { checkbox(root, 'Keep vuoro in the Dock').click(); for (let i = 0; i < 5; i++) await Promise.resolve() })
    expect(post).toHaveBeenLastCalledWith('/api/setup/background', { base: 'b1', refresh_minutes: 15, notify: true, dock: true })
    act(() => render(<Background setup={setup({ background: { ...mac, dock: true, dock_restart: true } })} onSetup={vi.fn()} />, root))
    expect(root.textContent).toContain('open it again to add it to the Dock')
    root.remove()
  })
})
