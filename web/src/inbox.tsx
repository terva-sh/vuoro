import { useEffect, useRef, useState } from 'preact/hooks'
import { api } from './api'
import { changeNumber, refKey, type Ref } from './types'
import { ago } from './ui'

export interface InboxRow {
  ref: Ref; kind: string; account_key: string; account: string; title: string; web_url: string; draft: boolean
  roles: string[]
  turn: { owner: 'mine' | 'theirs'; since: string | null; reasons: { rule: string; text: string; since: string | null }[]; last_activity: string | null }
  snoozed_until?: string
  error?: string // the last refresh could not read it; its facts are the last ones read
}
export interface InboxView {
  accounts: { key: string; name: string; kind: string; host: string; username: string; last_attempt: string; last_success: string; error?: string; warning?: string }[]
  mine: InboxRow[]; theirs: InboxRow[]; snoozed: InboxRow[]
  unread?: InboxRow[] // discovered, never read: no title, link or turn yet
}
const knownTime = (value: string) => !value.startsWith('0001-') && Number.isFinite(Date.parse(value))

// Only a mounted inbox polls. All requests share one busy gate and cancellation;
// actions cannot race an older refresh response into restoring a prior snooze.
export function Inbox({ onOpen, opening = false }: { onOpen: (url: string) => void; opening?: boolean }) {
  const [view, setView] = useState<InboxView>()
  const [busy, setBusy] = useState(true)
  const [error, setError] = useState<string>()
  const refresh = useRef<() => void>()
  const snooze = useRef<(row: InboxRow, until: string) => void>()

  useEffect(() => {
    let active = true
    let working = true
    let timer: ReturnType<typeof setTimeout> | undefined
    let controller = new AbortController()
    const request = async (path: string, body: unknown) => {
      if (!active || working) return
      working = true
      clearTimeout(timer)
      setBusy(true)
      setError(undefined)
      controller = new AbortController()
      try {
        const next = await api<InboxView>(path, { method: 'POST', body: JSON.stringify(body), signal: controller.signal })
        if (active) setView(next)
      } catch (e) {
        if (active) setError(e instanceof Error ? e.message : String(e))
      } finally {
        if (active) {
          working = false
          setBusy(false)
          timer = setTimeout(() => void request('/api/inbox/refresh', {}), 60_000)
        }
      }
    }
    refresh.current = () => void request('/api/inbox/refresh', {})
    snooze.current = (row, until) => void request('/api/inbox/snooze', { ref: row.ref, until })
    void (async () => {
      try {
        const cached = await api<InboxView>('/api/inbox', { signal: controller.signal })
        if (active) setView(cached)
      } catch (e) {
        if (active) setError(e instanceof Error ? e.message : String(e))
      } finally {
        working = false
        if (active) void request('/api/inbox/refresh', {})
      }
    })()
    return () => { active = false; clearTimeout(timer); controller.abort(); refresh.current = undefined; snooze.current = undefined }
  }, [])

  const rows = (items: InboxRow[], sleeping = false) => (
    <ul class="inbox-rows">
      {items.map((row) => (
        <li key={refKey(row.ref)} class={`inbox-row ${row.turn.owner}`}>
          <div class="inbox-content">
            <div class="inbox-meta"><span>{row.kind === 'forgejo' ? 'Forgejo' : 'GitLab'} · {row.account}</span><span>{row.ref.project}{changeNumber(row.kind, row.ref.number)}</span>{row.draft && <span class="badge">Draft</span>}</div>
            <button class="link inbox-title" disabled={opening} onClick={() => onOpen(row.web_url)}>{row.title}</button>
            <div class="inbox-reasons">{row.turn.reasons.map((reason) => <p key={`${reason.rule}:${reason.text}`}>{reason.text}</p>)}</div>
            {row.error && <p class="inbox-stale" role="status">⚠ Last known state. {row.error}</p>}
            <div class="inbox-meta"><span>{row.roles.map((role) => role.charAt(0).toUpperCase() + role.slice(1)).join(' · ')}</span><span title={row.turn.since ? new Date(row.turn.since).toLocaleString() : undefined}>{row.turn.since ? `Waiting ${ago(row.turn.since).replace(/ ago$/, '')}` : 'Age unknown'}</span></div>
            {sleeping && row.snoozed_until && <p class="muted">Snoozed until {new Date(row.snoozed_until).toLocaleString()}</p>}
          </div>
          <div class="inbox-actions">
            <a href={row.web_url} target="_blank" rel="noreferrer">Open on {row.kind === 'forgejo' ? 'Forgejo' : 'GitLab'} ↗</a>
            <button disabled={busy} onClick={() => snooze.current?.(row, sleeping ? '0001-01-01T00:00:00Z' : new Date(Date.now() + 86_400_000).toISOString())}>{sleeping ? 'Restore' : 'Snooze 1 day'}</button>
          </div>
        </li>
      ))}
    </ul>
  )

  return (
    <section class="inbox" aria-label="Review inbox" aria-busy={busy}>
      <div class="inbox-heading"><div><h1>Your reviews</h1><p class="muted">The oldest unanswered work comes first.</p></div><button disabled={busy} onClick={() => refresh.current?.()}>{busy ? 'Refreshing…' : 'Refresh inbox'}</button></div>
      {error && <p class="alert" role="alert">{error}</p>}
      {!view && <p class="muted">Loading your inbox…</p>}
      {view && <>
        {view.accounts.length === 0 ? <p>Add your GitLab or Forgejo account in Vuoro’s configuration to discover your reviews. You can also open a change by URL above.</p> : <ul class="inbox-accounts">
          {view.accounts.map((account) => <li key={account.key} class={account.error ? 'inbox-account stale' : 'inbox-account'}>
            <strong>{account.name}</strong> <span class="muted">{account.host} · @{account.username}</span>
            <span class="muted">{knownTime(account.last_success) ? `Last refreshed ${ago(account.last_success)}` : 'No successful refresh yet'}</span>
            {account.error && <span class="alert" role="status">Cached results · {account.error}</span>}
            {!account.error && account.warning && <span class="alert" role="status">⚠ {account.warning}</span>}
          </li>)}
        </ul>}
        {view.accounts.length > 0 && <div class="inbox-groups">
          <section aria-label="Your turn"><h2>Your turn <span class="badge">{view.mine.length}</span></h2>{view.mine.length ? rows(view.mine) : <p class="muted">{view.accounts.some((a) => a.error) ? 'No actions in the available snapshot. Check account refresh errors above.' : 'No unanswered action is recorded for you.'}</p>}</section>
          <section aria-label="Their turn"><h2>Their turn <span class="badge">{view.theirs.length}</span></h2>{view.theirs.length ? rows(view.theirs) : <p class="muted">No changes are waiting on others in the available snapshot.</p>}</section>
        </div>}
        {(view.unread?.length ?? 0) > 0 && <section class="inbox-unread" aria-label="Could not read">
          <h2>Could not read <span class="badge">{view.unread!.length}</span></h2>
          <p class="muted">Found on the forge, but not read yet, so whose turn it is is unknown.</p>
          <ul class="inbox-rows">{view.unread!.map((row) => <li key={refKey(row.ref)} class="inbox-row">
            <div class="inbox-content">
              <div class="inbox-meta"><span>{row.kind === 'forgejo' ? 'Forgejo' : 'GitLab'} · {row.account}</span><span>{row.ref.project}{changeNumber(row.kind, row.ref.number)}</span><span>{row.roles.map((role) => role.charAt(0).toUpperCase() + role.slice(1)).join(' · ')}</span></div>
              <p class="inbox-stale">⚠ {row.error}</p>
            </div>
          </li>)}</ul>
        </section>}
        {view.snoozed.length > 0 && <details class="inbox-snoozed"><summary>Snoozed ({view.snoozed.length})</summary><p class="muted">A new forge event or the snooze date brings these back.</p>{rows(view.snoozed, true)}</details>}
      </>}
    </section>
  )
}
