// The editor's work: its commits beside the findings they answer, its diff,
// and the push, which always asks first.
import { useEffect, useRef, useState } from 'preact/hooks'
import { errorText, useAction } from './action'
import { api, post } from './api'
import { diffStat } from './diff'
import { DiffBrowser, Stat, useFilePicker } from './diffview'
import { Modal } from './modal'
import { TextEditor } from './textedit'
import type { Draft, EditorDetail, EditorState, FileDiff, Run, StackPreview } from './types'
import { changeWord } from './types'
import { WriteContext } from './write'
import { short, StatusBadge } from './ui'

// Editor hands the kept findings to an editor session, which fixes them on
// a local branch in a sandbox, and shows its commits beside the findings
// they answer (D39, D40). Nothing here touches the forge.
// On a triage of the person's own change (D41), picked are the thread replies
// to fix and describe names one; on a review, the kept findings are fixed.
export function Editor({ run, url, onRun, picked, describe, picks = false }: {
  run: Run; url: string; onRun: (r: Run) => void; picked?: Draft[]; describe?: (d: Draft) => string
  picks?: boolean // a review of the person's own work, whose findings they pick (D42)
}) {
  const [detail, setDetail] = useState<EditorDetail>()
  const { busy, error, setError, run: perform } = useAction()
  const [discarding, setDiscarding] = useState(false)
  const threads = run.mode === 'triage'
  const own = threads || picks || run.kind === 'local'
  const kept = run.drafts.filter((d) => d.decision === 'kept' && !d.fixed_in)
  const name = describe ?? ((d: Draft) => d.title)
  const ed = run.editor && !run.editor.discarded ? run.editor : undefined
  const session = ed && run.sessions.find((s) => s.id === ed.session)
  const working = session?.status === 'working' || session?.status === 'preparing'
  // The editor's work is read again each time its turn ends.
  useEffect(() => {
    if (!ed || working) return
    api<EditorDetail>(`/api/runs/${run.id}/editor`).then(setDetail, (e: unknown) => setError(errorText(e)))
  }, [ed?.session, working, session?.log?.length])
  const act = (path: string, body?: unknown) => perform(async () => {
    onRun(await post<Run>(path, body))
    setDetail(undefined)
    setDiscarding(false)
  })
  if (run.mode && !threads) return null
  const byID = new Map(run.drafts.map((d) => [d.id, d]))
  const title = (id: string) => { const d = byID.get(id); return d ? name(d) : '' }
  const waves = (run.waves ?? []).filter((w) => w.pushed)
  // A start, first or after a push. A later wave goes on from the last push
  // and is given only what no wave has fixed (D50).
  const start = (again: boolean) => {
    const items = own ? picked ?? [] : kept
    const what = threads ? 'thread' : 'finding'
    const why = items.length > 0 ? '' : own ? `Pick a ${what} first` : 'Keep a finding first'
    const label = own ? `the picked ${what}s` : 'the kept findings'
    return (
      <button disabled={!!why || busy} onClick={() => void act(`/api/runs/${run.id}/editor`, own ? { url, items: items.map((d) => d.id) } : { url })}
        title={why || `An agent fixes ${label} ${again ? 'on the pushed branch, from its last push' : 'on a local branch'}, in a sandbox, with no forge credential. Nothing is pushed.`}>
        {busy ? '⧗ Starting the editor…'
          : again ? `⚒ Fix ${label} in another wave (${items.length})`
          : own ? `⚒ Fix ${label} in a branch (${items.length})` : `⚒ Fix these in a branch (${kept.length} kept)`}
      </button>
    )
  }
  if (!ed) {
    return (
      <div class="editor-panel">
        {waves.length > 0 && <Waves waves={waves} title={title} />}
        {start(waves.length > 0)}
        {error && <p class="alert" role="alert">⚠ {error}</p>}
      </div>
    )
  }
  const fixed = new Set(detail?.commits.flatMap((c) => c.findings) ?? [])
  const given = own ? (ed.items ?? []).map((id) => byID.get(id)).filter((d): d is Draft => !!d) : kept
  const left = given.filter((d) => !fixed.has(d.id))
  return (
    <div class="editor-panel">
      <header class="editor-head">
        <strong>⚒ Editor</strong>
        <span class="mono">{ed.branch}</span>
        {session && <StatusBadge status={session.status} />}
        <span class="muted">
          {ed.continues ? 'from its last push, ' : 'from '}<span class="mono">{short(ed.base)}</span>{ed.pushed ? '' : ', local only'}
        </span>
        {discarding ? (
          <span class="push">
            Delete the branch and its work?{' '}
            <button class="primary" disabled={busy} onClick={() => void act(`/api/runs/${run.id}/editor/discard`)}>Discard</button>{' '}
            <button class="link" onClick={() => setDiscarding(false)}>Keep it</button>
          </span>
        ) : (
          <button class="push" disabled={busy} onClick={() => setDiscarding(true)}>Discard…</button>
        )}
      </header>
      {waves.length > 0 && <Waves waves={waves} title={title} />}
      {working && <p class="muted">⧗ The editor is working. Open its session above to follow it or talk to it.</p>}
      {detail && (
        <>
          <ol class="editor-commits">
            {detail.commits.map((c) => (
              <li key={c.sha}>
                <span class="commit-sha">{short(c.sha)}</span>
                <div class="commit-body">
                  <div class="commit-subject">{c.subject}</div>
                  {c.findings.map((id) => (
                    <div class="commit-answers" key={id}>
                      ↳ answers <span class="badge">{id}</span> {title(id)}
                    </div>
                  ))}
                </div>
              </li>
            ))}
          </ol>
          {detail.commits.length === 0 && !working && <p class="muted">No commits yet.</p>}
          {left.length > 0 && !working && (
            <p class="muted">
              ○ No commit names {left.map((d) => `${d.id} ${name(d)}`).join(', ')}. Its reply in the session says why.
            </p>
          )}
          <PushBar run={run} url={url} detail={detail} working={working} onRun={onRun} />
          {ed.pushed && !working && detail.commits.at(-1)?.sha === ed.pushed.sha && (
            <div class="editor-again">
              <p class="muted">All of this wave is pushed. Another wave goes on from <span class="mono">{short(ed.pushed.sha)}</span>.</p>
              {start(true)}
            </div>
          )}
          {detail.files.length > 0 && <EditorDiff files={detail.files} />}
        </>
      )}
      {error && <p class="alert" role="alert">⚠ {error}</p>}
    </div>
  )
}

