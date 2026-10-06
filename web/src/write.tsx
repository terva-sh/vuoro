// Writing to the forge. Every write here is a click of the person's: an
// agent's drafts reach the forge only when the person keeps them.
import { createContext } from 'preact'
import { useContext, useState } from 'preact/hooks'
import { errorText } from './action'
import { post } from './api'
import type { Capabilities, Change } from './types'

export interface Writer {
  url: string
  caps: Capabilities
  // act posts to a write route, which answers with the change as it now
  // stands, and shows that.
  act: (path: string, body: Record<string, unknown>) => Promise<void>
}

export const WriteContext = createContext<Writer | undefined>(undefined)

export function useWriter(): Writer | undefined {
  return useContext(WriteContext)
}

export function makeWriter(url: string, caps: Capabilities, onChange: (c: Change) => void, onError: (e: string) => void): Writer {
  return {
    url,
    caps,
    act: async (path, body) => {
      try {
        onChange(await post<Change>(path, { url, ...body }))
      } catch (e) {
        onError(errorText(e))
        throw e
      }
    },
  }
}

// Composer is a box for a comment of the person's own.
export function Composer({ placeholder, initial = '', submit, onDone, label = 'Add to my pending comments' }: {
  placeholder: string; initial?: string; label?: string
  submit: (text: string) => Promise<void>; onDone: () => void
}) {
  const [text, setText] = useState(initial)
  const [busy, setBusy] = useState(false)
  return (
    <form
      class="composer"
      onSubmit={async (e) => {
        e.preventDefault()
        if (!text.trim()) return
        setBusy(true)
        try {
          await submit(text)
          onDone()
        } catch {
          // the writer has shown the error
        } finally {
          setBusy(false)
        }
      }}
    >
      <textarea rows={4} value={text} placeholder={placeholder} aria-label={placeholder}
        onInput={(e) => setText((e.target as HTMLTextAreaElement).value)} />
      <div class="composer-actions">
        <button class="primary" disabled={busy || !text.trim()}>{busy ? 'Saving…' : label}</button>
        <button type="button" onClick={onDone}>Cancel</button>
      </div>
    </form>
  )
}
