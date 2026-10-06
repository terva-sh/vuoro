// Writing to the forge. Every write here is a click of the person's: an
// agent's drafts reach the forge only when the person keeps them.
import { createContext } from 'preact'
import { useContext, useState } from 'preact/hooks'
import { errorText, useAction } from './action'
import { post } from './api'
import { EditForm, TextEditor } from './textedit'
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
  // The writer shows what went wrong, so the form does not show it again.
  const { busy, run } = useAction()
  const save = () => void run(async () => { await submit(text); onDone() })
  return (
    <EditForm class="composer" onSave={save} onCancel={onDone} busy={busy} canSave={!!text.trim()} save={label}>
      <TextEditor value={text} onInput={setText} label={placeholder} placeholder={placeholder} />
    </EditForm>
  )
}