// Waves are a review's earlier editors, each with what it pushed, folded
// away (D50).
function Waves({ waves, title }: { waves: EditorState[]; title: (id: string) => string }) {
  return (
    <details class="editor-waves">
      <summary>{waves.length === 1 ? '1 earlier wave' : `${waves.length} earlier waves`}</summary>
      <ol>
        {waves.map((w) => (
          <li key={w.session}>
            <span class="mono">{w.branch}</span>{' '}
            <span class="muted">pushed <span class="mono">{short(w.pushed!.sha)}</span> to {w.pushed!.target}</span>
            {w.pushed!.url && <> · <a href={w.pushed!.url} target="_blank" rel="noreferrer">its change</a></>}
            <ol class="editor-commits">
              {(w.commits ?? []).map((c) => (
                <li key={c.sha}>
                  <span class="commit-sha">{short(c.sha)}</span>
                  <div class="commit-body">
                    <div class="commit-subject">{c.subject}</div>
                    {c.findings.map((id) => (
                      <div class="commit-answers" key={id}>↳ answers <span class="badge">{id}</span> {title(id)}</div>
                    ))}
                  </div>
                </li>
              ))}
            </ol>
          </li>
        ))}
      </ol>
    </details>
  )
}

// EditorDiff is the editor's diff, laid out as the change's is: its files in
// a column, each picked one opened and scrolled to. It is not the change's
// diff, so nothing here comments on the forge.
function EditorDiff({ files }: { files: FileDiff[] }) {
  const { picked, pick } = useFilePicker(editorFiles)
  return (
    <>
      <h4 class="editor-diff-head">
        Changes · {files.length} file{files.length === 1 ? '' : 's'}
        <Stat {...files.reduce((t, f) => { const s = diffStat(f.patch); return { add: t.add + s.add, del: t.del + s.del } }, { add: 0, del: 0 })} />
      </h4>
      <WriteContext.Provider value={undefined}>
        <DiffBrowser files={files} picked={picked} onPick={pick} idPrefix={editorFiles} />
      </WriteContext.Provider>
    </>
  )
}

