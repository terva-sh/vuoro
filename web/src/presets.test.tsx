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

const setup = (presets: Preset[]): Setup => ({
  editable: true, base: 'b1', config_path: '/home/pat/.config/vuoro/config.json', forges: [], sources: [],
  tools: [{ name: 'claude', tool: 'Claude Code', state: 'ok' }], presets,
  profiles: [{ name: 'code', title: 'Code', prompt: '' }, { name: 'security', title: 'Security', prompt: '' }],
  waived: [], background: { refresh_minutes: 15, notify: true, autostart: false, login: true, dock: false, dock_here: false, dock_restart: false },
  scheme: '', tab: '',
})

const button = (root: Element, text: string) => [...root.querySelectorAll('button')].find((b) => b.textContent === text)
const input = (root: Element, label: string) =>
  [...root.querySelectorAll('label')].find((l) => l.textContent!.startsWith(label))!.querySelector('input, select') as HTMLInputElement
const type = (el: HTMLInputElement, value: string) =>
  act(() => { el.value = value; el.dispatchEvent(new Event(el.tagName === 'SELECT' ? 'change' : 'input')) })
const click = (root: Element, text: string) =>
  act(async () => { button(root, text)!.click(); for (let i = 0; i < 5; i++) await Promise.resolve() })

const mount = (presets: Preset[], onSetup = vi.fn()) => {
  act(() => render(<Presets setup={setup(presets)} onSetup={onSetup} />, root))
  return { root, onSetup }
}

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
    const { root, onSetup } = mount([quick])
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
