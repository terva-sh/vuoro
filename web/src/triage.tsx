import { useEffect, useRef, useState } from 'preact/hooks'
import { api, post } from './api'
import { MyDraft, ThreadView } from './diffview'
import { SessionPane } from './run'
import { HeadBadge, useRunHead, type HeadState } from './head'
import { ago, Prose, short, StatusBadge } from './ui'
import { useWriter, WriteContext } from './write'
import { refKey, type Change, type Draft, type Run, type Setup, type Thread, type TriageCategory } from './types'

const categories: [TriageCategory, string, string][] = [
  ['must_fix', 'Must fix', 'Supported defects to address before merging.'],
  ['discuss', 'Discuss', 'Questions, tradeoffs, or findings that need more evidence.'],
  ['nit', 'Nit', 'Minor improvements you can choose to make.'],
]
export const humanThreads = (change: Change) => (change.threads ?? []).filter((t) => !t.resolved && t.notes.some((n) => !n.system && !n.author.bot && n.author.username.toLowerCase() !== change.author.username.toLowerCase()))
// Compare complete human discussions, including undated edits. The server uses
// its own fingerprint after a fresh forge read before accepting any reply.
export const threadStamp = (t: Thread) => JSON.stringify([
  t.id, t.resolved, t.resolvable,
  t.anchor ? [t.anchor.path, t.anchor.old_path, t.anchor.new_line, t.anchor.old_line, t.anchor.diff] : null,
  t.notes.filter((n) => !n.system && !n.author.bot).map((n) => [n.id, n.author.username, n.body, n.at]),
])

type TriageProps = { change: Change; setup?: Setup; onPick: (path: string) => void }

export function TriagePanel(props: TriageProps) {
  // Scope selection, polling and mutation responses to this change, even when
  // a caller reuses the panel without keying its own change workspace.
  return <TriageWorkspace {...props} key={refKey(props.change.ref)} />
}

