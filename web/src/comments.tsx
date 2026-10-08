// Comments on a change: the forge's threads, the person's own pending
// comments, and an agent's drafts, which the person keeps, edits, or drops.
import { useState } from 'preact/hooks'
import { errorText } from './action'
import { post } from './api'
import { HeadBadge, useHead } from './head'
import type { Draft, ForgeDraft, Thread } from './types'
import { ago, Prose, SeverityBadge } from './ui'
import { Composer, useWriter } from './write'

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

// suggestionBlock is the forge's suggestion block replacing every line of the
// anchor's range: GitLab's, for a comment on its last line, or, with span,
// GitHub's, for a comment spanning the range.
export function suggestionBlock(d: Draft, span = false): string {
  if (d.suggestion === undefined || d.anchor?.side !== 'head') return ''
  const f = fence(d.suggestion)
  return `${f}suggestion${span ? '' : `:-${d.anchor.end_line - d.anchor.start_line}+0`}\n${d.suggestion}\n${f}`
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
export function keepText(d: Draft, applies: boolean, span = false): string {
  let b = `**${d.title}**\n\n${d.description.trim()}`
  const e = d.evidence?.trim()
  if (e) b += `\n\n${fence(e)}\n${e}\n${fence(e)}`
  const s = applies ? suggestionBlock(d, span) : replacementBlock(d)
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
    post(`/api/runs/${runID}/drafts/${draft.id}/suggest`).catch((e: unknown) => setAsked(`⚠ ${errorText(e)}`))
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
        {draft.earlier && ` · restates earlier finding ${draft.earlier}`}
        {draft.decision === 'kept' && (draft.carried ? ' · ✓ kept in the earlier review, as your comment' : ` · ✓ kept${draft.edited ? ', edited,' : ''} as your pending comment`)}
        {draft.decision === 'dropped' && (draft.carried ? ' · ✗ dropped by you in the earlier review' : ' · ✗ dropped by you')}
      </div>
      {label && <div class="draft-label">{label}</div>}
      {!draft.fixed_in && <HeadBadge state={head} />}
      {editing && w && runID ? (
        <Composer
          placeholder="Edit before keeping"
          initial={keepText(draft, w.caps.suggestions, w.caps.span_suggestions)}
          label="Keep as my pending comment"
          // Unchanged text is sent as no text, so vuoro does not mark it edited.
          submit={(text) => w.act(`/api/runs/${runID}/drafts/${draft.id}/keep`, text === keepText(draft, w.caps.suggestions, w.caps.span_suggestions) ? {} : { body: text })}
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
