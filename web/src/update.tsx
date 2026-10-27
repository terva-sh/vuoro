// Updating a change from its target (section 6.10, D66, D67): vuoro merges
// or rebases, an editor resolves each conflicting stop, the person accepts
// each resolution, and the push always asks first.
import { useEffect, useRef, useState } from 'preact/hooks'
import { errorText, useAction } from './action'
import { api, post } from './api'
import { DiffBrowser, useFilePicker } from './diffview'
import { Modal } from './modal'
import { usePoll } from './poll'
import { SessionPane } from './session'
import { TextEditor } from './textedit'
import { changeWord, refKey, type Capabilities, type Change, type FileDiff, type Run, type Setup, type Stop, type UpdateDetail } from './types'
import { ago, short, StatusBadge } from './ui'
import { WriteContext } from './write'

type Props = { change: Change; setup?: Setup; own: boolean }

const forgeName = (kind: string) => ({ forgejo: 'Forgejo', github: 'GitHub' })[kind] ?? 'GitLab'

export function UpdatePanel(props: Props) {
  return <UpdateWorkspace {...props} key={refKey(props.change.ref)} />
}

function UpdateWorkspace({ change, setup, own }: Props) {
  const [runs, setRuns] = useState<Run[]>([])
  const [selected, setSelected] = useState<string>()
  const [run, setRun] = useState<Run>()
  const [strategy, setStrategy] = useState<'merge' | 'rebase'>('merge')
  const [tool, setTool] = useState<string>()
  const { busy, error, setError, run: perform } = useAction()
  const tools = setup?.tools.filter((t) => !t.error) ?? []
  const url = change.web_url

  useEffect(() => {
    const controller = new AbortController()
    api<Run[]>('/api/changes/updates', { method: 'POST', body: JSON.stringify({ url }), signal: controller.signal }).then((all) => {
      if (controller.signal.aborted) return
      setRuns(all)
      setSelected((id) => id ?? all[0]?.id)
    }).catch((e: unknown) => { if (!controller.signal.aborted) setError(errorText(e)) })
    return () => controller.abort()
  }, [url])

  usePoll(async (signal) => {
    if (!selected) return undefined
    try {
      const next = await api<Run>(`/api/runs/${selected}`, { signal })
      if (!signal.aborted) setRun(next)
    } catch (e) { if (!signal.aborted) setError(errorText(e)) }
    return 1500
  }, [selected])

  const show = (r: Run) => { setRun(r); setSelected(r.id); setRuns((all) => [r, ...all.filter((x) => x.id !== r.id)]) }
  const current = run?.id === selected ? run : undefined
  const open = !!current?.update && !current.update.pushed && !current.update.discarded && current.status !== 'failed' && current.status !== 'closed'
  const start = () => perform(async () => show(await post<Run>('/api/changes/update', { url, strategy, tool: tool ?? tools[0]?.name })))
  const why = change.state !== 'open' ? `Only an open ${changeWord(change.kind)} can be updated`
    : tools.length === 0 ? 'No agent tool is available to resolve conflicts'
    : open ? 'Finish or discard the update below first' : ''

  return (
    <section class="update-panel" aria-label="Update from the target">
      <div class="toolbar">
        <div>
          <h2>Update from <span class="mono">{change.target_branch}</span></h2>
          <p class="muted">vuoro brings in what <span class="mono">{change.target_branch}</span> gained. If anything conflicts, an agent resolves it in a sandbox and you accept each resolution. Nothing is pushed until you confirm.</p>
        </div>
        <fieldset class="update-strategy">
          <legend>Strategy</legend>
          <label><input type="radio" name="update-strategy" checked={strategy === 'merge'} onChange={() => setStrategy('merge')} /> Merge</label>
          <label title={own ? 'Replays your commits onto the target and rewrites the branch, pushed with a lease' : 'A rebase rewrites the branch, so vuoro rebases only your own change'}>
            <input type="radio" name="update-strategy" checked={strategy === 'rebase'} disabled={!own} onChange={() => setStrategy('rebase')} /> Rebase
          </label>
        </fieldset>
        {tools.length > 1 && <label>Agent <select value={tool ?? tools[0]?.name} disabled={busy} onChange={(e) => setTool(e.currentTarget.value)}>{tools.map((t) => <option key={t.name} value={t.name}>{t.tool}</option>)}</select></label>}
        <button class="primary" disabled={busy || !!why} title={why} onClick={() => void start()}>{busy ? '⧗ Starting…' : strategy === 'merge' ? `Merge ${change.target_branch} in` : `Rebase onto ${change.target_branch}`}</button>
        {runs.length > 1 && <label>Update <select value={selected} onChange={(e) => { setSelected(e.currentTarget.value); setRun(undefined) }}>{runs.map((r) => <option key={r.id} value={r.id}>{r.update?.strategy} {ago(r.created)} at {short(r.head)}</option>)}</select></label>}
      </div>
      {!own && <p class="muted">This is not your {changeWord(change.kind)}, so a merge goes up as a draft {changeWord(change.kind)} onto <span class="mono">{change.source_branch}</span> for its author.</p>}
      {error && <p class="alert" role="alert">⚠ {error}</p>}
      {current?.update && <UpdateView run={current} url={url} onRun={show} caps={change.capabilities} />}
    </section>
  )
}

