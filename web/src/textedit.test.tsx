// @vitest-environment jsdom
import { render } from 'preact'
import { act } from 'preact/test-utils'
import { describe, expect, it } from 'vitest'
import { EditForm, TextEditor } from './textedit'

const key = (t: HTMLTextAreaElement, init: KeyboardEventInit) =>
  act(() => { t.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true, cancelable: true, ...init })) })

function form(props: { busy?: boolean; canSave?: boolean; submitOn?: 'enter' | 'mod-enter'; error?: string; note?: string } = {}) {
  let saved = 0
  let cancelled = 0
  const root = document.createElement('div')
  document.body.append(root)
  act(() => render(
    <EditForm save="Save" onSave={() => saved++} onCancel={() => cancelled++} busy={props.busy} canSave={props.canSave} error={props.error} note={props.note}>
      <TextEditor value="hello" label="Reply" submitOn={props.submitOn} />
    </EditForm>, root))
  return { root, box: root.querySelector('textarea')!, saved: () => saved, cancelled: () => cancelled, button: (l: string) => [...root.querySelectorAll('button')].find((b) => b.textContent === l)! }
}

describe('TextEditor', () => {
  it('submits its form on Cmd or Ctrl+Enter, and leaves a plain Enter as a new line', () => {
    const f = form()
    key(f.box, {})
    expect(f.saved()).toBe(0)
    key(f.box, { metaKey: true })
    key(f.box, { ctrlKey: true })
    expect(f.saved()).toBe(2)
  })

  it("submits on Enter alone when asked, and Shift+Enter still starts a line", () => {
    const f = form({ submitOn: 'enter' })
    key(f.box, { shiftKey: true })
    expect(f.saved()).toBe(0)
    key(f.box, {})
    expect(f.saved()).toBe(1)
  })

  it('names itself, and a read-only one selects its text on focus to copy', () => {
    const root = document.createElement('div')
    act(() => render(<TextEditor value="the report" label="Report" readOnly mono />, root))
    const t = root.querySelector('textarea')!
    expect(t.getAttribute('aria-label')).toBe('Report')
    expect(t.classList.contains('mono')).toBe(true)
    act(() => { t.dispatchEvent(new FocusEvent('focus')) })
    expect([t.selectionStart, t.selectionEnd]).toEqual([0, 'the report'.length])
  })
})

describe('EditForm', () => {
  it('saves and cancels from its buttons', () => {
    const f = form()
    act(() => f.button('Save').click())
    act(() => f.button('Cancel').click())
    expect([f.saved(), f.cancelled()]).toEqual([1, 1])
  })

  it('saves nothing while busy or with nothing to save, by button or by key', () => {
    for (const props of [{ busy: true }, { canSave: false }]) {
      const f = form(props)
      act(() => f.button(props.busy ? 'Saving…' : 'Save').click())
      key(f.box, { metaKey: true })
      expect(f.saved()).toBe(0)
    }
  })

  it('offers no Cancel without onCancel, and titles Save with its hint', () => {
    const root = document.createElement('div')
    act(() => render(<EditForm save="Add" hint="Test the token first" canSave={false} onSave={() => {}}><input /></EditForm>, root))
    expect([...root.querySelectorAll('button')].map((b) => b.textContent)).toEqual(['Add'])
    expect(root.querySelector('button')!.title).toBe('Test the token first')
  })

  it('shows its error and its note', () => {
    const f = form({ error: 'refused', note: 'Saving does not post.' })
    expect(f.root.querySelector('[role=alert]')?.textContent).toBe('⚠ refused')
    expect(f.root.textContent).toContain('Saving does not post.')
  })
})
