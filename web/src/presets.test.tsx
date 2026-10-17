// @vitest-environment jsdom
import { render } from 'preact'
import { act } from 'preact/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { post } from './api'
import { copyName, describe as describePreset, Presets } from './presets'
import type { Preset, Setup } from './types'

vi.mock('./api', () => ({ api: vi.fn(), post: vi.fn() }))

const root = document.createElement('div')
document.body.append(root) // a form submits only from the document
afterEach(() => { act(() => render(null, root)); vi.resetAllMocks() })

const quick: Preset = { name: 'quick', reviewers: [{ profile: 'code' }], builtin: true }
const mine: Preset = { name: 'mine', reviewers: [{ profile: 'security', model: 'opus' }] }

const setup = (presets: Preset[], over: Partial<Setup> = {}): Setup => ({
  editable: true, base: 'b1', config_path: '/home/pat/.config/vuoro/config.json', forges: [], sources: [],
  tools: [{ name: 'claude', tool: 'Claude Code', state: 'ok' }], presets,
  profiles: [{ name: 'code', title: 'Code', prompt: '' }, { name: 'security', title: 'Security', prompt: '' }],
  waived: [], background: { refresh_minutes: 15, notify: true, autostart: false, login: true, dock: false, dock_here: false, dock_restart: false },
  scheme: '', tab: '',
  ...over,
})

const button = (root: Element, text: string) => [...root.querySelectorAll('button')].find((b) => b.textContent === text)
const input = (root: Element, label: string) =>
  [...root.querySelectorAll('label')].find((l) => l.textContent!.startsWith(label))!.querySelector('input, select') as HTMLInputElement
const type = (el: HTMLInputElement, value: string) =>
  act(() => { el.value = value; el.dispatchEvent(new Event(el.tagName === 'SELECT' ? 'change' : 'input')) })
const click = (root: Element, text: string) =>
  act(async () => { button(root, text)!.click(); for (let i = 0; i < 5; i++) await Promise.resolve() })

const mount = (presets: Preset[], over: Partial<Setup> = {}, onSetup = vi.fn()) => {
  act(() => render(<Presets setup={setup(presets, over)} onSetup={onSetup} />, root))
  return { root, onSetup }
}

// gone names a profile no configuration defines, which is what a preset
// holds after the person deletes the profile it named.
const gone: Preset = { name: 'old', reviewers: [{ profile: 'vibes' }] }

describe('describe', () => {
  it('words a preset as the run picker words it', () => {
    expect(describePreset(mine)).toBe('security (opus)')
    expect(describePreset({ name: 'x', reviewers: [{ profile: 'code' }, { profile: 'security' }] })).toBe('code, security')
  })
})

describe('copyName', () => {
  it('finds a free name, so copying twice does not collide', () => {
    expect(copyName('quick', ['quick'])).toBe('quick copy')
    expect(copyName('quick', ['quick', 'quick copy'])).toBe('quick copy 2')
    expect(copyName('quick', ['quick', 'quick copy', 'quick copy 2'])).toBe('quick copy 3')
  })
})

describe('Presets', () => {
  // D61: a shipped preset keeps its meaning across an upgrade, so the page
  // offers to copy it and never to change or delete it.
  it('copies a shipped preset and never edits or removes it', () => {
    const { root } = mount([quick])
    expect(root.textContent).toContain('shipped with vuoro')
    expect(button(root, 'Copy')).toBeDefined()
    expect(button(root, 'Edit')).toBeUndefined()
    expect(button(root, 'Remove')).toBeUndefined()
  })

  it('edits and removes one the person wrote', () => {
    const { root } = mount([mine, quick])
    expect(button(root, 'Edit')).toBeDefined()
    expect(button(root, 'Remove')).toBeDefined()
  })

  it('saves a copy of a shipped preset as a new one, under a free name', async () => {
    const { root, onSetup } = mount([quick], {})
    act(() => button(root, 'Copy')!.click())
    expect((input(root, 'Name') as HTMLInputElement).value).toBe('quick copy')
    vi.mocked(post).mockResolvedValue(setup([quick]))
    await click(root, 'Add preset')
    expect(post).toHaveBeenCalledWith('/api/setup/presets', {
      base: 'b1',
      original: '', // a copy is a new preset, never a change to the shipped one
      preset: { name: 'quick copy', reviewers: [{ profile: 'code', tool: undefined, model: undefined }], synthesizer: undefined, verifier: undefined },
    })
    expect(onSetup).toHaveBeenCalled()
  })

  it('saves a change to an existing preset against the name it replaces', async () => {
    const { root } = mount([mine])
    act(() => button(root, 'Edit')!.click())
    await type(input(root, 'Name'), 'ours')
    vi.mocked(post).mockResolvedValue(setup([]))
    await click(root, 'Save')
    expect(post).toHaveBeenCalledWith('/api/setup/presets', {
      base: 'b1',
      original: 'mine',
      preset: { name: 'ours', reviewers: [{ profile: 'security', tool: undefined, model: 'opus' }], synthesizer: undefined, verifier: undefined },
    })
  })

  it('asks before removing, and then removes by name', async () => {
    const { root } = mount([mine])
    act(() => button(root, 'Remove')!.click())
    expect(root.textContent).toContain('Remove mine?')
    vi.mocked(post).mockResolvedValue(setup([]))
    await click(root, 'Remove')
    expect(post).toHaveBeenCalledWith('/api/setup/presets/remove', { base: 'b1', name: 'mine' })
  })

  it('will not save a preset with no name', () => {
    const { root } = mount([])
    act(() => button(root, 'Add a preset')!.click())
    expect(button(root, 'Add preset')!.disabled).toBe(true)
    expect(post).not.toHaveBeenCalled()
  })

  it('offers nothing to change when the file cannot be written', () => {
    act(() => render(<Presets setup={{ ...setup([mine, quick]), editable: false }} onSetup={vi.fn()} />, root))
    expect(button(root, 'Edit')).toBeUndefined()
    expect(button(root, 'Copy')).toBeUndefined()
    expect(button(root, 'Add a preset')).toBeUndefined()
  })
})

