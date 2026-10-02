import { Fragment } from 'preact'
import { useMemo, useState } from 'preact/hooks'
import { parsePatch, patchLines, placeAll, type Place } from './diff'
import type { Draft, FileDiff, Thread } from './types'
import { ago, Prose, SeverityBadge } from './ui'

// A file past this many lines starts collapsed, and its rows are built only
// when the person opens it, so a large change stays responsive.
export const collapseAt = 400

const threadPlace = (t: Thread): Place | undefined => {
  const a = t.anchor
  if (!a) return undefined
  return a.new_line > 0 ? { side: 'head', line: a.new_line } : { side: 'base', line: a.old_line }
}

const draftPlace = (d: Draft): Place | undefined =>
  d.anchor ? { side: d.anchor.side, line: d.anchor.end_line } : undefined

export function ThreadView({ thread }: { thread: Thread }) {
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
    </div>
  )
}

export function DraftCard({ draft, onPick }: { draft: Draft; onPick?: () => void }) {
  const a = draft.anchor
  return (
    <div class={`draft ${draft.status}`}>
      <div class="draft-head">
        <SeverityBadge severity={draft.severity} />
        <strong>{draft.title}</strong>
      </div>
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
        {draft.status === 'withdrawn' && ' · withdrawn'}
      </div>
      <Prose text={draft.description} />
      {draft.evidence && <pre class="evidence">{draft.evidence}</pre>}
    </div>
  )
}

export function fileID(path: string) {
  return 'file-' + path.replace(/[^A-Za-z0-9_-]/g, '_')
}

export function FileView({ file, threads, drafts }: { file: FileDiff; threads: Thread[]; drafts: Draft[] }) {
  const lines = useMemo(() => patchLines(file.patch), [file.patch])
  const [open, setOpen] = useState(lines <= collapseAt)
  const name = file.renamed ? `${file.old_path} → ${file.new_path}` : file.new_path
  const tag = file.new ? 'added' : file.deleted ? 'deleted' : file.renamed ? 'renamed' : ''
  const notes = threads.length + drafts.length
  return (
    <div class="file" id={fileID(file.new_path)}>
      <header class="file-head">
        <button class="link file-toggle" onClick={() => setOpen(!open)} aria-expanded={open}>
          {open ? '▾' : '▸'} <span class="mono">{name}</span>
        </button>
        {tag && <span class="badge">{tag}</span>}
        {file.rebuilt && <span class="badge" title="The forge did not send this diff, and vuoro built it from both versions">rebuilt</span>}
        {notes > 0 && <span class="badge">💬 {notes}</span>}
        <span class="muted">{lines} lines</span>
      </header>
      {file.omitted ? (
        <p class="muted pad">Binary or too large to show.</p>
      ) : open ? (
        <Hunks file={file} threads={threads} drafts={drafts} />
      ) : (
        <p class="muted pad">
          Collapsed. <button class="link" onClick={() => setOpen(true)}>Show {lines} lines</button>
        </p>
      )}
    </div>
  )
}

function Hunks({ file, threads, drafts }: { file: FileDiff; threads: Thread[]; drafts: Draft[] }) {
  const rows = useMemo(() => parsePatch(file.patch), [file.patch])
  const placedThreads = useMemo(() => placeAll(rows, threads, threadPlace), [rows, threads])
  const placedDrafts = useMemo(() => placeAll(rows, drafts, draftPlace), [rows, drafts])
  const elsewhere = [...placedThreads.rest]
  return (
    <>
      <table class="diff">
        <tbody>
          {rows.map((r, i) => {
            const ts = placedThreads.at.get(i)
            const ds = placedDrafts.at.get(i)
            return (
              <Fragment key={i}>
                <tr class={`row-${r.kind}`}>
                  <td class="ln">{r.old ?? ''}</td>
                  <td class="ln">{r.new ?? ''}</td>
                  <td class="code">
                    <span class="sign">{r.kind === 'add' ? '+' : r.kind === 'del' ? '−' : ' '}</span>
                    {r.text}
                  </td>
                </tr>
                {(ts || ds) && (
                  <tr class="row-comments">
                    <td colSpan={3}>
                      {ts?.map((t) => <ThreadView thread={t} key={t.id} />)}
                      {ds?.map((d) => <DraftCard draft={d} key={d.id} />)}
                    </td>
                  </tr>
                )}
              </Fragment>
            )
          })}
        </tbody>
      </table>
      {elsewhere.length > 0 && (
        <div class="pad">
          <p class="muted">On lines this diff no longer shows:</p>
          {elsewhere.map((t) => <ThreadView thread={t} key={t.id} />)}
        </div>
      )}
    </>
  )
}
