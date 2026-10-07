// One project of the Local tab (docs/plan.md section 6.8): its branch rows,
// ten at a time, each with its worktrees, its last review, its open change,
// and the reviews it offers.
import { useState } from 'preact/hooks'
import { useAction } from './action'
import { post } from './api'
import { changeNumber, type LocalBranch, type LocalChange, type LocalProject, type Run } from './types'
import { ago, StatusBadge } from './ui'

// pageSize is how many branch rows a project shows at a time.
export const pageSize = 10

// Reviewing is what a row's "Review local commits" runs with: the preset the
// person picked, and why it cannot run, when no agent tool can.
export interface Reviewing {
  preset?: string
  unavailable?: string
}

// standing says how a branch stands against its change's head.
export function standing(c: LocalChange): string {
  switch (c.standing) {
    case 'in-step': return 'in step'
    case 'ahead': return `${c.ahead} not pushed`
    case 'behind': return `${c.behind} behind`
    case 'diverged': return `${c.ahead} not pushed, ${c.behind} behind`
    default: return 'has commits not fetched here'
  }
}

// ChangeLink names a branch's open change, and opens its page as the inbox
// does.
export function ChangeLink({ change: c, onOpen }: { change: LocalChange; onOpen?: (url: string) => void }) {
  const name = `${changeNumber(c.kind, c.ref.number)} ${c.draft ? 'draft' : 'open'}`
  return (
    <span class="local-change">
      {onOpen ? <button class="link" title={c.title} onClick={() => onOpen(c.web_url)}>{name}</button> : <span title={c.title}>{name}</span>}
      {' '}<span class="muted">{standing(c)}</span>
    </span>
  )
}

const quietly = (b: LocalBranch) => b.quiet && b.needs.length === 0 && !b.change
const folder = (path: string) => path.split(/[\\/]/).filter(Boolean).pop() ?? path

export function Project({ project: p, quietDays, busy, reviewing, onHide, onBase, onOpen, onOpenRun }: {
  project: LocalProject; quietDays: number; busy: boolean; reviewing: Reviewing
  onHide: () => void; onBase: (base: string) => Promise<boolean>
  onOpen?: (url: string) => void; onOpenRun?: (id: string) => void
}) {
  const [limit, setLimit] = useState(pageSize)
  const [showQuiet, setShowQuiet] = useState(false)
  const [basing, setBasing] = useState(false)
  const needing = p.branches.filter((b) => b.needs.length > 0)
  const quiet = p.branches.filter(quietly)
  // What needs the person first, then the rest, each newest first as the
  // server sends them, and the quiet branches only when asked for.
  const ordered = [...needing, ...p.branches.filter((b) => b.needs.length === 0 && !quietly(b)), ...(showQuiet ? quiet : [])]
  const rows = ordered.slice(0, limit)
  const dirty = new Set(p.worktrees.filter((w) => w.dirty).map((w) => w.path))
  return (
    <section class="local-project" aria-label={p.name}>
      <h3>
        {p.name} <span class="badge" aria-label={`${needing.length} need you`}>{needing.length}</span>{' '}
        <span class="muted">{p.path}{p.base && ` · against ${p.base}`}</span>{' '}
        <button class="link" disabled={busy} onClick={() => setBasing(!basing)}>Change base</button>
        <span class="muted"> · </span>
        <button class="link" disabled={busy} onClick={onHide}>Hide</button>
      </h3>
      {basing && (
        <BaseForm base={p.base ?? ''} chosen={p.base_chosen} busy={busy} onCancel={() => setBasing(false)}
          onSave={async (base) => { if (await onBase(base)) setBasing(false) }} />
      )}
      {p.error ? (
        <p class="muted">⚠ {p.error}</p>
      ) : (
        <>
          {rows.length > 0 && (
            <ul class="local-branches">
              {rows.map((b) => (
                <BranchRow project={p} branch={b} dirty={dirty} reviewing={reviewing} onOpen={onOpen} onOpenRun={onOpenRun} key={b.name} />
              ))}
            </ul>
          )}
          <p class="local-more muted">
            {p.branches.length === 0 && `No branches beyond ${p.base}.`}
            {ordered.length > limit && (
              <button class="link" onClick={() => setLimit(limit + pageSize)}>
                Show {Math.min(pageSize, ordered.length - limit)} more of {ordered.length - limit}
              </button>
            )}{' '}
            {quiet.length > 0 && (
              <>
                <button class="link" onClick={() => setShowQuiet(!showQuiet)}>
                  {showQuiet ? 'Hide' : 'Show'} {quiet.length === 1 ? '1 quiet branch' : `${quiet.length} quiet branches`}
                </button>
                , with no commit in {quietDays} days
              </>
            )}
          </p>
        </>
      )}
    </section>
  )
}