// UpdateView is one update: its stops, each with the editor's resolution,
// and once nothing is left to resolve, what the push would carry.
export function UpdateView({ run, url, onRun, caps }: { run: Run; url: string; onRun: (r: Run) => void; caps?: Capabilities }) {
  const u = run.update!
  const [detail, setDetail] = useState<UpdateDetail>()
  const [discarding, setDiscarding] = useState(false)
  const [pushing, setPushing] = useState(false)
  const { busy, error, setError, run: perform } = useAction()
  const sessions = new Map(run.sessions.map((s) => [s.id, s]))
  const last = u.stops.at(-1)
  const session = last?.session ? sessions.get(last.session) : undefined
  const working = session?.status === 'working' || session?.status === 'preparing'
  // The detail is read again whenever the update or its editor's turn moves.
  const stamp = [run.status, u.stops.length, last?.accepted, u.result, u.pushed?.sha, u.discarded, session?.status, session?.log?.length].join('|')
  useEffect(() => {
    if (run.status === 'preparing') return
    api<UpdateDetail>(`/api/runs/${run.id}/update`).then(setDetail, (e: unknown) => setError(errorText(e)))
  }, [run.id, stamp])
  const act = (path: string) => perform(async () => {
    onRun(await post<Run>(path))
    setDiscarding(false)
  })
  const state = detail?.state ?? ''
  const done = !!u.pushed || !!u.discarded || run.status === 'closed'
  const word = changeWord(run.kind)
  // A clean update of the person's own change goes through the forge's own
  // branch update, when it has the strategy (section 6.10).
  const viaForge = u.own && u.stops.length === 0 && !!caps?.update_branch?.includes(u.strategy)
  return (
    <div class="update-run">
      <header class="update-head">
        <strong>{u.strategy === 'merge' ? `Merge ${u.target} into ${u.source}` : `Rebase ${u.source} onto ${u.target}`}</strong>
        <StatusBadge status={run.status} />
        <span class="muted">from <span class="mono">{short(run.head)}</span>{u.target_sha && <> and <span class="mono">{u.target}</span> at <span class="mono">{short(u.target_sha)}</span></>}</span>
        {!done && (discarding ? (
          <span class="push">
            Delete the update and its work?{' '}
            <button class="primary" disabled={busy} onClick={() => void act(`/api/runs/${run.id}/update/discard`)}>Discard</button>{' '}
            <button class="link" onClick={() => setDiscarding(false)}>Keep it</button>
          </span>
        ) : <button class="push" disabled={busy} onClick={() => setDiscarding(true)}>Discard…</button>)}
      </header>
      {run.status === 'preparing' && <p class="muted">⧗ Fetching the {word} and {u.target}…</p>}
      {run.error && <p class="alert" role="alert">⚠ {run.error}</p>}
      {u.discarded && <p class="muted">Discarded. Nothing was pushed.</p>}
      {u.stops.length === 0 && state === 'ready' && <p>✓ No conflicts. vuoro {u.strategy === 'merge' ? 'merged' : 'rebased'} it without an agent.</p>}
      {u.remaining && u.remaining.length > 0 && !done && <p class="muted">{u.remaining.length} commit{u.remaining.length === 1 ? '' : 's'} still to replay after this stop.</p>}
      {(u.dropped?.length ?? 0) > 0 && <p class="muted">{u.dropped!.length} commit{u.dropped!.length === 1 ? '' : 's'} dropped, because {u.target} already held {u.dropped!.length === 1 ? 'its change' : 'their changes'}.</p>}
      <ol class="update-stops">
        {u.stops.map((s, i) => (
          <StopView key={s.seed} run={run} stop={s} index={i} files={detail?.resolutions?.[i] ?? []} working={!s.accepted && working}
            busy={busy} onAccept={() => void act(`/api/runs/${run.id}/update/accept`)} />
        ))}
      </ol>
      {state === 'ready' && detail && (
        <div class="push-bar">
          <h4>To push · {detail.commits.length} commit{detail.commits.length === 1 ? '' : 's'}</h4>
          <ol class="editor-commits">
            {detail.commits.map((c) => <li key={c.sha}><span class="commit-sha">{short(c.sha)}</span><div class="commit-body"><div class="commit-subject">{c.subject}</div></div></li>)}
          </ol>
          <button class="primary" disabled={busy} onClick={() => setPushing(true)}>
            {viaForge ? `⇪ Ask ${forgeName(run.kind)} to ${u.strategy} ${u.source}…` : u.own ? (u.strategy === 'rebase' ? `⇪ Push to ${u.source} with a lease…` : `⇪ Push to ${u.source}…`) : `⇪ Push and open a draft ${word}…`}
          </button>
        </div>
      )}
      {u.pushed && (
        <p class="push-bar">
          {u.pushed.forge ? <>✓ {forgeName(run.kind)} is updating <span class="mono">{u.pushed.branch}</span> itself. Its commits show on the {word} once it finishes.</>
            : <>✓ Pushed <span class="mono">{u.pushed.branch}</span> at <span class="mono">{short(u.pushed.sha)}</span></>}
          {!u.own && (u.pushed.url ? <> · <a href={u.pushed.url} target="_blank" rel="noreferrer">the {u.pushed.not_draft ? '' : 'draft '}{word}</a></> : ` · the ${word} did not open`)}
          {u.pushed.not_draft && <span class="warn"> · ⚠ It opened as an ordinary {word}, not a draft. Mark it as a draft on the forge.</span>}
        </p>
      )}
      {error && <p class="alert" role="alert">⚠ {error}</p>}
      {pushing && detail?.result && <PushUpdateDialog run={run} url={url} sha={detail.result} commits={detail.commits.length} viaForge={viaForge}
        onClose={() => setPushing(false)} onRun={(r) => { onRun(r); setPushing(false) }} />}
    </div>
  )
}

