// Text the person writes or copies: one box for every comment, reply,
// message, and description, and one form around the ones that save.
import type { ComponentChildren } from 'preact'
import { useLayoutEffect, useRef } from 'preact/hooks'

// TextEditor is a text box that grows with what it holds. Cmd or Ctrl+Enter
// submits the form it sits in, or Enter alone with submitOn 'enter', where
// Shift+Enter still starts a new line. Submitting goes through the form, so
// its guards hold whichever way the person saves. A read-only one selects
// all of its text when it gets the focus, to copy by hand.
export function TextEditor({ value, onInput, label, placeholder, rows = 4, mono = false, readOnly = false, submitOn = 'mod-enter' }: {
  value: string
  onInput?: (text: string) => void
  label: string // its accessible name
  placeholder?: string
  rows?: number
  mono?: boolean
  readOnly?: boolean
  submitOn?: 'mod-enter' | 'enter'
}) {
  const box = useRef<HTMLTextAreaElement>(null)
  // Growing it: shrink to its rows, then take the height its text needs.
  useLayoutEffect(() => {
    const t = box.current
    if (!t) return
    t.style.height = ''
    if (t.scrollHeight > t.clientHeight) t.style.height = `${t.scrollHeight + 2}px`
  }, [value])
  return (
    <textarea
      ref={box}
      class={`text-editor${mono ? ' mono' : ''}`}
      rows={rows}
      value={value}
      placeholder={placeholder}
      aria-label={label}
      readOnly={readOnly}
      onInput={(e) => onInput?.(e.currentTarget.value)}
      onFocus={readOnly ? (e) => e.currentTarget.select() : undefined}
      onKeyDown={(e) => {
        const form = e.currentTarget.form
        if (!form || e.key !== 'Enter') return
        if (submitOn === 'enter' ? !e.shiftKey : e.metaKey || e.ctrlKey) {
          e.preventDefault()
          form.requestSubmit()
        }
      }}
    />
  )
}

// EditForm is a form that saves: its fields, then Save and Cancel, then the
// error and a note. Save is the form's submit, so Enter in a field and
// Cmd+Enter in a TextEditor both reach it. Without onCancel there is no
// Cancel, for a form that is the only way forward.
export function EditForm({ onSave, onCancel, busy = false, canSave = true, save, saving = 'Saving…', hint, error, note, class: cls = '', children }: {
  onSave: () => void
  onCancel?: () => void
  busy?: boolean
  canSave?: boolean
  save: string // the Save button's label
  saving?: string // its label while busy
  hint?: string // the Save button's title, such as why it cannot save yet
  error?: string
  note?: ComponentChildren
  class?: string
  children: ComponentChildren
}) {
  return (
    <form class={`edit-form ${cls}`.trim()} onSubmit={(e) => { e.preventDefault(); if (!busy && canSave) onSave() }}>
      {children}
      <div class="actions">
        <button class="primary" disabled={busy || !canSave} title={hint}>{busy ? saving : save}</button>
        {onCancel && <button type="button" disabled={busy} onClick={onCancel}>Cancel</button>}
      </div>
      {error && <p class="alert" role="alert">⚠ {error}</p>}
      {note && <p class="muted">{note}</p>}
    </form>
  )
}