// BaseForm changes what a project's branches are compared with. Empty, or
// "Use the default", goes back to origin/HEAD, main, or master.
function BaseForm({ base, chosen, busy, onSave, onCancel }: {
  base: string; chosen: boolean; busy: boolean; onSave: (base: string) => Promise<void>; onCancel: () => void
}) {
  const [value, setValue] = useState(base)
  return (
    <form class="local-path" onSubmit={(e) => { e.preventDefault(); void onSave(value.trim()) }}>
      <label>
        Compare branches with:{' '}
        <input type="text" autoFocus value={value} placeholder="origin/main" aria-label="The project's base branch"
          onInput={(e) => setValue((e.target as HTMLInputElement).value)} />
      </label>
      <button class="primary" disabled={busy}>Save</button>
      {chosen && <button type="button" disabled={busy} onClick={() => void onSave('')}>Use the default</button>}
      <button type="button" onClick={onCancel}>Cancel</button>
    </form>
  )
}

// BranchRow is one branch: its name, the worktrees it is checked out in,
// how far it is beyond the base, its last review and open change, what it
// needs, and its reviews. A branch with an open change offers both of its
// reviews side by side (D49).
function BranchRow({ project: p, branch: b, dirty, reviewing, onOpen, onOpenRun }: {
  project: LocalProject; branch: LocalBranch; dirty: Set<string>; reviewing: Reviewing
  onOpen?: (url: string) => void; onOpenRun?: (id: string) => void
}) {
  const act = useAction()
  const uncommitted = b.worktrees.filter((w) => dirty.has(w))
  const review = async () => {
    const r = await act.run(() => post<{ run: Run; uncommitted: string[] }>('/api/local/review', { key: p.key, branch: b.name, preset: reviewing.preset }))
    if (r) onOpenRun?.(r.run.id)
  }
  const change = b.change
  return (
    <li class="local-branch" aria-label={b.name}>
      <div class="local-branch-head">
        <code class="local-branch-name">{b.name}</code>
        {b.worktrees.map((w) => (
          <span class="local-tag" title={w} key={w}>{folder(w)}{dirty.has(w) && ' · uncommitted'}</span>
        ))}
        <span class="muted">{b.ahead} ahead · {ago(b.committed)}</span>
        <span class="local-branch-actions">
          {change && <button onClick={() => onOpen?.(change.web_url)}>Review the change</button>}
          <button disabled={act.busy || !!reviewing.unavailable} onClick={() => void review()}
            title={reviewing.unavailable ?? (b.run ? 'Follows the last review: its findings go to the reviewers, and what you decided on them carries over' : undefined)}>
            {act.busy ? 'Starting…' : 'Review local commits'}
          </button>
        </span>
      </div>
      <div class="local-branch-facts">
        {change && <ChangeLink change={change} onOpen={onOpen} />}
        {b.run && (
          <span>
            <button class="link" onClick={() => onOpenRun?.(b.run!.id)}>Reviewed {ago(b.run.created)}</button>{' '}
            <StatusBadge status={b.run.status} />
          </span>
        )}
        {b.fixes.map((f) => (
          <span class="local-tag" key={f.branch}>{f.branch} · {f.waiting ? 'waits to be merged' : 'merged'}</span>
        ))}
      </div>
      {b.needs.length > 0 && <p class="local-needs">Needs you: {b.needs.join('; ')}</p>}
      {uncommitted.length > 0 && (
        <p class="muted">Uncommitted changes in {uncommitted.map(folder).join(', ')} are not in a review of local commits.</p>
      )}
      {act.error && <p class="alert" role="alert">⚠ {act.error}</p>}
    </li>
  )
}
