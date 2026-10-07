// A review run: its agents, its findings, and the hand-back, polled while
// anything works.
import { useEffect, useState } from 'preact/hooks'
import { errorText, useAction } from './action'
import { api, post } from './api'
import { Editor } from './editor'
import { Drafts, HandOff, shownDrafts } from './findings'
import { HeadBadge, useHead } from './head'
import { usePoll } from './poll'
import { SessionPane } from './session'
import type { Change, Draft, Level, Run, Session, Setup } from './types'
import { refKey } from './types'
import { WriteContext } from './write'
import { ago, levelMark, short, StatusBadge, usd } from './ui'

// useRunPoll reads a run, quickly while anything works and slowly otherwise.
function useRunPoll(id: string | undefined, onRun: (r: Run) => void) {
  usePoll(async (signal) => {
    if (!id) return undefined
    try {
      const r = await api<Run>(`/api/runs/${id}`, { signal })
      if (signal.aborted) return undefined
      onRun(r)
      const busy = r.status === 'preparing' || r.sessions.some((s) => s.status === 'working' || s.status === 'preparing')
      return busy ? 1000 : 3000
    } catch {
      return 5000
    }
  }, [id])
}

export function RunPanel({ change, setup, onPick, onDrafts }: {
  change: Change; setup?: Setup; onPick: (path: string) => void; onDrafts: (d: Draft[], runID?: string, head?: string, pushed?: string) => void
}) {
  const [runs, setRuns] = useState<Run[]>([])
  const [selected, setSelected] = useState<string>()
  const [run, setRun] = useState<Run>()
  const [preset, setPreset] = useState<string>()
  const { busy, error, setError, run: act } = useAction()

  const key = refKey(change.ref)
  useEffect(() => {
    api<Run[]>('/api/runs')
      .then((all) => {
        const mine = all.filter((r) => !r.mode && refKey(r.ref) === key)
        setRuns(mine)
        if (mine.length > 0) setSelected((s) => s ?? mine[0].id)
      })
      .catch((e: unknown) => setError(String(e)))
  }, [change]) // a refresh reads the change again, and the runs with it
  // A run read again replaces its entry in the list too, so the selector's
  // label follows its status.
  const update = (r: Run) => {
    setRun(r)
    setRuns((rs) => rs.map((x) => (x.id === r.id ? r : x)))
  }
  useRunPoll(selected, update)
  // A review of the person's own change posts nothing (D42).
  const me = setup?.forges.find((f) => { try { return new URL(f.url).host === change.ref.host } catch { return false } })
  const own = !!me && me.username.toLowerCase() === change.author.username.toLowerCase()
  useEffect(() => onDrafts(run ? shownDrafts(run) : [], run?.id, run?.head, run?.editor?.pushed?.sha), [run])

  const tools = setup?.tools.filter((t) => !t.error) ?? []
  const start = () => act(async () => {
    const r = await post<Run>('/api/runs', { url: change.web_url, preset: preset ?? setup?.presets[0]?.name })
    setRuns([r, ...runs])
    setSelected(r.id)
    setRun(r)
  })
  const close = async () => {
    if (!run) return
    try {
      update(await post<Run>(`/api/runs/${run.id}/close`))
    } catch (e) {
      setError(errorText(e))
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
      {run && <RunView run={run} url={change.web_url} own={own} onClose={close} onPick={onPick} onRun={update} />}
    </div>
  )
}

// RunView is one run. own marks a review of the person's own work, a merge
// request of theirs or a local range, whose findings are picked for the
// editor or dismissed, and never posted (D42).
export function RunView({ run, url, own = false, onClose, onPick, onRun }: {
  run: Run; url: string; own?: boolean; onClose: () => void; onPick: (path: string) => void; onRun: (r: Run) => void
}) {
  const self = own || run.kind === 'local'
  // The findings to hand the editor: pending and undecided ones above low
  // severity that the verifier did not doubt, until the person changes them.
  const [picks, setPicks] = useState<Record<string, boolean>>({})
  const pickable = (d: Draft) => self && d.status === 'pending' && !d.decision && !d.fixed_in && !d.thread_id
  const isPicked = (d: Draft) => pickable(d) && (picks[d.id] ?? (['critical', 'high', 'medium'].includes(d.severity) && (d.override ?? d.confidence)?.level !== 'doubtful'))
  const headState = useHead()
  // A pane opens while its session works, and the person can fold or open
  // any of them.
  const [opened, setOpened] = useState<Record<string, boolean>>({})
  const isOpen = (s: Session) => opened[s.id] ?? (s.status === 'working' || s.status === 'preparing')
  const shown = shownDrafts(run)
  const kept = shown.filter((d) => d.decision === 'kept').length
  const dropped = shown.filter((d) => d.decision === 'dropped').length
  const fixed = shown.filter((d) => d.fixed_in).length
  const marked = (l: Level) => shown.filter((d) => (d.override ?? d.confidence)?.level === l).length
  const unchecked = shown.filter((d) => !(d.override ?? d.confidence)).length
  return (
    <div class="run">
      <section class="run-section">
        <header class="run-section-head">
          <h3>Agents</h3>
          <StatusBadge status={run.status} />
          {headState ? <HeadBadge state={headState} /> : <span>reviewed at <span class="mono">{short(run.head)}</span></span>}
          <span class="muted">total {usd(run.cost_usd)}</span>
          {run.status !== 'closed' && <button class="push" onClick={onClose}>Close the run</button>}
        </header>
        {run.error && <p class="alert" role="alert">⚠ {run.error}</p>}
        <div class="session-strip">
          {run.sessions.map((s) => (
            <button class={`session-chip ${isOpen(s) ? 'on' : ''}`} key={s.id} aria-expanded={isOpen(s)}
              onClick={() => setOpened({ ...opened, [s.id]: !isOpen(s) })}>
              <strong>{s.profile}{s.target ? ` ${s.target}` : ''}</strong> <StatusBadge status={s.status} />
              <span class="muted">{s.info?.model ?? s.model ?? ''} · {usd(s.usage.cost_usd)}</span>
            </button>
          ))}
        </div>
        {run.sessions.some(isOpen) && (
          <div class="sessions">
            {run.sessions.filter(isOpen).map((s) => <SessionPane run={run} session={s} key={s.id} />)}
          </div>
        )}
      </section>
      <section class="run-section">
        <header class="run-section-head triage">
          <h3>Findings · {shown.length}</h3>
          <span>✓ {kept} kept</span>
          <span>✗ {dropped} dropped</span>
          {fixed > 0 && <span>✓ {fixed} fixed</span>}
          <span>○ {shown.filter((d) => !d.decision && !d.fixed_in).length} to decide</span>
          {run.sessions.some((s) => s.role === 'verifier') && (
            <span class="muted">
              {(['confirmed', 'plausible', 'doubtful'] as Level[]).map((l) => `${levelMark[l]} ${marked(l)} ${l}`).join(' · ')}
              {unchecked > 0 && ` · ${unchecked} not checked yet`}
            </span>
          )}
          <span class="muted push">{self ? 'Your own work: nothing here is posted. Pick findings for the editor, or dismiss them.' : 'An agent drafts, and nothing is posted until you keep it and submit.'}</span>
        </header>
        {self ? (
          // Nothing here posts: the findings are picked or dismissed.
          <WriteContext.Provider value={undefined}>
            <Drafts run={run} onPick={onPick} onRun={onRun}
              pick={{ isPicked, pickable, set: (d, on) => setPicks({ ...picks, [d.id]: on }) }} />
          </WriteContext.Provider>
        ) : (
          <Drafts run={run} onPick={onPick} onRun={onRun} />
        )}
      </section>
      <section class="run-section">
        <header class="run-section-head"><h3>{self ? 'Fix' : 'Hand back'}</h3><span class="muted">{self ? 'the findings you picked, on a branch' : 'the kept findings, as a report or as fixes'}</span></header>
        {!self && <HandOff run={run} kept={run.drafts.filter((d) => d.decision === 'kept').length} />}
        <Editor run={run} url={url} onRun={onRun} picks={self} picked={self ? shownDrafts(run).filter(isPicked) : undefined} />
      </section>
    </div>
  )
}

// LocalRun is a review of a local range, from vuoro review, which has no
// change page: its findings, and the editor that fixes them (D42).
export function LocalRun({ id }: { id: string }) {
  const [run, setRun] = useState<Run>()
  const [error, setError] = useState<string>()
  useRunPoll(id, setRun)
  if (!run) return <p class="muted">Loading the run…</p>
  const close = () => void post<Run>(`/api/runs/${run.id}/close`).then(setRun, (e: unknown) => setError(errorText(e)))
  return (
    <div class="local-run">
      <h1>{run.title}</h1>
      <p class="muted">local commits of {run.ref.project}, <span class="mono">{short(run.base)}..{short(run.head)}</span>, {ago(run.created)}</p>
      {error && <p class="alert" role="alert">⚠ {error}</p>}
      <RunView run={run} url="" own onClose={close} onPick={() => undefined} onRun={setRun} />
    </div>
  )
}
