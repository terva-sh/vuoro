// Asking first. An action that acts on the forge as the person, where others
// see it, asks before it runs, unless the person waived asking for it.
import { useRef, useState } from 'preact/hooks'
import { api, post } from './api'
import { Modal } from './modal'
import { changeNumber, type Change, type Setup } from './types'

// The actions that ask first, as the server names them for waivers.
export type Confirmable = 'submit' | 'approve' | 'request_changes'

const actionWords: Record<Confirmable, string> = { submit: 'submitting a review', approve: 'approving', request_changes: 'requesting changes' }

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
  return (
    <Modal title={ask.title} onClose={onClose} focus={cancel}>
      <ul>{ask.lines.map((l) => <li key={l}>{l}</li>)}</ul>
      <label class="waive">
        <input type="checkbox" checked={waive} onChange={(e) => setWaive((e.target as HTMLInputElement).checked)} />
        {' '}Don't ask again before {ask.actions.map((a) => actionWords[a]).join(' or ')}
      </label>
      <div class="actions">
        <button ref={cancel} onClick={onClose}>Cancel</button>
        <button class="primary" onClick={() => { if (waive) onWaive(ask.actions); onClose(); ask.run() }}>{ask.go}</button>
      </div>
    </Modal>
  )
}

// SubmitBar is the person's pending comments, and the actions that publish
// them with a verdict or approve, each of which asks first unless waived. It
// offers only the verdicts the forge's capabilities name.
export function SubmitBar({ change, me, pending, approved, waived, act, onWaive }: {
  change: Change; me: string; pending: number; approved: boolean; waived: string[]
  act: (path: string, body: Record<string, unknown>) => void
  onWaive: (actions: Confirmable[]) => void
}) {
  const [ask, setAsk] = useState<Ask>()
  const caps = change.capabilities
  const mr = changeNumber(change.kind, change.ref.number)
  const confirm = (a: Ask) => (a.actions.every((x) => waived.includes(x)) ? a.run() : setAsk(a))
  const comments = `${pending} pending comment${pending === 1 ? '' : 's'}`
  const publish = pending > 0
    ? `Publishes your ${comments} as @${me}, and everyone on ${mr} sees them.`
    : 'You have no pending comments, so this only marks your review as submitted.'
  const own = change.author.username === me
    ? ['You opened this change. A forge may refuse an author’s own verdict.']
    : []
  const approve = `Approves ${mr} as @${me}, and everyone on it sees it.`
  const changes = `Asks for changes on ${mr} as @${me}, and everyone on it sees it.`
  return (
    <div class="submit-bar" data-tour="submit">
      <span>✎ {comments} of yours, seen only by you until you submit</span>
      <button class="primary" disabled={pending === 0} onClick={() => confirm({
        title: `Submit your review of ${mr}?`, lines: [publish], go: 'Submit', actions: ['submit'],
        run: () => act('/api/changes/submit', {}),
      })}>Submit</button>
      {caps.approve && <button disabled={approved} onClick={() => confirm({
        title: `Submit your review of ${mr} and approve it?`, lines: [publish, approve, ...own], go: 'Submit and approve', actions: ['submit', 'approve'],
        run: () => act('/api/changes/submit', { verdict: 'approved' }),
      })}>Submit and approve</button>}
      {caps.request_changes && <button onClick={() => confirm({
        title: `Submit your review of ${mr} and request changes?`, lines: [publish, changes, ...own], go: 'Request changes', actions: ['submit', 'request_changes'],
        run: () => act('/api/changes/submit', { verdict: 'changes_requested' }),
      })}>Request changes</button>}
      {approved ? caps.unapprove && (
        <button onClick={() => confirm({
          title: `Revoke your approval of ${mr}?`, lines: [`Takes back your approval as @${me}, and everyone on ${mr} sees it.`], go: 'Revoke approval', actions: ['approve'],
          run: () => act('/api/changes/approve', { approve: false }),
        })}>Revoke approval</button>
      ) : caps.approve && (
        <button onClick={() => confirm({
          title: `Approve ${mr}?`, lines: [approve, ...own], go: 'Approve', actions: ['approve'],
          run: () => act('/api/changes/approve', { approve: true }),
        })}>Approve</button>
      )}
      {ask && <ConfirmDialog ask={ask} onClose={() => setAsk(undefined)} onWaive={onWaive} />}
    </div>
  )
}
