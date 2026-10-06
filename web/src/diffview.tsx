import { Fragment, type ComponentChildren } from 'preact'
import { useLayoutEffect, useMemo, useState } from 'preact/hooks'
import { parsePatch, patchLines, placeAll, type Place, type Row } from './diff'
import type { Draft, FileDiff, ForgeDraft, Thread } from './types'
import { ago, CommentBubble, Prose, SeverityBadge, worstSeverity } from './ui'
import { post } from './api'
import { HeadBadge, useHead } from './head'
import { Composer, useWriter } from './write'

// A file past this many lines starts collapsed, and its rows are built only
// when the person opens it, so a large change stays responsive.
export const collapseAt = 400

// manyFiles is the file count past which a change opens with every file
// collapsed, and the file list opens the one the person picks.
export const manyFiles = 25

const threadPlace = (t: Thread): Place | undefined => {
  const a = t.anchor
  if (!a) return undefined
  return a.new_line > 0 ? { side: 'head', line: a.new_line } : { side: 'base', line: a.old_line }
}

const draftPlace = (d: Draft): Place | undefined =>
  d.anchor ? { side: d.anchor.side, line: d.anchor.end_line } : undefined

const minePlace = (d: ForgeDraft): Place | undefined => {
  const a = d.anchor
  if (!a) return undefined
  return a.new_line > 0 ? { side: 'head', line: a.new_line } : { side: 'base', line: a.old_line }
}

export function ThreadView({ thread }: { thread: Thread }) {
  const w = useWriter()
  const [replying, setReplying] = useState(false)
  const notes = thread.notes.filter((n) => !n.system)
  if (notes.length === 0) return null
  return (
    <div class={`thread ${thread.resolved ? 'resolved' : ''}`}>
      {thread.resolvable && <div class="thread-state">{thread.resolved ? '✓ resolved' : '○ open'}</div>}
      {notes.map((n) => (
        <div class="note" key={n.id}>
          <div class="note-head">
            <strong>@{n.author.username}</strong>
            {n.author.bot && <span class="badge">bot</span>}
            <span class="muted">{ago(n.at)}</span>
          </div>
          <Prose text={n.body} />
        </div>
      ))}
      {w && (w.caps.reply || (thread.resolvable && w.caps.resolve)) && (
        replying ? (
          <Composer
            placeholder="Reply, as a pending comment until you submit"
            submit={(text) => w.act('/api/changes/drafts', { body: text, thread_id: thread.id })}
            onDone={() => setReplying(false)}
          />
        ) : (
          <div class="actions">
            {w.caps.reply && <button onClick={() => setReplying(true)}>Reply</button>}
            {thread.resolvable && w.caps.resolve && (
              <button onClick={() => void w.act('/api/changes/resolve', { thread_id: thread.id, resolved: !thread.resolved }).catch(() => undefined)}>
                {thread.resolved ? 'Unresolve' : 'Resolve'}
              </button>
            )}
          </div>
        )
      )}
    </div>
  )
}

// MyDraft is one of the person's own pending comments on the forge.
export function MyDraft({ draft, where }: { draft: ForgeDraft; where?: string }) {
  const w = useWriter()
  const [editing, setEditing] = useState(false)
  return (
    <div class="mine">
      <div class="draft-meta muted">✎ my pending comment{where ? ` · ${where}` : ''}{draft.thread_id ? ' · a reply' : ''}</div>
      {editing && w ? (
        <Composer
          placeholder="Edit the comment"
          initial={draft.body}
          label="Save"
          submit={(text) => w.act(`/api/changes/drafts/${draft.id}/update`, { body: text })}
          onDone={() => setEditing(false)}
        />
      ) : (
        <>
          <Prose text={draft.body} />
          {w && (
            <div class="actions">
              <button onClick={() => setEditing(true)}>Edit</button>
              <button onClick={() => void w.act(`/api/changes/drafts/${draft.id}/delete`, {}).catch(() => undefined)}>Delete</button>
            </div>
          )}
        </>
      )}
    </div>
  )
}