const editorFiles = 'editor-'

// PushBar offers the push of the editor's work and the stacked draft change,
// and links it once open. The push always asks first: it is the
// one action here that reaches the forge.
export function PushBar({ run, url, detail, working, onRun }: {
  run: Run; url: string; detail: EditorDetail; working: boolean; onRun: (r: Run) => void
}) {
  const [preview, setPreview] = useState<StackPreview>()
  const [asked, setAsked] = useState<string>()
  const { busy: loading, error, run: perform } = useAction()
  const pushed = run.editor?.pushed
  const own = !!run.editor?.own
  const local = !!run.editor?.local
  const last = detail.commits[detail.commits.length - 1]
  const fresh = !!last && last.sha !== pushed?.sha
  const word = changeWord(run.kind)
  const ask = () => perform(async () => setPreview(await post<StackPreview>(`/api/runs/${run.id}/editor/stack`, { url })))
  return (
    <div class="push-bar">
      {pushed && local && (
        <span>✓ The branch <span class="mono">{pushed.branch}</span> is in your repository at <span class="mono">{short(pushed.sha)}</span>. Merge or cherry-pick it: <span class="mono">git merge {pushed.branch}</span></span>
      )}
      {pushed && !local && (own ? (
        <span>✓ Pushed to your branch <span class="mono">{pushed.branch}</span> at <span class="mono">{short(pushed.sha)}</span> · the fixed threads' replies cite their commits</span>
      ) : (
        <span>
          ✓ Pushed <span class="mono">{pushed.branch}</span> at <span class="mono">{short(pushed.sha)}</span>
          {pushed.url ? <> · <a href={pushed.url} target="_blank" rel="noreferrer">the {pushed.not_draft ? '' : 'draft '}{word}</a></> : ` · the ${word} did not open; push again to retry`}
          {pushed.not_draft && <span class="warn"> · ⚠ It opened as an ordinary {word}, not a draft, so it can be merged as it is. Mark it as a draft on the forge.</span>}
        </span>
      ))}
      {(fresh || (!own && !local && pushed && !pushed.url)) && (
        <button class="primary" disabled={working || loading} onClick={() => void ask()}
          title={working ? 'Wait for the editor to finish' : 'Shows where it goes, and asks first'}>
          {loading ? '⧗ Reading…' : pushed ? '⇪ Push the new commits…' : local ? '⇪ Hand back as a branch in your repository…' : own ? '⇪ Push to your branch…' : `⇪ Push and open a draft ${word}…`}
        </button>
      )}
      {asked && <p class="notice" role="status">⧗ {asked}</p>}
      {error && <p class="alert" role="alert">⚠ {error}</p>}
      {preview && <PushDialog run={run} url={url} preview={preview} onClose={() => setPreview(undefined)} onRun={onRun}
        onAsked={(text) => { setAsked(text); setPreview(undefined) }} />}
    </div>
  )
}