// A preset's names are checked when a run starts, not when it is written, so
// without this the person learns on the way into a review that the preset
// they chose cannot run.
describe('a preset naming something that does not resolve', () => {
  it('names the profile that is gone', () => {
    const { root } = mount([gone])
    expect(root.textContent).toContain('No profile named "vibes", so this preset cannot run here.')
  })

  it('names a tool this machine never probed', () => {
    const { root } = mount([{ name: 'old', reviewers: [{ profile: 'code', tool: 'terva' }] }])
    expect(root.textContent).toContain('No agent tool named "terva" on this machine')
  })

  // probeTools offers a driver whenever its probe returned no error, and a
  // tool with no account probes clean. So a signed-out tool resolves and the
  // session fails at authentication, while a missing one is refused before
  // the run begins. The page says which.
  it('reads differently for a tool that is signed out and one that is absent', () => {
    const pinned = [{ name: 'old', reviewers: [{ profile: 'code', tool: 'claude' }] }]
    const out = mount(pinned, { tools: [{ name: 'claude', tool: 'Claude Code', state: 'signed_out' }] })
    expect(out.root.textContent).toContain('Claude Code is not signed in, so a run would start and then fail.')
    expect(out.root.textContent).not.toContain('cannot run here')

    const absent = mount(pinned, { tools: [{ name: 'claude', tool: 'Claude Code', state: 'missing' }] })
    expect(absent.root.textContent).toContain('Claude Code is not installed, so this preset cannot run here.')
  })

  // A shipped preset cannot name a profile that is gone, which a Go test
  // holds, but it can pin a tool.
  it('marks a shipped preset too', () => {
    const { root } = mount([{ ...quick, reviewers: [{ profile: 'code', tool: 'claude' }] }],
      { tools: [{ name: 'claude', tool: 'Claude Code', state: 'too_old' }] })
    expect(root.textContent).toContain('Claude Code is too old, so this preset cannot run here.')
  })

  it('says each missing name once, however many reviewers name it', () => {
    const { root } = mount([{ name: 'old', reviewers: [{ profile: 'vibes' }, { profile: 'vibes' }] }])
    expect(root.textContent!.match(/No profile named/g)).toHaveLength(1)
  })

  it('says nothing about a preset that resolves', () => {
    const { root } = mount([mine, quick])
    expect(root.textContent).not.toContain('cannot run here')
  })

  // A select whose value matches no option shows the first one. Without an
  // option for the dangling name the editor would read as "Code" here.
  it('shows the name the preset holds rather than the first option', () => {
    const { root } = mount([gone])
    act(() => button(root, 'Edit')!.click())
    const profile = input(root, 'Reviews for') as unknown as HTMLSelectElement
    expect(profile.value).toBe('vibes')
    expect([...profile.options].map((o) => o.textContent)).toContain('vibes (no such profile here)')
  })

  // D62: the page reports and never refuses, because a preset naming a tool
  // absent from this machine is still correct on the machine that has it.
  it('round-trips the dangling name through an edit of another field', async () => {
    const { root } = mount([gone])
    act(() => button(root, 'Edit')!.click())
    await type(input(root, 'Name'), 'older')
    vi.mocked(post).mockResolvedValue(setup([]))
    await click(root, 'Save')
    expect(post).toHaveBeenCalledWith('/api/setup/presets', {
      base: 'b1',
      original: 'old',
      preset: { name: 'older', reviewers: [{ profile: 'vibes', tool: undefined, model: undefined }], synthesizer: undefined, verifier: undefined },
    })
  })

  it('marks a tool that cannot run in the select, so pinning one is a choice', () => {
    const { root } = mount([mine], { tools: [{ name: 'claude', tool: 'Claude Code', state: 'missing' }] })
    act(() => button(root, 'Edit')!.click())
    const tool = input(root, 'On') as unknown as HTMLSelectElement
    expect([...tool.options].map((o) => o.textContent)).toContain('Claude Code (not installed)')
  })
})
