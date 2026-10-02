import { Fragment } from 'preact'
import { useMemo, useState } from 'preact/hooks'
import { parsePatch, patchLines, placeAll, type Place, type Row } from './diff'
import type { Draft, FileDiff, ForgeDraft, Thread } from './types'
import { ago, CommentBubble, Prose, SeverityBadge, worstSeverity } from './ui'
import { post } from './api'
import { Composer, useWriter } from './write'

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
      {w && (
        replying ? (
          <Composer
            placeholder="Reply, as a pending comment until you submit"
            submit={(text) => w.act('/api/changes/drafts', { body: text, thread_id: thread.id })}
            onDone={() => setReplying(false)}
          />
        ) : (
          <div class="actions">
            <button onClick={() => setReplying(true)}>Reply</button>
            {thread.resolvable && (
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

// keepText is what a kept draft says on the forge, as the server's keepBody
// composes it. The two must agree, so unedited text is sent as no text.
export function keepText(d: Draft): string {
  let b = `**${d.title}**\n\n${d.description.trim()}`
  const e = d.evidence?.trim()
  if (e) b += `\n\n${fence(e)}\n${e}\n${fence(e)}`
  const s = suggestionBlock(d)
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
  const a = draft.anchor
  const open = draft.status === 'pending' && !draft.decision
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
      {editing && w && runID ? (
        <Composer
          placeholder="Edit before keeping"
          initial={keepText(draft)}
          label="Keep as my pending comment"
          // Unchanged text is sent as no text, so vuoro does not mark it edited.
          submit={(text) => w.act(`/api/runs/${runID}/drafts/${draft.id}/keep`, text === keepText(draft) ? {} : { body: text })}
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
              <button class="primary" onClick={() => void w.act(`/api/runs/${runID}/drafts/${draft.id}/keep`, {}).catch(() => undefined)}>Keep</button>
              <button onClick={() => setEditing(true)}>Edit and keep</button>
              <button onClick={() => void w.act(`/api/runs/${runID}/drafts/${draft.id}/drop`, {}).catch(() => undefined)}>Drop</button>
              {a?.side === 'head' && draft.suggestion === undefined && (
                <button disabled={asked === 'asked'} onClick={ask}
                  title="The agent that drafted this writes the replacement lines, and keeping it posts a suggestion the author can apply">
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

export function fileID(path: string) {
  return 'file-' + path.replace(/[^A-Za-z0-9_-]/g, '_')
}

export function FileView({ file, threads, drafts, mine = [], runID }: {
  file: FileDiff; threads: Thread[]; drafts: Draft[]; mine?: ForgeDraft[]; runID?: string
}) {
  const lines = useMemo(() => patchLines(file.patch), [file.patch])
  const [open, setOpen] = useState(lines <= collapseAt)
  const name = file.renamed ? `${file.old_path} → ${file.new_path}` : file.new_path
  const tag = file.new ? 'added' : file.deleted ? 'deleted' : file.renamed ? 'renamed' : ''
  const notes = threads.length + drafts.length + mine.length
  return (
    <div class={`file ${open && !file.omitted ? '' : 'collapsed'}`} id={fileID(file.new_path)}>
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
