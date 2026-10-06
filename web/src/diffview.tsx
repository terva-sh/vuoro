import { Fragment, type ComponentChildren } from 'preact'
import { useLayoutEffect, useMemo, useState } from 'preact/hooks'
import { diffStat, parsePatch, patchLines, placeAll, type Place, type Row } from './diff'
import type { Draft, FileDiff, ForgeDraft, Thread } from './types'
import { CommentBubble, worstSeverity } from './ui'
import { DraftCard, MyDraft, ThreadView } from './comments'
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

// Stat is a file's added and removed line counts, each with its sign, so
// the colour is never the only signal.
export function Stat({ add, del }: { add: number; del: number }) {
  return (
    <span class="stat" aria-label={`${add} added, ${del} removed`}>
      <span class="stat-add">+{add}</span> <span class="stat-del">−{del}</span>
    </span>
  )
}

// FileList is the column of files beside a diff. badge marks a file, such
// as with its comments, and current is the file picked last.
export function FileList({ files, onPick, badge, current }: {
  files: FileDiff[]; onPick: (path: string) => void; badge?: (f: FileDiff) => ComponentChildren; current?: string
}) {
  const stats = useMemo(() => new Map(files.map((f) => [f.new_path, diffStat(f.patch)])), [files])
  return (
    <nav class="file-list">
      {files.map((f) => {
        const tag = f.new ? 'added' : f.deleted ? 'deleted' : f.renamed ? 'renamed' : ''
        return (
          <button class={`link file-link${f.new_path === current ? ' current' : ''}`} key={f.new_path} title={f.new_path}
            aria-current={f.new_path === current ? 'location' : undefined} onClick={() => onPick(f.new_path)}>
            <span class="file-name">
              {f.new_path.split('/').pop()}
              {badge?.(f)}
            </span>
            <span class="file-sub">
              <span class="file-dir">{f.new_path.includes('/') ? f.new_path.slice(0, f.new_path.lastIndexOf('/')) : ''}</span>
              {tag && <span class="file-tag">{tag}</span>}
              {!f.omitted && <Stat {...stats.get(f.new_path)!} />}
            </span>
          </button>
        )
      })}
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
  const stat = useMemo(() => diffStat(file.patch), [file.patch])
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
          <span class="muted push">binary or too large to show</span>
        ) : (
          <span class="push file-meta">
            <Stat {...stat} />
            {!open && <button class="link show-lines" onClick={() => setOpen(true)}>Show {lines} lines</button>}
          </span>
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
