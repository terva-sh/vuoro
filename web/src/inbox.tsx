// The inbox (D38): a summary strip of what needs the person, "Your turn" in
// lanes, two-line rows with an age badge, and a detail pane for the selected
// change. Their turn, Dormant, and Snoozed fold away until asked for.
import { useEffect, useRef, useState } from 'preact/hooks'
import { errorText } from './action'
import { api } from './api'
import { changeNumber, refKey, type Ref } from './types'
import { ago } from './ui'

export interface InboxReason { rule: string; text: string; since: string | null; thread_id?: string; who?: string }
export interface InboxWaiting { rule: string; who: string; where: string; text: string; since: string | null }
export interface InboxRow {
  ref: Ref; kind: string; account_key: string; account: string; title: string; web_url: string; draft: boolean
  roles: string[]
  turn: { owner: 'mine' | 'theirs'; since: string | null; reasons: InboxReason[]; last_activity: string | null }
  snoozed_until?: string
  error?: string // the last refresh could not read it; its facts are the last ones read
  lane?: Lane
  summary?: string[] // the reasons, one line per kind, with counts
  waiting?: InboxWaiting[]
  quiet_since?: string | null // the last human activity
}
export interface InboxView {
  accounts: { key: string; name: string; kind: string; host: string; username: string; last_attempt: string; last_success: string; error?: string; warning?: string }[]
  mine: InboxRow[]; theirs: InboxRow[]; snoozed: InboxRow[]
  unread?: InboxRow[] // discovered, never read: no title, link or turn yet
  dormant?: InboxRow[] // no human activity for the dormant threshold
}
type Lane = 'yours' | 'replies' | 'reviews'

const knownTime = (value: string) => !value.startsWith('0001-') && Number.isFinite(Date.parse(value))
const forgeName = (kind: string) => (kind === 'forgejo' ? 'Forgejo' : 'GitLab')
const roleNames = (roles: string[]) => roles.map((role) => role.charAt(0).toUpperCase() + role.slice(1)).join(' · ')
// shortRef is the project's last segment and the change's number.
const shortRef = (row: InboxRow) => `${row.ref.project.split('/').pop()}${changeNumber(row.kind, row.ref.number)}`

// The snooze that only a new forge event ends. The server keeps it as this date.
export const untilChange = '9999-12-31T00:00:00Z'
const restore = '0001-01-01T00:00:00Z'
const tomorrowMorning = () => { const d = new Date(); d.setDate(d.getDate() + 1); d.setHours(8, 0, 0, 0); return d.toISOString() }
const inAWeek = () => new Date(Date.now() + 7 * 86_400_000).toISOString()
const snoozeLabel = (until: string) => (until.startsWith('9999-') ? 'until it changes' : `until ${new Date(until).toLocaleString()}`)

// The kinds the summary strip counts, in the order they press on the person.
const kinds = [
  { key: 'changes', label: 'changes requested on yours', rules: ['A2'] },
  { key: 'merge', label: 'ready to merge', rules: ['A3'] },
  { key: 'replies', label: 'replies owed', rules: ['A1', 'R3'] },
  { key: 'reviews', label: 'reviews requested', rules: ['R1', 'R2', 'R2b'] },
] as const
type Kind = (typeof kinds)[number]['key']
const lanes: { key: Lane; title: string }[] = [
  { key: 'yours', title: 'Your changes' },
  { key: 'replies', title: 'Replies owed' },
  { key: 'reviews', title: 'Reviews' },
]
const hasKind = (row: InboxRow, kind: Kind) => row.turn.reasons.some((r) => (kinds.find((k) => k.key === kind)!.rules as readonly string[]).includes(r.rule))

// AgeBadge is how long the change has waited, coloured blue, amber, then
// magenta as it ages, and always with the number.
function AgeBadge({ since }: { since: string | null }) {
  if (!since) return <span class="age unknown" title="Age unknown">?</span>
  const hours = (Date.now() - Date.parse(since)) / 3_600_000
  const label = hours < 1 ? `${Math.max(1, Math.round(hours * 60))}m` : hours < 48 ? `${Math.round(hours)}h` : `${Math.round(hours / 24)}d`
  const tone = hours < 48 ? 'fresh' : hours < 14 * 24 ? 'week' : 'old'
  return <span class={`age ${tone}`} title={`Waiting since ${new Date(since).toLocaleString()}`}>{label}</span>
}

