import { useEffect, useRef, useState } from 'preact/hooks'
import { api, post } from './api'
import { DraftCard } from './diffview'
import type { Change, Draft, Entry, Run, Session, Setup } from './types'
import { refKey } from './types'
import { ago, Prose, short, StatusBadge, usd } from './ui'

// usePoll reads a run, quickly while anything works and slowly otherwise.
function usePoll(id: string | undefined, onRun: (r: Run) => void) {
  const live = useRef(true)
  useEffect(() => {
    live.current = true
    if (!id) return
    let timer: ReturnType<typeof setTimeout>
    const tick = async () => {
      try {
        const r = await api<Run>(`/api/runs/${id}`)
        if (!live.current) return
        onRun(r)
        const busy = r.status === 'preparing' || r.sessions.some((s) => s.status === 'working' || s.status === 'preparing')
        timer = setTimeout(tick, busy ? 1000 : 5000)
      } catch {
        timer = setTimeout(tick, 5000)
      }
    }
    void tick()
    return () => {
      live.current = false
      clearTimeout(timer)
    }
  }, [id])
}

export function RunPanel({ change, setup, onPick, onDrafts }: {
  change: Change; setup?: Setup; onPick: (path: string) => void; onDrafts: (d: Draft[]) => void
}) {
  const [runs, setRuns] = useState<Run[]>([])
  const [selected, setSelected] = useState<string>()
  const [run, setRun] = useState<Run>()
  const [preset, setPreset] = useState<string>()
  const [error, setError] = useState<string>()
  const [busy, setBusy] = useState(false)

  const key = refKey(change.ref)
  useEffect(() => {
    api<Run[]>('/api/runs')
      .then((all) => {
        const mine = all.filter((r) => refKey(r.ref) === key)
        setRuns(mine)
        if (mine.length > 0) setSelected((s) => s ?? mine[0].id)
      })
      .catch((e: unknown) => setError(String(e)))
  }, [change]) // a refresh reads the change again, and the runs with it
  usePoll(selected, setRun)
  useEffect(() => onDrafts(run?.drafts ?? []), [run])

  const tools = setup?.tools.filter((t) => !t.error) ?? []
  const start = async () => {
    setBusy(true)
    setError(undefined)
    try {
      const r = await post<Run>('/api/runs', { url: change.web_url, preset: preset ?? setup?.presets[0]?.name })
      setRuns([r, ...runs])
      setSelected(r.id)
      setRun(r)
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e))
    } finally {
      setBusy(false)
    }
  }
  const close = async () => {
    if (!run) return
    try {
      setRun(await post<Run>(`/api/runs/${run.id}/close`))
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e))
    }
  }

  return (
    <div class="run-panel">
      <div class="toolbar">
        {tools.length === 0 ? (
          <span class="muted">
            {setup?.tools.map((t) => `${t.tool}: ${t.error}`).join('; ') || 'No agent tool is available.'}
          </span>
        ) : (
          <>
            <label>
              Preset{' '}
              <select value={preset} onChange={(e) => setPreset((e.target as HTMLSelectElement).value)}>
                {setup?.presets.map((p) => (
                  <option value={p.name} key={p.name}>
                    {p.name}: {p.reviewers.map((r) => r.profile + (r.model ? ` (${r.model})` : '')).join(', ')}
                  </option>
                ))}
              </select>
            </label>
            <button class="primary" disabled={busy} onClick={start}>{busy ? 'Starting…' : 'Start a review'}</button>
            <span class="muted">{tools.map((t) => `${t.tool} ${t.version}${t.account ? `, ${t.account}` : ''}`).join(' · ')}</span>
          </>
        )}
        {runs.length > 0 && (
          <label class="push">
            Run{' '}
            <select value={selected} onChange={(e) => setSelected((e.target as HTMLSelectElement).value)}>
              {runs.map((r) => (
                <option value={r.id} key={r.id}>{ago(r.created)} at {short(r.head)}, {r.status}</option>
              ))}
            </select>
          </label>
        )}
      </div>
      {error && <p class="alert" role="alert">⚠ {error}</p>}
      {run && <RunView run={run} change={change} onClose={close} onPick={onPick} />}
    </div>
  )
}

function RunView({ run, change, onClose, onPick }: { run: Run; change: Change; onClose: () => void; onPick: (path: string) => void }) {
  const moved = run.head !== change.diff.head
  const pending = run.drafts.filter((d) => d.status === 'pending')
  const withdrawn = run.drafts.length - pending.length
  return (
    <div class="run">
      <div class="run-head">
        <StatusBadge status={run.status} />
        <span>reviewed at <span class="mono">{short(run.head)}</span></span>
        {moved && <span class="badge warn">⚠ the change has moved to {short(change.diff.head)}</span>}
        <span class="muted">total {usd(run.cost_usd)}</span>
        {run.status !== 'closed' && <button class="push" onClick={onClose}>Close the run</button>}
      </div>
      {run.error && <p class="alert" role="alert">⚠ {run.error}</p>}
      <div class="sessions">
        {run.sessions.map((s) => <SessionPane run={run} session={s} key={s.id} />)}
      </div>
      <h3>
        Drafts: {pending.length} pending{withdrawn > 0 && `, ${withdrawn} withdrawn`}
      </h3>
      <p class="muted">An agent drafts, and nothing here is posted. Posting is yours, from the pending drafts.</p>
      {run.drafts.map((d) => (
        <DraftCard draft={d} key={d.id} onPick={d.anchor ? () => onPick(d.anchor!.path) : undefined} />
      ))}
    </div>
  )
}

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
        <strong>{session.profile}</strong>
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
  const [error, setError] = useState<string>()
  const [sending, setSending] = useState(false)
  if (run.status === 'closed') return <p class="muted pad">The run is closed.</p>
  const working = session.status === 'working'
  const send = async () => {
    if (!text.trim() || working) return
    setSending(true)
    setError(undefined)
    try {
      await post(`/api/runs/${run.id}/sessions/${session.id}/messages`, { text })
      setText('')
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e))
    } finally {
      setSending(false)
    }
  }
  const stop = async () => {
    try {
      await post(`/api/runs/${run.id}/sessions/${session.id}/stop`)
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e))
    }
  }
  return (
    <>
      {error && <p class="alert">⚠ {error}</p>}
      <form class="talk" onSubmit={(e) => { e.preventDefault(); void send() }}>
        <textarea
          rows={2}
          value={text}
          placeholder={session.status === 'ended' ? 'Ask, and the session resumes' : `Ask the ${session.profile} reviewer why, or to look elsewhere, or to reword a draft`}
          aria-label={`Message the ${session.profile} reviewer`}
          onInput={(e) => setText((e.target as HTMLTextAreaElement).value)}
          onKeyDown={(e) => {
            if (e.key === 'Enter' && !e.shiftKey) {
              e.preventDefault()
              void send()
            }
          }}
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
