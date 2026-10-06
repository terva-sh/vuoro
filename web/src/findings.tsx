// The findings of a run, each opened in place with its marks, and the
// hand-off report of the kept ones.
import { useState } from 'preact/hooks'
import { post } from './api'
import { DraftCard } from './comments'
import { HeadBadge, useHead } from './head'
import type { Draft, Level, Run } from './types'
import { ConfidenceBadge, levelMark, SeverityBadge, severityOrder, short } from './ui'

// shownDrafts are the drafts the person works from: the synthesizer's once
// it has proposed any, and otherwise every reviewer's.
export function shownDrafts(run: Run): Draft[] {
  const synth = new Set(run.sessions.filter((s) => s.role === 'synthesizer').map((s) => s.id))
  const pending = run.drafts.filter((d) => d.status === 'pending')
  const merged = pending.filter((d) => synth.has(d.session))
  return merged.length > 0 ? merged : pending
}

// sourceLabel says where a merged draft came from. A finding that only one
// reviewer raised is kept and says so, rather than being voted away.
export function sourceLabel(d: Draft, byID: Map<string, Draft>, profile: Map<string, string>): string {
  const srcs = (d.sources ?? []).map((id) => byID.get(id)).filter((x): x is Draft => !!x)
  const who = [...new Set(srcs.map((x) => profile.get(x.session) ?? x.session))]
  const list = `from ${(d.sources ?? []).join(', ')}`
  return who.length === 1 ? `◇ raised by one reviewer, ${who[0]}, ${list}` : `merged ${list} (${who.join(', ')})`
}


// Pick is how a review of the person's own work marks findings for the
// editor.
interface Pick { isPicked: (d: Draft) => boolean; pickable: (d: Draft) => boolean; set: (d: Draft, on: boolean) => void }

export function Drafts({ run, onPick, onRun, pick: picker }: { run: Run; onPick: (path: string) => void; onRun: (r: Run) => void; pick?: Pick }) {
  const byID = new Map(run.drafts.map((d) => [d.id, d]))
  const profile = new Map(run.sessions.map((s) => [s.id, s.profile]))
  const shown = [...shownDrafts(run)].sort((a, b) => severityOrder.indexOf(a.severity) - severityOrder.indexOf(b.severity))
  const shownIDs = new Set(shown.map((d) => d.id))
  const cited = new Set(shown.flatMap((d) => d.sources ?? []))
  // What the synthesizer did not carry forward, and what agents withdrew.
  const rest = run.drafts.filter((d) => !shownIDs.has(d.id) && !cited.has(d.id))
  const pick = (d: Draft) => (d.anchor ? () => onPick(d.anchor!.path) : undefined)

  return (
    <>
      <div class="findings">
        {shown.map((d) => (
          <Finding draft={d} key={d.id} runID={run.id} onPick={pick(d)} onRun={onRun} picker={picker}
            checking={run.sessions.some((s) => s.role === 'verifier' && (s.status === 'waiting' || s.status === 'working' || s.status === 'preparing'))}
            from={d.sources?.length ? sourceLabel(d, byID, profile) : `${profile.get(d.session) ?? d.session} reviewer`}
            sources={(d.sources ?? []).map((id) => byID.get(id)).filter((x): x is Draft => !!x)}
            profile={profile} />
        ))}
      </div>
      {rest.length > 0 && (
        <details class="not-carried">
          <summary>Not carried forward or withdrawn: {rest.length}</summary>
          {rest.map((d) => (
            <DraftCard draft={d} key={d.id} onPick={pick(d)}
              label={`${profile.get(d.session) ?? d.session} · ${d.status === 'withdrawn' ? 'withdrawn by its agent' : 'set aside by the synthesizer, whose reply says why'}`} />
          ))}
        </details>
      )}
    </>
  )
}