function TriageWorkspace({ change, setup, onPick }: TriageProps) {
  const [runs, setRuns] = useState<Run[]>([])
  const [selected, setSelected] = useState<string>()
  const [run, setRun] = useState<Run>()
  const [tool, setTool] = useState<string>()
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string>()
  const version = useRef(0)
  const working = useRef(false)
  const mounted = useRef(true)
  const key = refKey(change.ref)
  const tools = setup?.tools.filter((t) => !t.error) ?? []
  const openThreads = humanThreads(change)

  useEffect(() => {
    mounted.current = true
    const controller = new AbortController()
    const stamp = version.current
    api<Run[]>('/api/runs', { signal: controller.signal }).then((all) => {
      if (!mounted.current || controller.signal.aborted || stamp !== version.current) return
      const mine = all.filter((r) => r.mode === 'triage' && refKey(r.ref) === key)
      setRuns(mine)
      setSelected((id) => id ?? mine[0]?.id)
    }).catch((e: unknown) => { if (!controller.signal.aborted) setError(String(e)) })
    return () => { mounted.current = false; controller.abort() }
  }, [key])

  useEffect(() => {
    if (!selected) return
    let active = true
    let timer: ReturnType<typeof setTimeout> | undefined
    const controller = new AbortController()
    const tick = async () => {
      const stamp = version.current
      if (!working.current) {
        try {
          const next = await api<Run>(`/api/runs/${selected}`, { signal: controller.signal })
          if (active && stamp === version.current && !working.current) setRun(next)
        } catch (e) { if (active) setError(e instanceof Error ? e.message : String(e)) }
      }
      if (active) timer = setTimeout(tick, 1500)
    }
    void tick()
    return () => { active = false; controller.abort(); clearTimeout(timer) }
  }, [selected])

  const mutate = async (action: () => Promise<Run>) => {
    if (working.current) return
    working.current = true
    version.current++
    setBusy(true)
    setError(undefined)
    try {
      const next = await action()
      if (mounted.current) { setRun(next); setSelected(next.id); setRuns((all) => [next, ...all.filter((r) => r.id !== next.id)]) }
    } catch (e) { if (mounted.current) setError(e instanceof Error ? e.message : String(e)) }
    finally { working.current = false; if (mounted.current) setBusy(false) }
  }
  const start = () => mutate(() => post<Run>('/api/triage', { url: change.web_url, tool: tool ?? tools[0]?.name }))
  const current = run && run.id === selected && refKey(run.ref) === key ? run : undefined
  const headState = useRunHead(current?.id, current?.head, change)
  const moved = !!current && current.head !== change.diff.head
  const drafts = new Map((current?.drafts ?? []).filter((d) => d.status === 'pending').map((d) => [d.thread_id, d]))
  const snapshots = new Map((current?.triage_threads ?? []).map((t) => [t.id, t]))
  const openIDs = new Set(openThreads.map((t) => t.id))
  const history = (current?.triage_threads ?? []).filter((t) => !openIDs.has(t.id) && drafts.has(t.id))
  const threads = [...openThreads, ...history]
  const stale = (t: Thread) => change.state !== 'open' || moved || !openIDs.has(t.id) || (!!drafts.get(t.id) && (!snapshots.get(t.id) || threadStamp(t) !== threadStamp(snapshots.get(t.id)!)))
  const group = (category?: TriageCategory) => threads.filter((t) => drafts.get(t.id)?.category === category)
  const show = (items: Thread[]) => <div class="author-threads">{items.map((t) => {
    const d = drafts.get(t.id)
    return <article class="author-thread" key={t.id}>
      <div class="author-thread-heading"><strong>{t.anchor ? `${t.anchor.path}:${t.anchor.new_line || t.anchor.old_line}` : 'General discussion'}</strong>{t.anchor && <button class="link" onClick={() => onPick(t.anchor!.path)}>Open in Changes</button>}</div>
      {d && <div class="triage-reason"><span class="badge">Agent suggestion{d.edited ? ' · edited by you' : ''}</span><p>{d.reason}</p></div>}
      {openIDs.has(t.id) ? <ThreadView thread={t} /> : <>
        <p class="alert" role="status">This thread is no longer open in the current snapshot. Its saved discussion and reply remain here for reference.</p>
        <WriteContext.Provider value={undefined}><ThreadView thread={t} /></WriteContext.Provider>
      </>}
      {d && current && <ReplyCard key={`${current.id}:${d.id}`} draft={d} run={current} headState={headState} stale={stale(t)} busy={busy} mutate={mutate} />}
      {(change.drafts ?? []).filter((p) => p.thread_id === t.id).map((p) => <MyDraft draft={p} key={p.id} />)}
      {!d && <p class="muted">{current?.status === 'working' || current?.status === 'preparing' ? 'Waiting for the agent to classify this thread…' : 'Not triaged yet. You can reply directly or start triage.'}</p>}
    </article>
  })}</div>

  return <section class="author-triage" aria-label="Author thread triage">
    <div class="toolbar"><div><h2>Work through your review threads</h2><p class="muted">The agent suggests priorities and replies. You decide what to say.</p></div>
      {tools.length > 1 && <label>Agent <select value={tool ?? tools[0]?.name} disabled={busy} onChange={(e) => setTool(e.currentTarget.value)}>{tools.map((t) => <option key={t.name} value={t.name}>{t.tool}</option>)}</select></label>}
      <button class="primary" disabled={busy || tools.length === 0 || openThreads.length === 0 || change.state !== 'open' || current?.status === 'working' || current?.status === 'preparing'} onClick={start}>{busy ? 'Working…' : current ? 'Run triage again' : 'Triage open threads'}</button>
      {runs.length > 0 && <label>Triage run <select disabled={busy} value={selected} onChange={(e) => { version.current++; setSelected(e.currentTarget.value); setRun(undefined) }}>{runs.map((r) => <option key={r.id} value={r.id}>{ago(r.created)} at {short(r.head)}</option>)}</select></label>}
    </div>
    {tools.length === 0 && <p class="muted">No agent tool is available. You can still read the threads and reply directly when posting is supported.</p>}
    {!change.capabilities.drafts && <p class="muted">Replies can be drafted, edited and copied locally. Posting on this forge is unavailable.</p>}
    {error && <p class="alert" role="alert">{error}</p>}
    {current && <div class="triage-run"><StatusBadge status={current.status} /><HeadBadge state={headState} label="triaged" />{current.error && <p class="alert">{current.error}</p>}{moved && <p class="alert" role="status">The head changed. Refresh and run triage again before accepting replies.</p>}
      <details><summary>Talk to the agent · progress and reasoning</summary>{current.sessions.map((s) => <SessionPane run={current} session={s} key={s.id} />)}</details>
    </div>}
    {threads.length === 0 ? <p>No open human review threads to triage.</p> : <>
      <nav class="triage-groups" aria-label="Thread categories">{categories.map(([c, label]) => <a key={c} href={`#triage-${c}`}>{label} <span class="badge">{group(c).length}</span></a>)}<a href="#triage-untriaged">Untriaged <span class="badge">{group().length}</span></a></nav>
      {categories.map(([c, label, explanation]) => <section id={`triage-${c}`} class="triage-group" key={c}><h3>{label} <span class="badge">{group(c).length}</span></h3><p class="muted">{explanation}</p>{show(group(c))}</section>)}
      {group().length > 0 && <section id="triage-untriaged" class="triage-group"><h3>Untriaged <span class="badge">{group().length}</span></h3>{show(group())}</section>}
    </>}
  </section>
}

