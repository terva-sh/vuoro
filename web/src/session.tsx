// One agent session: its transcript, and the person's line to it.
import { useEffect, useRef, useState } from 'preact/hooks'
import { errorText, useAction } from './action'
import { post } from './api'
import { TextEditor } from './textedit'
import type { Entry, Run, Session } from './types'
import { Prose, StatusBadge, usd } from './ui'

export function SessionPane({ run, session }: { run: Run; session: Session }) {
  const end = useRef<HTMLDivElement>(null)
  const log = session.log ?? []
  useEffect(() => {
    end.current?.scrollIntoView({ block: 'nearest' })
  }, [log.length, log[log.length - 1]?.text?.length])
  const i = session.info
  return (
    <div class="session">
      <header class="session-head">
        <strong>{session.profile}{session.target ? ` for ${session.target}` : ''}</strong>
        <StatusBadge status={session.status} />
        <span class="muted">
          {i ? `${i.tool} ${i.version} · ${i.model}${i.account ? ` · ${i.account}` : ''}` : `${session.tool}${session.model ? ` · ${session.model}` : ''}`}
        </span>
        <span class="muted push">
          {usd(session.usage.cost_usd)} · {session.usage.input + session.usage.cache_read + session.usage.cache_write} in, {session.usage.output} out
        </span>
      </header>
      {session.error && <p class="alert">⚠ {session.error}</p>}
      <div class="transcript">
        {log.map((e, k) => <EntryView entry={e} key={k} />)}
        <div ref={end} />
      </div>
      <Talk run={run} session={session} />
    </div>
  )
}

function EntryView({ entry: e }: { entry: Entry }) {
  switch (e.kind) {
    case 'change':
      return (
        <details class="entry change">
          <summary>vuoro sent the change</summary>
          <pre>{e.text}</pre>
        </details>
      )
    case 'user':
      return <div class="entry user"><Prose text={e.text ?? ''} /></div>
    case 'assistant':
      return <div class="entry assistant"><Prose text={e.text ?? ''} /></div>
    case 'tool':
      return (
        <details class={`entry tool ${e.is_error ? 'refused' : ''}`}>
          <summary>
            {e.is_error ? '⚠' : '▸'} {e.tool?.replace(/^mcp__vuoro__/, '')} <span class="muted mono">{toolHint(e.input)}</span>
          </summary>
          <pre>{e.input}</pre>
          {e.output && <pre class={e.is_error ? 'refusal' : ''}>{e.output}</pre>}
        </details>
      )
    case 'denied':
      return <div class="entry denied">⊘ refused by the lockdown: {e.tool}</div>
    case 'error':
      return <div class="entry error">⚠ {e.text}</div>
  }
}

// toolHint is the one argument that says what a call was about.
function toolHint(input?: string): string {
  try {
    const v = JSON.parse(input ?? '{}') as Record<string, unknown>
    const s = v.file_path ?? v.pattern ?? v.title ?? v.id ?? ''
    return String(s).replace(/^.*\/worktrees\/[^/]+\//, '')
  } catch {
    return ''
  }
}

// Talk is the person's line to one session. Enter sends, and Shift+Enter
// starts a new line. A session that ended is resumed by the next message.
function Talk({ run, session }: { run: Run; session: Session }) {
  const [text, setText] = useState('')
  const { busy: sending, error, setError, run: act } = useAction()
  // An editor outlives its review's close; the reviewers do not.
  if (run.status === 'closed' && session.role !== 'editor') return <p class="muted pad">The run is closed.</p>
  if (session.status === 'waiting') return <p class="muted pad">⧗ Starts when every reviewer has reported.</p>
  const working = session.status === 'working'
  const send = async () => {
    if (!text.trim() || working) return
    await act(async () => {
      await post(`/api/runs/${run.id}/sessions/${session.id}/messages`, { text })
      setText('')
    })
  }
  const stop = async () => {
    try {
      await post(`/api/runs/${run.id}/sessions/${session.id}/stop`)
    } catch (e) {
      setError(errorText(e))
    }
  }
  return (
    <>
      {error && <p class="alert">⚠ {error}</p>}
      <form class="talk" onSubmit={(e) => { e.preventDefault(); void send() }}>
        <TextEditor
          rows={2}
          value={text}
          placeholder={session.status === 'ended' ? 'Ask, and the session resumes' : session.role === 'ask' ? 'Ask a follow-up question' : `Ask the ${session.profile} reviewer why, or to look elsewhere, or to reword a draft`}
          label={session.role === 'ask' ? 'Ask a follow-up question' : `Message the ${session.profile} reviewer`}
          onInput={setText}
          submitOn="enter"
        />
        {working ? (
          <button type="button" onClick={stop}>■ Stop</button>
        ) : (
          <button class="primary" disabled={sending || !text.trim()}>Send</button>
        )}
      </form>
    </>
  )
}