// Only a mounted inbox polls. All requests share one busy gate and cancellation;
// actions cannot race an older refresh response into restoring a prior snooze.
export function Inbox({ onOpen, opening = false }: { onOpen: (url: string) => void; opening?: boolean }) {
  const [view, setView] = useState<InboxView>()
  const [busy, setBusy] = useState(true)
  const [error, setError] = useState<string>()
  const [filter, setFilter] = useState<Kind>()
  const [selected, setSelected] = useState<string>()
  const [unfolded, setUnfolded] = useState<Record<string, boolean>>({})
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
        if (active) setError(errorText(e))
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
        if (active) setError(errorText(e))
      } finally {
        working = false
        if (active) void request('/api/inbox/refresh', {})
      }
    })()
    return () => { active = false; clearTimeout(timer); controller.abort(); refresh.current = undefined; snooze.current = undefined }
  }, [])

  const mine = (view?.mine ?? []).filter((row) => !filter || hasKind(row, filter))
  const folds: { key: string; title: string; note: string; rows: InboxRow[] }[] = view ? [
    { key: 'theirs', title: 'Their turn', note: 'waiting on others', rows: view.theirs },
    { key: 'dormant', title: 'Dormant', note: 'no human activity for a long while', rows: view.dormant ?? [] },
    { key: 'snoozed', title: 'Snoozed', note: 'a new forge event or the snooze date brings these back', rows: view.snoozed },
  ] : []
  // visible is every row the list shows, in order, for the keyboard.
  const visible = [
    ...lanes.flatMap((lane) => mine.filter((row) => (row.lane ?? 'reviews') === lane.key)),
    ...folds.flatMap((f) => (unfolded[f.key] ? f.rows : [])),
  ]
  const all = view ? [...view.mine, ...view.theirs, ...(view.dormant ?? []), ...view.snoozed] : []
  const current = all.find((row) => refKey(row.ref) === selected) ?? visible[0]

  const keys = useRef<(e: KeyboardEvent) => void>()
  keys.current = (e) => {
    const target = e.target as HTMLElement | null
    if (e.metaKey || e.ctrlKey || e.altKey || (target && /^(INPUT|TEXTAREA|SELECT)$/.test(target.tagName))) return
    const at = current ? visible.findIndex((row) => refKey(row.ref) === refKey(current.ref)) : -1
    const move = (to: number) => { const row = visible[Math.max(0, Math.min(visible.length - 1, to))]; if (row) setSelected(refKey(row.ref)) }
    switch (e.key) {
      case 'j': move(at + 1); break
      case 'k': move(at - 1); break
      case 'Enter': if (current && !opening) onOpen(current.web_url); break
      case 'o': if (current) window.open(current.web_url, '_blank', 'noreferrer'); break
      case 's': if (current && !busy && !current.snoozed_until) snooze.current?.(current, tomorrowMorning()); break
      case 'r': if (!busy) refresh.current?.(); break
      case '/': (document.querySelector('input[aria-label="Merge request or pull request URL"]') as HTMLInputElement | null)?.focus(); break
      default: return
    }
    e.preventDefault()
  }
  useEffect(() => {
    const on = (e: KeyboardEvent) => keys.current?.(e)
    window.addEventListener('keydown', on)
    return () => window.removeEventListener('keydown', on)
  }, [])

  const rows = (items: InboxRow[]) => (
    <ul class="inbox-rows">
      {items.map((row) => {
        const key = refKey(row.ref)
        const on = current && refKey(current.ref) === key
        return (
          <li key={key}>
            <button class={`inbox-row ${row.turn.owner} ${on ? 'selected' : ''}`} aria-current={on ? 'true' : undefined}
              onClick={() => setSelected(key)} onDblClick={() => !opening && onOpen(row.web_url)}>
              <AgeBadge since={row.turn.since} />
              <span class="inbox-title">{row.title}{row.draft && <span class="badge">Draft</span>}</span>
              <span class="inbox-ref">{shortRef(row)}</span>
              <span class="inbox-why"><span class="muted">{roleNames(row.roles)} ·</span> {(row.summary ?? row.turn.reasons.map((r) => r.text)).join(' · ')}
                {row.error && <span class="inbox-stale"> ⚠ Last known state.</span>}
              </span>
            </button>
          </li>
        )
      })}
    </ul>
  )

  const counts = kinds.map((k) => ({ ...k, n: (view?.mine ?? []).filter((row) => hasKind(row, k.key)).length }))

  return (
    <section class="inbox" aria-label="Review inbox" aria-busy={busy}>
      <div class="inbox-heading">
        <h1>Your reviews</h1>
        {view && view.accounts.length > 0 && <ul class="inbox-accounts">
          {view.accounts.map((account) => <li key={account.key} class={account.error ? 'inbox-account stale' : 'inbox-account'}>
            <span class={account.error ? 'dot stale' : 'dot'} aria-hidden="true">●</span>
            <strong>{account.name}</strong> <span class="muted">@{account.username}</span>
            <span class="muted">{knownTime(account.last_success) ? `refreshed ${ago(account.last_success)}` : 'No successful refresh yet'}</span>
            {account.error && <span class="alert" role="status">Cached results · {account.error}</span>}
            {!account.error && account.warning && <span class="alert" role="status">⚠ {account.warning}</span>}
          </li>)}
        </ul>}
        <button disabled={busy} onClick={() => refresh.current?.()}>{busy ? 'Refreshing…' : 'Refresh inbox'}</button>
      </div>
      {error && <p class="alert" role="alert">{error}</p>}
      {!view && <p class="muted">Loading your inbox…</p>}
      {view && view.accounts.length === 0 && <p>Add your GitLab or Forgejo account in Vuoro’s configuration to discover your reviews. You can also open a change by URL above.</p>}
      {view && view.accounts.length > 0 && <>
        <div class="inbox-summary" role="group" aria-label="What needs you">
          <span class="muted">Needs you:</span>
          {counts.map((k) => (
            <button key={k.key} class={`chip ${filter === k.key ? 'on' : ''}`} aria-pressed={filter === k.key} disabled={k.n === 0 && filter !== k.key}
              onClick={() => setFilter(filter === k.key ? undefined : k.key)}>
              <strong>{k.n}</strong> {k.label}
            </button>
          ))}
          {filter && <button class="link" onClick={() => setFilter(undefined)}>Show all</button>}
        </div>
        <div class="inbox-layout">
          <div class="inbox-list">
            <section aria-label="Your turn">
              <h2>Your turn <span class="badge">{view.mine.length}</span></h2>
              {view.mine.length === 0 && <p class="muted">{view.accounts.some((a) => a.error) ? 'No actions in the available snapshot. Check account refresh errors above.' : 'No unanswered action is recorded for you.'}</p>}
              {lanes.map((lane) => {
                const items = mine.filter((row) => (row.lane ?? 'reviews') === lane.key)
                return items.length > 0 && <section key={lane.key} class="inbox-lane" aria-label={lane.title}><h3>{lane.title} <span class="muted">{items.length}</span></h3>{rows(items)}</section>
              })}
            </section>
            {folds.map((f) => f.rows.length > 0 && (
              <details key={f.key} class="inbox-fold" aria-label={f.title} open={!!unfolded[f.key]}
                onToggle={(e) => { const open = (e.target as HTMLDetailsElement).open; setUnfolded((u) => (u[f.key] === open ? u : { ...u, [f.key]: open })) }}>
                <summary><span class="fold-title">{f.title}</span> <span class="badge">{f.rows.length}</span> <span class="muted">{f.note}</span></summary>
                {unfolded[f.key] && rows(f.rows)}
              </details>
            ))}
            {(view.unread?.length ?? 0) > 0 && <section class="inbox-unread" aria-label="Could not read">
              <h2>Could not read <span class="badge">{view.unread!.length}</span></h2>
              <p class="muted">Found on the forge, but not read yet, so whose turn it is is unknown.</p>
              <ul class="inbox-rows">{view.unread!.map((row) => <li key={refKey(row.ref)} class="inbox-unread-row">
                <span class="mono">{row.ref.project}{changeNumber(row.kind, row.ref.number)}</span> <span class="muted">{roleNames(row.roles)}</span>
                <p class="inbox-stale">⚠ {row.error}</p>
              </li>)}</ul>
            </section>}
          </div>
          <aside class="inbox-detail" aria-label="Selected change">
            {current ? <Detail row={current} busy={busy} opening={opening} onOpen={onOpen} snooze={(until) => snooze.current?.(current, until)} />
              : <p class="muted">Nothing is waiting on you. Pick a change from the list to see what it needs.</p>}
            <p class="inbox-keys muted"><kbd>j</kbd>/<kbd>k</kbd> move · <kbd>↵</kbd> open · <kbd>o</kbd> open on the forge · <kbd>s</kbd> snooze until tomorrow · <kbd>r</kbd> refresh · <kbd>/</kbd> open by URL</p>
          </aside>
        </div>
      </>}
    </section>
  )
}

