import { useEffect, useRef, useState } from 'preact/hooks'
import { api, post } from './api'
import { HeadBadge, useRunHead } from './head'
import { refKey, type Change, type Run, type Setup } from './types'
import { ago, Prose, short, StatusBadge, usd } from './ui'

type Props = { change: Change; setup?: Setup }
export function SummaryPanel(props: Props) {
  return <SummaryWorkspace {...props} key={refKey(props.change.ref)} />
}

function SummaryWorkspace({ change, setup }: Props) {
  const [runs, setRuns] = useState<Run[]>([])
  const [reviews, setReviews] = useState<Run[]>([])
  const [selected, setSelected] = useState<string>()
  const [run, setRun] = useState<Run>()
  const [scope, setScope] = useState('change')
  const [baseline, setBaseline] = useState<string>()
  const [tool, setTool] = useState<string>()
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string>()
  const version = useRef(0)
  const mutating = useRef(false)
  const mounted = useRef(true)
  const key = refKey(change.ref)
  const tools = setup?.tools.filter((t) => !t.error) ?? []
  useEffect(() => {
    mounted.current = true
    const controller = new AbortController()
    const stamp = version.current
    api<Run[]>('/api/runs', { signal: controller.signal }).then((all) => {
      if (controller.signal.aborted || stamp !== version.current) return
      const mine = all.filter((r) => refKey(r.ref) === key && r.kind === change.kind)
      const summaries = mine.filter((r) => r.mode === 'summary')
      const completed = mine.filter((r) => !r.mode && r.review_complete)
      setRuns(summaries); setReviews(completed)
      setSelected((id) => id ?? summaries[0]?.id)
      setBaseline((id) => id ?? completed[0]?.id)
    }).catch((e: unknown) => { if (!controller.signal.aborted) setError(String(e)) })
    return () => { mounted.current = false; controller.abort() }
  }, [key, change.diff.head])
  useEffect(() => {
    if (!selected) return
    let active = true
    const controller = new AbortController()
    let timer: ReturnType<typeof setTimeout>
    const tick = async () => {
      const stamp = version.current
      try {
        const next = await api<Run>(`/api/runs/${selected}`, { signal: controller.signal })
        if (active && !mutating.current && stamp === version.current) {
          setRun(next)
          setRuns((all) => all.map((r) => r.id === next.id ? next : r))
        }
      } catch (e) { if (active) setError(e instanceof Error ? e.message : String(e)) }
      if (active) timer = setTimeout(tick, 1000)
    }
    void tick()
    return () => { active = false; controller.abort(); clearTimeout(timer) }
  }, [selected])

  const mutate = async (action: () => Promise<Run>) => {
    if (mutating.current) return
    mutating.current = true; version.current++; setBusy(true); setError(undefined)
    try {
      const next = await action()
      if (mounted.current) {
        setRun(next); setSelected(next.id)
        setRuns((all) => [next, ...all.filter((r) => r.id !== next.id)])
      }
    } catch (e) { if (mounted.current) setError(e instanceof Error ? e.message : String(e)) }
    finally { mutating.current = false; if (mounted.current) setBusy(false) }
  }
  const review = reviews.find((r) => r.id === baseline)
  const noNew = scope === 'since_review' && review?.head === change.diff.head
  const current = run?.id === selected ? run : undefined
  const head = useRunHead(current?.id, current?.head, change)
  const generating = current?.status === 'working' || current?.status === 'preparing'
  const output = current?.sessions.flatMap((s) => (s.log ?? []).filter((e) => e.kind === 'assistant').map((e) => e.text ?? '')).join('\n\n') ?? ''
  return <div class="summary-panel">
    <p class="muted">A local summary from your review tool. Nothing is posted to the forge.</p>
    <div class="toolbar">
      <label>Scope <select aria-label="Summary scope" value={scope} onChange={(e) => setScope(e.currentTarget.value)}>
        <option value="change">Whole change</option><option value="since_review" disabled={reviews.length === 0}>Since a Vuoro review</option>
      </select></label>
      {scope === 'since_review' && <label>Review <select aria-label="Review baseline" value={baseline} onChange={(e) => setBaseline(e.currentTarget.value)}>
        {reviews.map((r) => <option key={r.id} value={r.id}>{ago(r.created)} at {short(r.head)}</option>)}
      </select></label>}
      {tools.length > 0 && <label>Tool <select aria-label="Summary tool" value={tool} onChange={(e) => setTool(e.currentTarget.value)}>
        {tools.map((t) => <option key={t.name} value={t.name}>{t.tool}</option>)}
      </select></label>}
      <button class="primary" disabled={busy || tools.length === 0 || noNew || (scope === 'since_review' && !review)} onClick={() => void mutate(() => post<Run>('/api/summaries', { url: change.web_url, tool: tool ?? tools[0]?.name, review_id: scope === 'since_review' ? baseline : undefined }))}>{busy ? 'Starting…' : 'Summarise'}</button>
      {runs.length > 0 && <label class="push">History <select aria-label="Summary history" value={selected} onChange={(e) => { setRun(undefined); setSelected(e.currentTarget.value) }}>
        {runs.map((r) => <option key={r.id} value={r.id}>{ago(r.created)} · {r.summary?.scope === 'since_review' ? 'since review' : 'whole change'} · {short(r.head)} · {r.status}</option>)}
      </select></label>}
    </div>
    {reviews.length === 0 && <p class="muted">Since-review summaries need a completed Vuoro review. Older reviews without a recorded completion cannot be used. You can summarise the whole change.</p>}
    {noNew && <p class="muted">No new commits since this review.</p>}
    {tools.length === 0 && <p class="muted">No review tool is available. Saved summaries are still readable.</p>}
    {error && <p class="alert" role="alert">⚠ {error}</p>}
    {current && <section class="section">
      <div class="run-head"><StatusBadge status={current.status} /><HeadBadge state={head} label="summary" /><span class="muted">{usd(current.cost_usd)}</span>
        {generating && <button disabled={busy} class="push" onClick={() => void mutate(() => post<Run>(`/api/runs/${current.id}/sessions/s1/stop`))}>■ Stop summary</button>}
      </div>
      {current.summary && <p>Range <span class="mono" title={`${current.summary.from}..${current.summary.to}`}>{short(current.summary.from)}..{short(current.summary.to)}</span>{current.summary.scope === 'since_review' ? ' · since the selected Vuoro review' : ' · whole change'}</p>}
      {current.error && <p class="alert" role="alert">⚠ {current.error}</p>}
      {current.status === 'cancelled' && <p class="muted">Cancelled. Partial output is retained below.</p>}
      <div aria-live="polite" aria-busy={generating} class="summary-output">{output ? <Prose text={output} /> : <p class="muted">{generating ? 'Preparing the exact range and waiting for summary text…' : 'No summary text was produced.'}</p>}</div>
    </section>}
  </div>
}
