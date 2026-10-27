// Agent triage over a project (D65). An agent drafts proposals for reviewers,
// labels and what would unblock each change. The person edits, accepts or
// discards them, and vuoro, not the agent, makes the accepted writes.
import { useRef, useState } from 'preact/hooks'
import { errorText } from './action'
import { api, post } from './api'
import { Modal } from './modal'
import { usePoll } from './poll'
import type { ProjectRow, ProjectView } from './projects'
import { SessionPane } from './session'
import { changeNumber, type Draft, type Run, type Setup } from './types'
import { ago, StatusBadge } from './ui'

interface Outcome { id: string; result: 'accepted' | 'unchanged' | 'stale' | 'failed'; error?: string }
interface Edit { reviewers?: string[]; labels?: string[] }

const live = (run?: Run | null) => run?.status === 'working' || run?.status === 'preparing'

export function ProjectTriage({ projectKey, kind, rows, onView }: {
  projectKey: string; kind: string; rows: ProjectRow[]; onView: (v: ProjectView) => void
}) {
  const [run, setRun] = useState<Run | null>()
  const [tools, setTools] = useState<Setup['tools']>()
  const [tool, setTool] = useState<string>()
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string>()
  const [picks, setPicks] = useState<Record<string, boolean>>({})
  const [edits, setEdits] = useState<Record<string, Edit>>({})
  const [outcomes, setOutcomes] = useState<Record<string, Outcome>>({})
  const [asking, setAsking] = useState(false)
  const cancel = useRef<HTMLButtonElement>(null)

  usePoll(async (signal) => {
    if (!tools) api<Setup>('/api/setup', { signal }).then((s) => setTools(s.tools.filter((t) => !t.error))).catch(() => setTools([]))
    const next = await api<Run | null>(`/api/projects/${projectKey}/triage`, { signal })
    if (signal.aborted) return undefined
    if (!busy) setRun(next)
    return live(next) ? 1500 : undefined
  }, [projectKey, run?.id, live(run)])

  const act = async (f: () => Promise<void>) => {
    setBusy(true)
    setError(undefined)
    try { await f() } catch (e) { setError(errorText(e)) } finally { setBusy(false) }
  }
  const triageable = rows.filter((r) => r.place !== 'draft' && !r.error)
  const start = () => act(async () => {
    setOutcomes({}); setPicks({}); setEdits({})
    setRun(await post<Run>(`/api/projects/${projectKey}/triage`, { numbers: triageable.map((r) => r.ref.number), tool: tool ?? tools?.[0]?.name }))
  })
  const proposals = (run?.drafts ?? []).filter((d) => d.status === 'pending' && d.change)
  const open = (d: Draft) => !d.decision
  const reviewersOf = (d: Draft) => edits[d.id]?.reviewers ?? d.reviewers ?? []
  const labelsOf = (d: Draft) => edits[d.id]?.labels ?? d.labels ?? []
  const chosen = proposals.filter((d) => open(d) && picks[d.id])
  const row = (n?: number) => rows.find((r) => r.ref.number === n)
  const describe = (d: Draft) => {
    const parts = []
    if (reviewersOf(d).length) parts.push(`add reviewers ${reviewersOf(d).join(', ')}`)
    if (labelsOf(d).length) parts.push(`add labels ${labelsOf(d).join(', ')}`)
    return `${changeNumber(kind, d.change!)}: ${parts.join('; ') || 'nothing to write'}`
  }
  const accept = () => act(async () => {
    const res = await post<{ view: ProjectView; run: Run; outcomes: Outcome[] }>(`/api/projects/${projectKey}/triage/${run!.id}/accept`, {
      proposals: chosen.map((d) => ({ id: d.id, ...edits[d.id], stale: outcomes[d.id]?.result === 'stale' || undefined })),
    })
    setRun(res.run)
    onView(res.view)
    setOutcomes((o) => ({ ...o, ...Object.fromEntries(res.outcomes.map((x) => [x.id, x])) }))
    setPicks({})
  })
  const discard = (d: Draft) => act(async () => { setRun(await post<Run>(`/api/projects/${projectKey}/triage/${run!.id}/discard`, { id: d.id })) })
  const drop = (d: Draft, field: 'reviewers' | 'labels', value: string) =>
    setEdits((e) => ({ ...e, [d.id]: { ...e[d.id], [field]: (field === 'reviewers' ? reviewersOf(d) : labelsOf(d)).filter((v) => v !== value) } }))

  return (
    <details class="project-triage" open={!!run}>
      <summary>Triage with an agent{run && <span class="muted"> · {proposals.filter(open).length} proposals waiting · {ago(run.created)}</span>}</summary>
      <p class="muted">The agent proposes reviewers, labels and what would unblock each change. Nothing reaches the forge until you accept it.</p>
      <div class="toolbar">
        {(tools?.length ?? 0) > 1 && <label>Agent <select value={tool ?? tools![0].name} disabled={busy} onChange={(e) => setTool(e.currentTarget.value)}>
          {tools!.map((t) => <option key={t.name} value={t.name}>{t.tool}</option>)}</select></label>}
        <button class="primary" disabled={busy || !tools?.length || triageable.length === 0 || live(run)} onClick={start}>
          {live(run) ? 'The agent is working…' : `${run ? 'Triage again' : 'Triage'} ${triageable.length} shown`}</button>
        {chosen.length > 0 && <button disabled={busy} onClick={() => setAsking(true)}>Accept {chosen.length} selected</button>}
      </div>
      {tools?.length === 0 && <p class="muted">No agent tool is available.</p>}
      {error && <p class="alert" role="alert">{error}</p>}
      {run && <div class="triage-run"><StatusBadge status={run.status} />{run.error && <p class="alert">{run.error}</p>}
        <details><summary>Talk to the agent · progress and reasoning</summary>{run.sessions.map((s) => <SessionPane run={run} session={s} key={s.id} />)}</details>
      </div>}
      {run && !live(run) && proposals.length === 0 && <p class="muted">No proposals.</p>}
      <ul class="project-proposals">
        {proposals.map((d) => {
          const r = row(d.change)
          const o = outcomes[d.id]
          return (
            <li key={d.id} class={`project-proposal ${d.decision ?? ''}`}>
              <div class="project-proposal-head">
                {open(d) && <input type="checkbox" aria-label={`Select the proposal for ${changeNumber(kind, d.change!)}`} checked={!!picks[d.id]} disabled={busy}
                  onChange={(e) => setPicks((p) => ({ ...p, [d.id]: e.currentTarget.checked }))} />}
                <strong>{changeNumber(kind, d.change!)}</strong> {r?.title ?? <span class="muted">no longer listed</span>}
                <span class="badge">Agent suggestion</span>
                {d.decision === 'kept' && <span class="badge">✓ accepted</span>}
                {d.decision === 'dropped' && <span class="badge">✕ discarded</span>}
              </div>
              {reviewersOf(d).length > 0 && <p><span class="muted">Reviewers:</span> {reviewersOf(d).map((u) => <span key={u} class="badge">{u}
                {open(d) && <button class="link remove" aria-label={`Leave out ${u}`} onClick={() => drop(d, 'reviewers', u)}>✕</button>}</span>)}</p>}
              {labelsOf(d).length > 0 && <p><span class="muted">Labels:</span> {labelsOf(d).map((l) => <span key={l} class="badge">{l}
                {open(d) && <button class="link remove" aria-label={`Leave out ${l}`} onClick={() => drop(d, 'labels', l)}>✕</button>}</span>)}</p>}
              {d.unblock && <p><span class="muted">To unblock:</span> {d.unblock}</p>}
              <p class="muted">{d.reason}</p>
              {o?.result === 'stale' && open(d) && <p class="inbox-stale" role="status">⚠ The change moved since the agent read it. Check it, then select and accept again to apply anyway.</p>}
              {o?.result === 'unchanged' && <p class="muted" role="status">Nothing to add: the change already has these.</p>}
              {o?.result === 'failed' && <p class="alert" role="alert">{o.error}</p>}
              {open(d) && <button class="link" disabled={busy} onClick={() => discard(d)}>Discard</button>}
            </li>
          )
        })}
      </ul>
      {asking && <Modal title={`Accept ${chosen.length} ${chosen.length === 1 ? 'proposal' : 'proposals'}?`} onClose={() => setAsking(false)} focus={cancel}>
        <p>vuoro makes these changes on the forge, as you. Others see them.</p>
        <ul>{chosen.map((d) => <li key={d.id}>{describe(d)}{outcomes[d.id]?.result === 'stale' ? ' (the change moved since)' : ''}</li>)}</ul>
        <div class="actions">
          <button ref={cancel} onClick={() => setAsking(false)}>Cancel</button>
          <button class="primary" onClick={() => { setAsking(false); void accept() }}>Accept</button>
        </div>
      </Modal>}
    </details>
  )
}