// Detail is the selected change: what it waits for, and what can be done.
function Detail({ row, busy, opening, onOpen, snooze }: {
  row: InboxRow; busy: boolean; opening: boolean; onOpen: (url: string) => void; snooze: (until: string) => void
}) {
  const replies = (row.waiting ?? []).length
  return (
    <div class="inbox-detail-body">
      <h2>{row.title}</h2>
      <div class="inbox-meta">
        <span class="mono">{row.ref.project}{changeNumber(row.kind, row.ref.number)}</span>
        <span>{forgeName(row.kind)} · {row.account}</span>
        <span>{roleNames(row.roles)}</span>
        {row.draft && <span class="badge">Draft</span>}
        <span>{row.turn.since ? `waiting ${ago(row.turn.since).replace(/ ago$/, '')}` : 'Age unknown'}</span>
        {row.quiet_since && <span title="The last activity by a person">last human activity {ago(row.quiet_since)}</span>}
      </div>
      <div class="actions">
        <button class="primary" disabled={opening} onClick={() => onOpen(row.web_url)}>Open in vuoro</button>
        <a class="button" href={row.web_url} target="_blank" rel="noreferrer">Open on {forgeName(row.kind)} ↗</a>
      </div>
      <div class="actions snooze" role="group" aria-label="Snooze">
        {row.snoozed_until ? <>
          <span class="muted">Snoozed {snoozeLabel(row.snoozed_until)}.</span>
          <button disabled={busy} onClick={() => snooze(restore)}>Restore</button>
        </> : <>
          <span class="muted">Snooze:</span>
          <button disabled={busy} onClick={() => snooze(tomorrowMorning())}>Until tomorrow</button>
          <button disabled={busy} onClick={() => snooze(inAWeek())}>For a week</button>
          <button disabled={busy} onClick={() => snooze(untilChange)} title="Until a new event on the forge">Until it changes</button>
        </>}
      </div>
      {row.error && <p class="inbox-stale" role="status">⚠ Last known state. {row.error}</p>}
      <ul class="inbox-reasons">{(row.summary ?? row.turn.reasons.map((r) => r.text)).map((line) => <li key={line}>{line}</li>)}</ul>
      {replies > 0 && <>
        <h3>{row.turn.owner === 'mine' ? 'Waiting for you' : 'Threads'} ({replies})</h3>
        <ul class="inbox-threads">
          {row.waiting!.map((w, i) => (
            <li key={i}>
              <strong>{w.who}</strong> <span class="mono muted">{w.where}</span>
              {w.since && <span class="muted"> · {ago(w.since)}</span>}
              {w.text && <p>{w.text}</p>}
            </li>
          ))}
        </ul>
      </>}
    </div>
  )
}