// fence is a code fence longer than any run of backticks in s, as the
// server's drafts.Fence makes it.
export function fence(s: string): string {
  const longest = Math.max(0, ...(s.match(/`+/g) ?? []).map((r) => r.length))
  return '`'.repeat(Math.max(3, longest + 1))
}

// suggestionBlock is GitLab's suggestion block for a comment on the anchor's
// last line, replacing every line of its range.
export function suggestionBlock(d: Draft): string {
  if (d.suggestion === undefined || d.anchor?.side !== 'head') return ''
  const f = fence(d.suggestion)
  return `${f}suggestion:-${d.anchor.end_line - d.anchor.start_line}+0\n${d.suggestion}\n${f}`
}

// replacementBlock is a suggestion for a forge that cannot apply one: the
// replacement in a plain fence, under a line naming the lines it replaces.
export function replacementBlock(d: Draft): string {
  if (d.suggestion === undefined || d.anchor?.side !== 'head') return ''
  const a = d.anchor
  const lines = a.start_line < a.end_line ? `lines ${a.start_line}–${a.end_line}` : `line ${a.end_line}`
  const f = fence(d.suggestion)
  return `Suggested replacement for ${lines}:\n\n${f}\n${d.suggestion}\n${f}`
}

// keepText is what a kept draft says on the forge, as the server's keepBody
// composes it. The two must agree, so unedited text is sent as no text.
export function keepText(d: Draft, applies: boolean): string {
  let b = `**${d.title}**\n\n${d.description.trim()}`
  const e = d.evidence?.trim()
  if (e) b += `\n\n${fence(e)}\n${e}\n${fence(e)}`
  const s = applies ? suggestionBlock(d) : replacementBlock(d)
  if (s) b += '\n\n' + s
  return b
}

// DraftCard is an agent's draft. With a run, the person can keep it, which
// makes it their own pending comment on the forge, edit it first, or drop it.
export function DraftCard({ draft, onPick, label, runID, compact }: {
  draft: Draft; onPick?: () => void; label?: string; runID?: string
  compact?: boolean // inside a finding row, which already shows the title
}) {
  const w = useWriter()
  const [editing, setEditing] = useState(false)
  const [asked, setAsked] = useState<string>()
  const head = useHead()
  const staleAnchor = !!draft.anchor && !!head && head.reviewed_head !== head.current_head
  const a = draft.anchor
  const open = draft.status === 'pending' && !draft.decision
  // busy is the decision being written. Keep reads the change and writes to
  // the forge, which takes seconds on a large change, and a second click
  // must not write the comment twice. After a success the buttons stay off
  // until the run shows the decision.
  const [busy, setBusy] = useState<'keep' | 'drop' | 'done'>()
  const decide = (kind: 'keep' | 'drop') => {
    if (busy) return
    setBusy(kind)
    w?.act(`/api/runs/${runID}/drafts/${draft.id}/${kind}`, {})
      .then(() => setBusy('done'), () => setBusy(undefined))
  }
  const ask = () => {
    setAsked('asked')
    post(`/api/runs/${runID}/drafts/${draft.id}/suggest`).catch((e: unknown) => setAsked(`⚠ ${e instanceof Error ? e.message : String(e)}`))
  }
  return (
    <div class={`draft ${draft.status} ${draft.decision ?? ''} ${compact ? 'compact' : ''}`}>
      {!compact && (
        <div class="draft-head">
          <SeverityBadge severity={draft.severity} />
          <strong>{draft.title}</strong>
        </div>
      )}
      <div class="draft-meta muted">
        drafted by {draft.drafted_by}
        {a && (
          <>
            {' · '}
            {onPick ? (
              <button class="link" onClick={onPick}>{a.path}:{a.start_line}{a.end_line !== a.start_line ? `–${a.end_line}` : ''}{a.side === 'base' ? ' (base)' : ''}</button>
            ) : (
              `${a.path}:${a.start_line}`
            )}
          </>
        )}
        {draft.revision > 1 && draft.status === 'pending' && ` · revised ${draft.revision - 1}×`}
        {draft.status === 'withdrawn' && ' · withdrawn by the agent'}
        {draft.decision === 'kept' && ` · ✓ kept${draft.edited ? ', edited,' : ''} as your pending comment`}
        {draft.decision === 'dropped' && ' · ✗ dropped by you'}
      </div>
      {label && <div class="draft-label">{label}</div>}
      <HeadBadge state={head} />
      {editing && w && runID ? (
        <Composer
          placeholder="Edit before keeping"
          initial={keepText(draft, w.caps.suggestions)}
          label="Keep as my pending comment"
          // Unchanged text is sent as no text, so vuoro does not mark it edited.
          submit={(text) => w.act(`/api/runs/${runID}/drafts/${draft.id}/keep`, text === keepText(draft, w.caps.suggestions) ? {} : { body: text })}
          onDone={() => setEditing(false)}
        />
      ) : (
        <>
          <Prose text={draft.description} />
          {draft.evidence && <pre class="evidence">{draft.evidence}</pre>}
          {draft.suggestion !== undefined && a && (
            <div class="suggestion">
              <div class="muted">✎ Suggested change, replacing {a.path}:{a.start_line}{a.end_line !== a.start_line ? `–${a.end_line}` : ''}. Keeping it posts a suggestion the author can apply.</div>
              <pre class="md-code md-suggestion"><code>{draft.suggestion}</code></pre>
            </div>
          )}
          {open && w && runID && (
            <div class="actions">
              <button class="primary" disabled={staleAnchor || !!busy} aria-busy={busy === 'keep'} onClick={() => decide('keep')}>
                {busy === 'keep' ? '⧗ Keeping…' : 'Keep'}
              </button>
              <button disabled={staleAnchor || !!busy} onClick={() => setEditing(true)}>Edit and keep</button>
              <button disabled={!!busy} aria-busy={busy === 'drop'} onClick={() => decide('drop')}>{busy === 'drop' ? '⧗ Dropping…' : 'Drop'}</button>
              {a?.side === 'head' && draft.suggestion === undefined && (
                <button disabled={asked === 'asked'} onClick={ask}
                  title="A small new session reads this finding and its file and writes the replacement lines. Keeping the finding then posts a suggestion the author can apply">
                  {asked === 'asked' ? '⧗ Asked for a suggestion' : 'Ask for a suggestion'}
                </button>
              )}
            </div>
          )}
          {asked?.startsWith('⚠') && <p class="alert" role="alert">{asked}</p>}
        </>
      )}
    </div>
  )
}

// FileList is the column of files beside a diff. badge marks a file, such
// as with its comments.
export function FileList({ files, onPick, badge }: {
  files: FileDiff[]; onPick: (path: string) => void; badge?: (f: FileDiff) => ComponentChildren
}) {
  return (
    <nav class="file-list">
      {files.map((f) => (
        <button class="link file-link" key={f.new_path} title={f.new_path} onClick={() => onPick(f.new_path)}>
          <span class="file-name">
            {f.new_path.split('/').pop()}
            {badge?.(f)}
          </span>
          <span class="file-dir">{f.new_path.includes('/') ? f.new_path.slice(0, f.new_path.lastIndexOf('/')) : ''}</span>
        </button>
      ))}
    </nav>
  )
}

export function fileID(path: string) {
  return 'file-' + path.replace(/[^A-Za-z0-9_-]/g, '_')
}

// FileView is one file of the diff. picked counts the times the person has
// picked it from the file list, or is undefined when another file was picked
// last. A pick opens a collapsed file, and the next pick of another file
// folds it again, unless the person opened or closed it themselves.
export function FileView({ file, threads, drafts, mine = [], runID, collapsed = false, picked, idPrefix = '' }: {
  file: FileDiff; threads: Thread[]; drafts: Draft[]; mine?: ForgeDraft[]; runID?: string
  collapsed?: boolean; picked?: number
  idPrefix?: string // keeps a second diff of the same paths, such as the editor's, apart
}) {
  const lines = useMemo(() => patchLines(file.patch), [file.patch])
  const [open, setOpenState] = useState(!collapsed && lines <= collapseAt)
  const [opened, setOpened] = useState(false) // opened by a pick, not by hand
  const setOpen = (v: boolean) => { setOpened(false); setOpenState(v) }
  // A layout effect, so the file is open before the list scrolls to it.
  useLayoutEffect(() => {
    if (picked !== undefined && !open) {
      setOpenState(true)
      setOpened(true)
    } else if (picked === undefined && opened) {
      setOpenState(false)
      setOpened(false)
    }
  }, [picked])
  const name = file.renamed ? `${file.old_path} → ${file.new_path}` : file.new_path
  const tag = file.new ? 'added' : file.deleted ? 'deleted' : file.renamed ? 'renamed' : ''
  const notes = threads.length + drafts.length + mine.length
  return (
    <div class={`file ${open && !file.omitted ? '' : 'collapsed'}`} id={idPrefix + fileID(file.new_path)}>
      <header class="file-head">
        <button class="link file-toggle" onClick={() => setOpen(!open)} aria-expanded={open}>
          {open ? '▾' : '▸'} <span class="mono">{name}</span>
        </button>
        {tag && <span class="badge">{tag}</span>}
        {file.rebuilt && <span class="badge" title="The forge did not send this diff, and vuoro built it from both versions">rebuilt</span>}
        {notes > 0 && <CommentBubble count={notes} severity={worstSeverity(drafts)} />}
        {file.omitted ? (
          <span class="muted">binary or too large to show</span>
        ) : open ? (
          <span class="muted">{lines} lines</span>
        ) : (
          <button class="link show-lines" onClick={() => setOpen(true)}>Show {lines} lines</button>
        )}
      </header>
      {open && !file.omitted && <Hunks file={file} threads={threads} drafts={drafts} mine={mine} runID={runID} />}
    </div>
  )
}

// rowPlace is where a new comment on a row goes: the head's line, or the
// base's for a removed line.
const rowPlace = (r: Row): Place | undefined =>
  r.new !== undefined ? { side: 'head', line: r.new } : r.old !== undefined ? { side: 'base', line: r.old } : undefined

function Hunks({ file, threads, drafts, mine, runID }: { file: FileDiff; threads: Thread[]; drafts: Draft[]; mine: ForgeDraft[]; runID?: string }) {
  const w = useWriter()
  const [composing, setComposing] = useState<number>()
  const rows = useMemo(() => parsePatch(file.patch), [file.patch])
  const placedThreads = useMemo(() => placeAll(rows, threads, threadPlace), [rows, threads])
  const placedDrafts = useMemo(() => placeAll(rows, drafts, draftPlace), [rows, drafts])
  const placedMine = useMemo(() => placeAll(rows, mine, minePlace), [rows, mine])
  return (
    <>
      <table class="diff">
        <tbody>
          {rows.map((r, i) => {
            const ts = placedThreads.at.get(i)
            const ds = placedDrafts.at.get(i)
            const ms = placedMine.at.get(i)
            const p = rowPlace(r)
            return (
              <Fragment key={i}>
                <tr class={`row-${r.kind}`}>
                  <td class="ln">{r.old ?? ''}</td>
                  <td class="ln">{r.new ?? ''}</td>
                  <td class="code">
                    {w && p ? (
                      <button class="sign add-comment" title="Comment on this line" aria-label={`Comment on line ${p.line}`} onClick={() => setComposing(i)}>
                        {r.kind === 'add' ? '+' : r.kind === 'del' ? '−' : ' '}
                      </button>
                    ) : (
                      <span class="sign">{r.kind === 'add' ? '+' : r.kind === 'del' ? '−' : ' '}</span>
                    )}
                    {r.text}
                  </td>
                </tr>
                {(ts || ds || ms || composing === i) && (
                  <tr class="row-comments">
                    <td colSpan={3}>
                      {ts?.map((t) => <ThreadView thread={t} key={t.id} />)}
                      {ms?.map((d) => <MyDraft draft={d} key={d.id} />)}
                      {ds?.map((d) => <DraftCard draft={d} key={d.id} runID={runID} />)}
                      {composing === i && w && p && (
                        <Composer
                          placeholder={`Comment on line ${p.line}${p.side === 'base' ? ' (removed)' : ''}, as a pending comment until you submit`}
                          submit={(text) => w.act('/api/changes/drafts', { body: text, place: { path: p.side === 'base' ? file.old_path || file.new_path : file.new_path, side: p.side, line: p.line } })}
                          onDone={() => setComposing(undefined)}
                        />
                      )}
                    </td>
                  </tr>
                )}
              </Fragment>
            )
          })}
        </tbody>
      </table>
      {placedThreads.rest.length + placedMine.rest.length > 0 && (
        <div class="pad">
          <p class="muted">On lines this diff no longer shows:</p>
          {placedThreads.rest.map((t) => <ThreadView thread={t} key={t.id} />)}
          {placedMine.rest.map((d) => <MyDraft draft={d} key={d.id} />)}
        </div>
      )}
    </>
  )
}