// StopView is one conflicting stop: the files git could not merge, the
// editor's session to follow or talk to, and its resolution as a diff from
// the conflicted tree, so each marker it removed shows beside what it chose.
function StopView({ run, stop: s, index, files, working, busy, onAccept }: {
  run: Run; stop: Stop; index: number; files: FileDiff[]; working: boolean; busy: boolean; onAccept: () => void
}) {
  const prefix = `stop-${index}-`
  const { picked, pick } = useFilePicker(prefix)
  const session = s.session ? run.sessions.find((x) => x.id === s.session) : undefined
  const open = !s.accepted && !run.update?.discarded && run.status !== 'closed'
  return (
    <li class="update-stop">
      <header class="update-stop-head">
        <strong>Stop {index + 1}</strong>
        {s.commit && <span class="mono">{short(s.commit)}</span>}
        <span>{s.subject}</span>
        {s.accepted ? <span class="badge">✓ accepted</span> : session && <StatusBadge status={session.status} />}
      </header>
      <p>Conflicts in <span class="mono">{s.conflicted.join(', ')}</span></p>
      {(s.outside?.length ?? 0) > 0 && <p class="warn">⚠ The editor also changed <span class="mono">{s.outside!.join(', ')}</span>, outside the conflict. Its session says why.</p>}
      {session && <details><summary>Talk to the editor · its account of each file</summary><SessionPane run={run} session={session} /></details>}
      {working && <p class="muted">⧗ The editor is resolving the conflict.</p>}
      {files.length > 0 && (
        <WriteContext.Provider value={undefined}>
          <DiffBrowser files={files} picked={picked} onPick={pick} idPrefix={prefix} />
        </WriteContext.Provider>
      )}
      {open && !working && (
        <div class="actions">
          <button class="primary" disabled={busy || files.length === 0} title={files.length === 0 ? 'The editor has committed nothing yet' : 'vuoro checks every marker is gone, then writes the commit itself'}
            onClick={onAccept}>{busy ? '⧗ Accepting…' : 'Accept this resolution'}</button>
        </div>
      )}
    </li>
  )
}