// Finding is one row of the findings list, opened in place.
function Finding({ draft: d, runID, onPick, onRun, checking, from, sources, profile, picker }: {
  draft: Draft; runID: string; onPick?: () => void; onRun: (r: Run) => void; checking: boolean
  from: string; sources: Draft[]; profile: Map<string, string>; picker?: Pick
}) {
  const [error, setError] = useState<string>()
  const dismiss = () => post<Run>(`/api/runs/${runID}/drafts/${d.id}/dismiss`).then(onRun, (e: unknown) => setError(e instanceof Error ? e.message : String(e)))
  const mark = d.override ?? d.confidence
  const a = d.anchor
  const decided = d.fixed_in ? `✓ fixed in ${short(d.fixed_in)}` : d.decision === 'kept' ? '✓ kept' : d.decision === 'dropped' ? (picker ? '✗ dismissed' : '✗ dropped') : ''
  return (
    <div class="finding-row">
      {/* Outside the summary, whose click would swallow the checkbox's. */}
      {picker?.pickable(d) && (
        <label class="pick" title="Hand this finding to the editor">
          <input type="checkbox" checked={picker.isPicked(d)} onChange={(e) => picker.set(d, e.currentTarget.checked)} /> fix
        </label>
      )}
      <details class={`finding ${d.decision ?? 'undecided'}`}>
        <summary>
          <SeverityBadge severity={d.severity} />
          {mark ? <ConfidenceBadge c={mark} mine={!!d.override} /> : <span class="badge muted">{checking ? '⧗ checking' : '· not checked'}</span>}
          <span class="finding-title">{d.title}</span>
          {a && <span class="mono muted finding-place">{a.path.split('/').pop()}:{a.start_line}</span>}
          {decided && <span class="badge">{decided}</span>}
          <HeadBadge state={useHead()} />
        </summary>
        <div class="finding-body">
          <Marks draft={d} runID={runID} onRun={onRun} checking={checking} />
          {picker?.pickable(d) && (
            <div class="actions">
              <button onClick={() => void dismiss()} title="Sets it aside here. Nothing is posted.">Dismiss</button>
            </div>
          )}
          {error && <p class="alert" role="alert">⚠ {error}</p>}
          <DraftCard draft={d} runID={runID} onPick={onPick} label={from} compact />
          {sources.length > 1 && (
            <details class="sources">
              <summary>What each reviewer wrote: {sources.length}</summary>
              {sources.map((s) => <DraftCard draft={s} key={s.id} label={`${profile.get(s.session) ?? s.session} reviewer, ${s.id}`} />)}
            </details>
          )}
        </div>
      </details>
    </div>
  )
}

// HandOff copies the report of kept findings for an agent that fixes them,
// which vuoro also writes into the run's directory (D31).
export function HandOff({ run, kept }: { run: Run; kept: number }) {
  const [done, setDone] = useState<{ path: string; copied: boolean; markdown: string }>()
  const [error, setError] = useState<string>()
  const make = async () => {
    setError(undefined)
    try {
      const r = await post<{ markdown: string; path: string }>(`/api/runs/${run.id}/report`)
      // The clipboard can refuse, so the text is shown to copy by hand.
      const copied = await navigator.clipboard?.writeText(r.markdown).then(() => true, () => false) ?? false
      setDone({ ...r, copied })
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e))
    }
  }
  return (
    <div class="handoff">
      <button disabled={kept === 0} onClick={() => void make()}
        title={kept === 0 ? 'Keep a finding first' : 'A Markdown brief of the kept findings, for an agent that fixes them'}>
        ⧉ Copy the hand-off report ({kept} kept)
      </button>
      {done && (
        <span class="muted">
          {done.copied ? '✓ Copied, and written' : 'Written'} to <span class="mono">{done.path}</span>
        </span>
      )}
      {done && !done.copied && <textarea class="report-text" readOnly value={done.markdown} onFocus={(e) => (e.target as HTMLTextAreaElement).select()} />}
      {error && <p class="alert" role="alert">⚠ {error}</p>}
    </div>
  )
}

// Marks shows the verifier's mark and its reason, and lets the person set
// their own, which wins and stays beside the verifier's.
function Marks({ draft: d, runID, onRun, checking }: { draft: Draft; runID: string; onRun: (r: Run) => void; checking: boolean }) {
  const [error, setError] = useState<string>()
  const set = (level: Level | '') =>
    post<Run>(`/api/runs/${runID}/drafts/${d.id}/confidence`, { level }).then(onRun, (e: unknown) => setError(e instanceof Error ? e.message : String(e)))
  return (
    <div class="marks">
      <div>
        <span class="muted">Verifier: </span>
        {d.confidence ? (
          <><ConfidenceBadge c={d.confidence} /> {d.confidence.reason}</>
        ) : (
          <span class="muted">{checking ? 'not reached yet' : 'no mark'}</span>
        )}
      </div>
      <div class="mark-buttons">
        <span class="muted">Your mark: </span>
        {(['confirmed', 'plausible', 'doubtful'] as Level[]).map((l) => (
          <button key={l} aria-pressed={d.override?.level === l} class={d.override?.level === l ? 'on' : ''}
            onClick={() => void set(d.override?.level === l ? '' : l)}>{levelMark[l]} {l}</button>
        ))}
        {d.override && <button class="link" onClick={() => void set('')}>use the verifier's</button>}
      </div>
      {error && <p class="alert" role="alert">⚠ {error}</p>}
    </div>
  )
}