// PushDialog is the confirmation, with the title and description to edit.
// Cancel has the focus, so a stray Enter pushes nothing.
// A push that stopped to ask the editor about a picked finding no commit's
// trailer names (D50) answers 409; its text says so, and onAsked shows it
// while the editor answers in its session.
export function PushDialog({ run, url, preview: p, onClose, onRun, onAsked }: {
  run: Run; url: string; preview: StackPreview; onClose: () => void; onRun: (r: Run) => void; onAsked?: (text: string) => void
}) {
  const [title, setTitle] = useState(p.title ?? '')
  const [description, setDescription] = useState(p.description ?? '')
  const { busy, error, run: perform } = useAction()
  const cancel = useRef<HTMLButtonElement>(null)
  const word = changeWord(run.kind)
  const push = () => perform(async () => {
    try {
      onRun(await post<Run>(`/api/runs/${run.id}/editor/push`, { url, confirm: 'push', sha: p.sha, title, description }))
    } catch (e) {
      const text = errorText(e)
      if (!onAsked || !text.includes('vuoro asked the editor')) throw e
      onAsked(text.replace(/^review: /, ''))
      return
    }
    onClose()
  })
  return (
    <Modal class="push-dialog" onClose={onClose} focus={cancel} locked={busy}
      title={p.local ? 'Hand back as a branch' : p.own ? 'Push to your branch' : p.opened ? 'Push the new commits' : `Push and open a draft ${word}`}>
      <ul>
        {p.local ? <>
          <li>Writes {p.commits} commit{p.commits === 1 ? '' : 's'} as the branch <span class="mono">{p.branch}</span> in your repository.</li>
          <li>Nothing else changes there: not your checkout, your working tree, HEAD, or any other branch. A branch of that name you hold is never moved, and your hooks do not run.</li>
        </> : p.own ? <>
          <li>Pushes {p.commits} commit{p.commits === 1 ? '' : 's'} to <span class="mono">{p.target}</span>, your {word}'s branch, as you, with your own git access. Fast-forward only.</li>
          <li>Each fixed thread's drafted reply then cites its commit, unless you wrote the reply. Nothing is posted until you add replies and submit.</li>
          <li>Pull the branch into your own checkout afterwards; vuoro does not touch it.</li>
        </> : <>
          <li>Pushes {p.commits} commit{p.commits === 1 ? '' : 's'} to <span class="mono">{p.branch}</span>, as you, with your own git access.</li>
          <li>{p.opened ? <>Updates the open <a href={p.opened} target="_blank" rel="noreferrer">{p.not_draft ? '' : 'draft '}{word}</a>.</> : <>Opens a draft {word} onto <span class="mono">{p.target}</span>, the source branch of the reviewed change.</>}</li>
        </>}
        <li>Changes {p.files.length} file{p.files.length === 1 ? '' : 's'}: <span class="mono">{p.files.join(', ')}</span></li>
        <li>First, vuoro checks each picked {run.mode === 'triage' ? 'thread' : 'finding'} against the commits' trailers, and asks the editor once about any no commit names before pushing.</li>
        {p.ci && p.ci.length > 0 && <li class="warn">⚠ Changes CI configuration ({p.ci.join(', ')}), which runs when the branch is pushed.</li>}
        {p.moved && <li class="warn">⚠ <span class="mono">{p.target}</span> has moved since {p.own ? 'triage, so the push will be refused; run triage again on the new head' : `the review, so the ${word} may need a rebase`}.</li>}
      </ul>
      {!p.opened && !p.own && !p.local && (
        <>
          <label class="field">Title <input value={title} onInput={(e) => setTitle((e.target as HTMLInputElement).value)} /></label>
          <label class="field">Description <TextEditor rows={10} mono value={description} label="Description" onInput={setDescription} /></label>
        </>
      )}
      {error && <p class="alert" role="alert">⚠ {error}</p>}
      <div class="actions">
        <button ref={cancel} disabled={busy} onClick={onClose}>Cancel</button>
        <button class="primary" disabled={busy || (!p.own && !p.local && !title.trim()) || (p.own && p.moved)} onClick={() => void push()}>{busy ? '⧗ Pushing…' : p.local ? 'Write the branch' : p.own || p.opened ? 'Push' : 'Push and open'}</button>
      </div>
    </Modal>
  )
}