function ReplyCard({ draft, run, headState, stale, busy, mutate }: { draft: Draft; run: Run; headState?: HeadState; stale: boolean; busy: boolean; mutate: (action: () => Promise<Run>) => Promise<void> }) {
  const w = useWriter()
  const [editing, setEditing] = useState(false)
  const [body, setBody] = useState(draft.description)
  const [category, setCategory] = useState(draft.category!)
  const [copied, setCopied] = useState(false)
  useEffect(() => { if (!editing) { setBody(draft.description); setCategory(draft.category!) } }, [draft, editing])
  const base = `/api/runs/${run.id}/drafts/${draft.id}`
  const copy = <button disabled={busy || !draft.description.trim()} onClick={() => void navigator.clipboard.writeText(draft.description).then(() => setCopied(true)).catch(() => { if (!draft.decision) setEditing(true) })}>{copied ? 'Copied' : 'Copy reply'}</button>
  return <div class="triage-reply">
    <div class="draft-meta muted">Drafted reply · {draft.drafted_by}{draft.edited && ' · edited by you'}</div>
    <HeadBadge state={headState} label="triaged" />
    {stale && !draft.decision && <p class="alert" role="status">This discussion or head changed. Run triage again before accepting this reply.</p>}
    {draft.decision ? <><p class="muted">{draft.decision === 'kept' ? 'Accepted into pending comments. Current pending and published state comes from the forge.' : 'Reply discarded. The category and reason remain for reference.'}</p>{draft.description.trim() && <><Prose text={draft.description} />{copy}</>}</> : editing ? <form class="composer" onSubmit={(e) => { e.preventDefault(); void mutate(async () => { const next = await post<Run>(`${base}/edit-reply`, { body, category }); setEditing(false); return next }) }}>
      <label>Category <select value={category} onChange={(e) => setCategory(e.currentTarget.value as TriageCategory)}>{categories.map(([c,label]) => <option value={c} key={c}>{label}</option>)}</select></label>
      <textarea rows={5} value={body} aria-label="Edit drafted reply" onInput={(e) => setBody(e.currentTarget.value)} />
      <div class="actions"><button class="primary" disabled={busy}>Save locally</button><button type="button" disabled={busy} onClick={() => setEditing(false)}>Cancel</button></div>
      <p class="muted">Saving does not send anything to the forge.</p>
    </form> : <>
      {draft.description.trim() ? <Prose text={draft.description} /> : <p class="muted">No reply proposed. Write one if this thread needs an answer.</p>}
      <div class="actions"><button disabled={busy} onClick={() => setEditing(true)}>Edit reply and category</button>
        {w && <button class="primary" disabled={busy || stale || !draft.description.trim()} onClick={() => void mutate(async () => { await w.act(`${base}/keep`, {}); return api<Run>(`/api/runs/${run.id}`) })}>Add to pending replies</button>}
        {copy}
        <button disabled={busy} onClick={() => void mutate(() => post<Run>(`${base}/discard-reply`))}>Discard reply</button>
      </div>
      <p class="muted">{w ? 'Add to pending replies sends a private draft. Submit above publishes your pending comments.' : 'This reply stays local until you copy it or posting becomes available.'}</p>
    </>}
  </div>
}