// PushUpdateDialog is the one confirmation before an update is pushed. It
// names the branch and the strategy, and for a rebase that it rewrites the
// branch with a lease. Cancel has the focus, so a stray Enter pushes nothing.
export function PushUpdateDialog({ run, url, sha, commits, viaForge = false, onClose, onRun }: {
  run: Run; url: string; sha: string; commits: number; viaForge?: boolean; onClose: () => void; onRun: (r: Run) => void
}) {
  const u = run.update!
  const word = changeWord(run.kind)
  const [title, setTitle] = useState(`Merge ${u.target} into ${u.source}`)
  const [description, setDescription] = useState('')
  const { busy, error, run: perform } = useAction()
  const cancel = useRef<HTMLButtonElement>(null)
  const branch = u.own ? u.source : `vuoro/update-${run.ref.number}-${run.id.slice(-6)}`
  const push = () => perform(async () => onRun(await post<Run>(`/api/runs/${run.id}/update/push`, { url, confirm: 'push', sha, title, description })))
  return (
    <Modal class="push-dialog" onClose={onClose} focus={cancel} locked={busy}
      title={viaForge ? `Ask ${forgeName(run.kind)} to ${u.strategy} ${u.source}` : u.own ? `Push the ${u.strategy} to ${u.source}` : `Push the merge and open a draft ${word}`}>
      <ul>
        {viaForge ? <>
          <li>Asks {forgeName(run.kind)} to {u.strategy === 'merge' ? <>merge <span class="mono">{u.target}</span> into <span class="mono">{branch}</span></> : <>rebase <span class="mono">{branch}</span> onto <span class="mono">{u.target}</span></>}, your {word}'s branch, itself, with your token. Nothing conflicted, so no agent took part.</li>
          <li>{forgeName(run.kind)} writes its own commits, which match what vuoro computed above but not their hashes.</li>
          {u.strategy === 'rebase' && <li>A rebase rewrites the branch. Anyone with the old branch checked out must reset to the new one.</li>}
        </> : u.own && u.strategy === 'rebase' ? <>
          <li>Rewrites <span class="mono">{branch}</span>, your {word}'s branch, with {commits} rebased commit{commits === 1 ? '' : 's'} onto <span class="mono">{u.target}</span>, as you, with your own git access.</li>
          <li>Forces the push with a lease against <span class="mono">{short(run.head)}</span>, the head the rebase started from. If anyone pushed since, the push is refused and nothing is overwritten.</li>
          <li>Anyone with the old branch checked out must reset to the new one.</li>
        </> : u.own ? <>
          <li>Pushes a merge of <span class="mono">{u.target}</span> to <span class="mono">{branch}</span>, your {word}'s branch, as you, with your own git access. Fast-forward only.</li>
        </> : <>
          <li>Pushes a merge of <span class="mono">{u.target}</span> to <span class="mono">{branch}</span>, a new branch, as you, with your own git access.</li>
          <li>Opens a draft {word} onto <span class="mono">{u.source}</span>, which its author can merge. Their branch is not written.</li>
        </>}
        <li>Refused if <span class="mono">{u.source}</span> moved on the forge since the update started.</li>
        <li>Pull the branch into your own checkout afterwards; vuoro does not touch it.</li>
      </ul>
      {!u.own && (
        <>
          <label class="field">Title <input value={title} onInput={(e) => setTitle((e.target as HTMLInputElement).value)} /></label>
          <label class="field">Description <TextEditor rows={6} mono value={description} label="Description" onInput={setDescription} /></label>
        </>
      )}
      {error && <p class="alert" role="alert">⚠ {error}</p>}
      <div class="actions">
        <button ref={cancel} disabled={busy} onClick={onClose}>Cancel</button>
        <button class="primary" disabled={busy || (!u.own && !title.trim())} onClick={() => void push()}>{busy ? '⧗ Pushing…' : viaForge ? `Ask ${forgeName(run.kind)}` : u.own ? 'Push' : 'Push and open'}</button>
      </div>
    </Modal>
  )
}

// StartUpdate starts an update from a row of the project view, by merge or,
// on the person's own change, by rebase, and then opens the change, whose
// Update tab follows it.
export function StartUpdate({ url, target, onOpen }: { url: string; target: string; onOpen: (url: string) => void }) {
  const { busy, error, run: perform } = useAction()
  const start = (strategy: 'merge' | 'rebase') => perform(async () => {
    await post<Run>('/api/changes/update', { url, strategy })
    onOpen(url)
  })
  return (
    <>
      <button disabled={busy} title={`Merges ${target} in. An agent resolves any conflict, and nothing is pushed until you confirm.`}
        onClick={() => void start('merge')}>{busy ? '⧗ Starting…' : `Merge ${target} in`}</button>
      <button disabled={busy} title="On your own change: replays its commits onto the target, pushed with a lease once you confirm"
        onClick={() => void start('rebase')}>Rebase</button>
      {error && <p class="alert" role="alert">⚠ {error}</p>}
    </>
  )
}
