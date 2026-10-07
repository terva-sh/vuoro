// The findings of a run, each opened in place with its marks, and the
// hand-off report of the kept ones.
import { useContext, useState } from 'preact/hooks'
import { errorText, useAction } from './action'
import { post } from './api'
import { DraftCard } from './comments'
import { HeadBadge, useHead } from './head'
import { commitURL, RefsContext } from './markdown'
import { SavedPatch } from './patch'
import { TextEditor } from './textedit'
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
  // Fixed findings fold away, so the open ones come first (D50).
  const open = shown.filter((d) => !d.fixed_in)
  const fixed = shown.filter((d) => d.fixed_in)
  const shownIDs = new Set(shown.map((d) => d.id))
  const cited = new Set(shown.flatMap((d) => d.sources ?? []))
  // What the synthesizer did not carry forward, and what agents withdrew.
  const rest = run.drafts.filter((d) => !shownIDs.has(d.id) && !cited.has(d.id))
  const pick = (d: Draft) => (d.anchor ? () => onPick(d.anchor!.path) : undefined)
  const checking = run.sessions.some((s) => s.role === 'verifier' && (s.status === 'waiting' || s.status === 'working' || s.status === 'preparing'))
  const finding = (d: Draft) => (
    <Finding draft={d} key={d.id} runID={run.id} url={run.web_url} onPick={pick(d)} onRun={onRun} picker={picker} checking={checking}
      from={d.sources?.length ? sourceLabel(d, byID, profile) : `${profile.get(d.session) ?? d.session} reviewer`}
      sources={(d.sources ?? []).map((id) => byID.get(id)).filter((x): x is Draft => !!x)}
      profile={profile} />
  )

  return (
    <>
      <div class="findings">{open.map(finding)}</div>
      {fixed.length > 0 && (
        <details class="fixed-findings">
          <summary>Fixed · {fixed.length}</summary>
          <div class="findings">{fixed.map(finding)}</div>
        </details>
      )}
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
function Finding({ draft: d, runID, url, onPick, onRun, checking, from, sources, profile, picker }: {
  draft: Draft; runID: string; url: string; onPick?: () => void; onRun: (r: Run) => void; checking: boolean
  from: string; sources: Draft[]; profile: Map<string, string>; picker?: Pick
}) {
  const [error, setError] = useState<string>()
  const head = useHead()
  const dismiss = () => post<Run>(`/api/runs/${runID}/drafts/${d.id}/dismiss`).then(onRun, (e: unknown) => setError(errorText(e)))
  const mark = d.override ?? d.confidence
  const a = d.anchor
  const decided = (d.fixed_in ? `✓ fixed in ${short(d.fixed_in)}` : d.decision === 'kept' ? '✓ kept' : d.decision === 'dropped' ? (picker ? '✗ dismissed' : '✗ dropped') : '')
    + (d.carried && !d.fixed_in ? ' earlier' : '')
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
          {d.earlier && <span class="badge muted" title="It restates a finding of the review this one follows">↺ earlier {d.earlier}</span>}
          {/* A fixed finding's lines are the fix's business, not the reviewed head's. */}
          {!d.fixed_in && <HeadBadge state={head} />}
        </summary>
        <div class="finding-body">
          <Marks draft={d} runID={runID} onRun={onRun} checking={checking} />
          {picker?.pickable(d) && (
            <div class="actions">
              <button onClick={() => void dismiss()} title="Sets it aside here. Nothing is posted.">Dismiss</button>
            </div>
          )}
          {error && <p class="alert" role="alert">⚠ {error}</p>}
          {d.decision !== 'dropped' && <FixLink draft={d} runID={runID} url={url} onRun={onRun} />}
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

interface FixCommit { sha: string; subject: string }

// FixLink says which commit fixed a finding, and who linked it (D50). The
// person can link one by hand, from the commits vuoro pushed for the review
// and the change's since it was reviewed, or take back a link they made.
// Nothing reaches the forge.
export function FixLink({ draft: d, runID, url, onRun }: { draft: Draft; runID: string; url: string; onRun: (r: Run) => void }) {
  const refs = useContext(RefsContext)
  const [commits, setCommits] = useState<FixCommit[]>()
  const [sha, setSHA] = useState('')
  const act = useAction()
  const mark = (to: string) => act.run(async () => {
    onRun(await post<Run>(`/api/runs/${runID}/drafts/${d.id}/fixed`, { url, sha: to }))
    setCommits(undefined)
  })
  const link = d.fixed_in ? commitURL(refs, d.fixed_in) : undefined
  if (d.fixed_in) {
    return (
      <p class="fix-link">
        ✓ Fixed in {link ? <a class="mono" href={link} target="_blank" rel="noreferrer">{short(d.fixed_in)}</a> : <span class="mono">{short(d.fixed_in)}</span>}
        {d.fixed_subject && <> · {d.fixed_subject}</>}
        <span class="muted"> · {d.fixed_by === 'person' ? 'linked by you' : "named in the commit's trailer"}</span>
        <SavedPatch sha={d.fixed_in} />
        {d.fixed_by === 'person' && <> <button class="link" disabled={act.busy} onClick={() => void mark('')}>Not fixed</button></>}
        {act.error && <span class="alert" role="alert"> ⚠ {act.error}</span>}
      </p>
    )
  }
  const open = () => act.run(async () => {
    const cs = await post<FixCommit[]>(`/api/runs/${runID}/fixes`, { url })
    setCommits(cs)
    setSHA(cs[cs.length - 1]?.sha ?? '')
  })
  return (
    <div class="fix-link">
      {!commits ? (
        <button disabled={act.busy} onClick={() => void open()} title="Link this finding to the commit that fixed it. Nothing is posted.">
          {act.busy ? '⧗ Reading commits…' : 'Mark fixed…'}
        </button>
      ) : commits.length === 0 ? (
        <span class="muted">No commit since the review to link. <button class="link" onClick={() => setCommits(undefined)}>Close</button></span>
      ) : (
        <span class="fix-pick">
          <label>
            Fixed by{' '}
            <select value={sha} onChange={(e) => setSHA((e.target as HTMLSelectElement).value)}>
              {commits.map((c) => <option value={c.sha} key={c.sha}>{short(c.sha)} {c.subject}</option>)}
            </select>
          </label>
          <button class="primary" disabled={act.busy || !sha} onClick={() => void mark(sha)}>Mark fixed</button>
          <button onClick={() => setCommits(undefined)}>Cancel</button>
        </span>
      )}
      {act.error && <p class="alert" role="alert">⚠ {act.error}</p>}
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
      setError(errorText(e))
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
      {done && !done.copied && <TextEditor rows={12} mono readOnly value={done.markdown} label="The hand-off report" />}
      {error && <p class="alert" role="alert">⚠ {error}</p>}
    </div>
  )
}

// Marks shows the verifier's mark and its reason, and lets the person set
// their own, which wins and stays beside the verifier's.
function Marks({ draft: d, runID, onRun, checking }: { draft: Draft; runID: string; onRun: (r: Run) => void; checking: boolean }) {
  const [error, setError] = useState<string>()
  const set = (level: Level | '') =>
    post<Run>(`/api/runs/${runID}/drafts/${d.id}/confidence`, { level }).then(onRun, (e: unknown) => setError(errorText(e)))
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
