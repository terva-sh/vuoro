// Asking first. An action that acts on the forge as the person, where others
// see it, asks before it runs, unless the person waived asking for it.
import { useEffect, useRef, useState } from 'preact/hooks'
import { api, post } from './api'
import type { Change, Setup } from './types'

// The actions that ask first, as the server names them for waivers.
export type Confirmable = 'submit' | 'approve'

const actionWords: Record<Confirmable, string> = { submit: 'submitting a review', approve: 'approving' }

// waive turns asking off, or back on with ask, and returns the setup as it
// now stands.
export async function waive(actions: string[], ask: boolean): Promise<Setup> {
  for (const action of actions) await post('/api/prefs/confirm', { action, ask })
  return api<Setup>('/api/setup')
}

export interface Ask {
  title: string
  lines: string[]
  go: string // the confirming button's label
  actions: Confirmable[]
  run: () => void
}

// ConfirmDialog asks once. Cancel, Escape, and a click outside all do
// nothing, and Cancel has the focus, so a stray Enter does nothing either.
export function ConfirmDialog({ ask, onClose, onWaive }: {
  ask: Ask; onClose: () => void; onWaive: (actions: Confirmable[]) => void
}) {
  const [waive, setWaive] = useState(false)
  const cancel = useRef<HTMLButtonElement>(null)
  useEffect(() => {
    cancel.current?.focus()
    const onKey = (e: KeyboardEvent) => e.key === 'Escape' && onClose()
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [])
  return (
    <div class="modal-back" onClick={(e) => e.target === e.currentTarget && onClose()}>
      <div class="modal" role="dialog" aria-modal="true" aria-labelledby="confirm-title">
        <h2 id="confirm-title">{ask.title}</h2>
        <ul>{ask.lines.map((l) => <li key={l}>{l}</li>)}</ul>
        <label class="waive">
          <input type="checkbox" checked={waive} onChange={(e) => setWaive((e.target as HTMLInputElement).checked)} />
          {' '}Don't ask again before {ask.actions.map((a) => actionWords[a]).join(' or ')}
        </label>
        <div class="actions">
          <button ref={cancel} onClick={onClose}>Cancel</button>
          <button class="primary" onClick={() => { if (waive) onWaive(ask.actions); onClose(); ask.run() }}>{ask.go}</button>
        </div>
      </div>
    </div>
  )
}

// SubmitBar is the person's pending comments, and the actions that publish
// them or approve, each of which asks first unless waived.
export function SubmitBar({ change, me, pending, approved, waived, act, onWaive }: {
  change: Change; me: string; pending: number; approved: boolean; waived: string[]
  act: (path: string, body: Record<string, unknown>) => void
  onWaive: (actions: Confirmable[]) => void
}) {
  const [ask, setAsk] = useState<Ask>()
  const mr = `!${change.ref.number}`
  const confirm = (a: Ask) => (a.actions.every((x) => waived.includes(x)) ? a.run() : setAsk(a))
  const comments = `${pending} pending comment${pending === 1 ? '' : 's'}`
  const publish = pending > 0
    ? `Publishes your ${comments} as @${me}, and everyone on the merge request sees them.`
    : 'You have no pending comments, so this only marks your review as submitted.'
  const own = change.author.username === me
    ? ['You opened this merge request. GitLab refuses an author’s approval when the project forbids it.']
    : []
  const approve = `Approves ${mr} as @${me}, and everyone on the merge request sees it.`
  return (
    <div class="submit-bar">
      <span>✎ {comments} of yours, seen only by you until you submit</span>
      <button class="primary" disabled={pending === 0} onClick={() => confirm({
        title: `Submit your review of ${mr}?`, lines: [publish], go: 'Submit', actions: ['submit'],
        run: () => act('/api/changes/submit', {}),
      })}>Submit</button>
      <button disabled={approved} onClick={() => confirm({
        title: `Submit your review of ${mr} and approve it?`, lines: [publish, approve, ...own], go: 'Submit and approve', actions: ['submit', 'approve'],
        run: () => act('/api/changes/submit', { approve: true }),
      })}>Submit and approve</button>
      {approved ? (
        <button onClick={() => confirm({
          title: `Revoke your approval of ${mr}?`, lines: [`Takes back your approval as @${me}, and everyone on the merge request sees it.`], go: 'Revoke approval', actions: ['approve'],
          run: () => act('/api/changes/approve', { approve: false }),
        })}>Revoke approval</button>
      ) : (
        <button onClick={() => confirm({
          title: `Approve ${mr}?`, lines: [approve, ...own], go: 'Approve', actions: ['approve'],
          run: () => act('/api/changes/approve', { approve: true }),
        })}>Approve</button>
      )}
      {ask && <ConfirmDialog ask={ask} onClose={() => setAsk(undefined)} onWaive={onWaive} />}
    </div>
  )
}
