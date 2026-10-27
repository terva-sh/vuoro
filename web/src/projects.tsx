// Project triage (docs/plan.md section 6.9): every open change in one project,
// grouped by where it sits and oldest first, with what holds each one.
import { useEffect, useRef, useState } from 'preact/hooks'
import { errorText } from './action'
import { api, post } from './api'
import { AgeBadge } from './inbox'
import { ProjectTriage } from './projecttriage'
import { usePoll } from './poll'
import { changeNumber, type Ref } from './types'
import { ago } from './ui'
import { StartUpdate } from './update'

export type Place = 'draft' | 'blocked' | 'author' | 'review' | 'unreviewed' | 'approved'
export interface ProjectEntry { key: string; kind: string; host: string; path: string; name: string; web_url: string }
export interface ProjectSummary extends ProjectEntry { open: number; last_success?: string; error?: string }
export interface ProjectList { projects: ProjectSummary[]; accounts: string[] }
export interface FoundProject { host: string; path: string; name: string; web_url: string; kind: string; account: string; added: boolean }
export interface SearchResult { projects: FoundProject[]; errors?: string[] }
export interface ProjectReason { place: Place; text: string; since: string | null; who?: string }
export interface ProjectRow {
  ref: Ref; kind: string; title: string; web_url: string; author: string; draft: boolean
  source_branch: string; target_branch: string; opened: string | null
  place: Place; since: string | null; reasons: ProjectReason[]; holder: string[]
  reviewers: { username: string; state: 'unreviewed' | 'reviewed' | 'requested_changes' | 'approved' | '' }[]
  assignees: string[]; labels: string[]; pipeline?: string; conflicts?: boolean
  threads: { total: number; unresolved: number; author: number; reviewers: number }
  stack?: { root: number; depth: number; on: number[]; above: number[] }
  can?: { reviewers: boolean; assignees: boolean; labels: boolean; draft: boolean }
  error?: string
}
// Changes are the person's edits to one change (D65): additions and removals,
// never a replacement set.
export interface Changes {
  add_reviewers?: string[]; remove_reviewers?: string[]; add_assignees?: string[]; remove_assignees?: string[]
  add_labels?: string[]; remove_labels?: string[]; draft?: boolean
}
export interface ProjectView {
  project: ProjectEntry; last_attempt: string; last_success: string; error?: string; warning?: string
  dependencies: boolean; open: number
  groups: { place: Place; rows: ProjectRow[] }[]
  unread: ProjectRow[]
}

export const placeTitles: Record<Place, string> = {
  draft: 'Draft',
  blocked: 'Blocked',
  author: 'Waiting on the author',
  review: 'Waiting on review',
  unreviewed: 'No reviewers',
  approved: 'Approved, not merged',
}
const knownTime = (value?: string | null) => !!value && !value.startsWith('0001-') && Number.isFinite(Date.parse(value))
const forgeName = (kind: string) => ({ forgejo: 'Forgejo', github: 'GitHub' })[kind] ?? 'GitLab'
// Every state carries a symbol and a word, so colour is never the only signal.
const reviewerMark: Record<string, [string, string]> = {
  approved: ['✓', 'approved'], requested_changes: ['✗', 'changes requested'], reviewed: ['◦', 'reviewed'], unreviewed: ['●', 'pending'], '': ['●', 'pending'],
}
const pipelineMark = (status: string): [string, string] =>
  status === 'success' ? ['✓', 'pipeline passed'] : status === 'failed' ? ['✗', 'pipeline failed'] : status === 'running' || status === 'pending' ? ['⟳', `pipeline ${status}`] : ['·', `pipeline ${status}`]

// ProjectsTab is the home page's third tab: the person's projects, adding one,
// and the selected project's changes.
export function ProjectsTab({ onOpen, opening = false, active = true }: { onOpen: (url: string) => void; opening?: boolean; active?: boolean }) {
  const [list, setList] = useState<ProjectList>()
  const [error, setError] = useState<string>()
  const [chosen, setChosen] = useState<string>()
  const [adding, setAdding] = useState(false)
  useEffect(() => {
    api<ProjectList>('/api/projects').then((l) => { setList(l); setAdding(l.projects.length === 0) }).catch((e: unknown) => setError(errorText(e)))
  }, [])
  const selected = list?.projects.find((p) => p.key === chosen) ?? list?.projects[0]
  const remove = async (key: string) => {
    try {
      setList(await post<ProjectList>('/api/projects/remove', { key }))
      if (chosen === key) setChosen(undefined)
    } catch (e) {
      setError(errorText(e))
    }
  }
  return (
    <section class="projects" aria-label="Projects">
      <div class="inbox-heading">
        <h1>Projects</h1>
        {list && list.projects.length > 0 && <nav class="project-picker" aria-label="Your projects">
          {list.projects.map((p) => (
            <button key={p.key} class={`chip ${selected?.key === p.key ? 'on' : ''}`} aria-pressed={selected?.key === p.key} onClick={() => setChosen(p.key)} title={p.name}>
              {p.path.split('/').pop()} <span class="muted">{p.open}</span>{p.error && <span aria-label="Last refresh failed"> ⚠</span>}
            </button>
          ))}
        </nav>}
        <button onClick={() => setAdding((v) => !v)} aria-expanded={adding}>{adding ? 'Close' : 'Add a project'}</button>
      </div>
      {error && <p class="alert" role="alert">{error}</p>}
      {adding && list && <AddProject list={list} onAdded={(l, key) => { setList(l); setChosen(key); setAdding(false) }} />}
      {list && list.projects.length === 0 && !adding && <p class="muted">No projects yet.</p>}
      {selected && <ProjectChanges key={selected.key} entry={selected} onOpen={onOpen} opening={opening} active={active && !adding}
        onRemove={() => void remove(selected.key)} onListed={(open, err) => setList((l) => l && { ...l, projects: l.projects.map((p) => p.key === selected.key ? { ...p, open, error: err } : p) })} />}
    </section>
  )
}

// AddProject searches every configured account, or takes a pasted URL.
function AddProject({ list, onAdded }: { list: ProjectList; onAdded: (l: ProjectList, key: string) => void }) {
  const [query, setQuery] = useState('')
  const [url, setURL] = useState('')
  const [found, setFound] = useState<SearchResult>()
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string>()
  const run = async (f: () => Promise<void>) => {
    setBusy(true)
    setError(undefined)
    try { await f() } catch (e) { setError(errorText(e)) } finally { setBusy(false) }
  }
  const add = (body: unknown) => run(async () => {
    const r = await post<{ project: ProjectEntry; list: ProjectList }>('/api/projects/add', body)
    onAdded(r.list, r.project.key)
  })
  if (list.accounts.length === 0) return <p class="project-add">Add the forge you work on in Settings, at the top right, and vuoro searches its projects.</p>
  return (
    <div class="project-add">
      <form onSubmit={(e) => { e.preventDefault(); void run(async () => setFound(await post<SearchResult>('/api/projects/search', { query }))) }}>
        <input type="search" placeholder="Search your projects…" value={query} aria-label="Search projects"
          onInput={(e) => setQuery((e.target as HTMLInputElement).value)} />
        <button class="primary" disabled={busy}>Search</button>
        <span class="muted">in {list.accounts.join(', ')}</span>
      </form>
      <form onSubmit={(e) => { e.preventDefault(); void add({ url: url.trim() }) }}>
        <input type="url" placeholder="…or paste a project URL" value={url} aria-label="Project URL"
          onInput={(e) => setURL((e.target as HTMLInputElement).value)} />
        <button disabled={busy || !url.trim()}>Add</button>
      </form>
      {error && <p class="alert" role="alert">{error}</p>}
      {found && <>
        {(found.errors ?? []).map((e) => <p key={e} class="inbox-stale" role="status">⚠ {e}</p>)}
        {found.projects.length === 0 && <p class="muted">No project matches.</p>}
        <ul class="project-found">
          {found.projects.map((p) => (
            <li key={`${p.host}/${p.path}`}>
              <span><strong>{p.name || p.path}</strong> <span class="mono muted">{p.host}/{p.path}</span> <span class="muted">· {p.account}</span></span>
              {p.added ? <span class="muted">Added</span> : <button disabled={busy} onClick={() => void add({ host: p.host, path: p.path })}>Add</button>}
            </li>
          ))}
        </ul>
      </>}
    </div>
  )
}

interface Filters { place?: Place; label?: string; author?: string; reviewer?: string; stack?: number }

// ProjectChanges reads one project's view: the cache first, then a refresh,
// and again every five minutes while the tab shows.
function ProjectChanges({ entry, onOpen, opening, active, onRemove, onListed }: {
  entry: ProjectSummary; onOpen: (url: string) => void; opening: boolean; active: boolean
  onRemove: () => void; onListed: (open: number, error?: string) => void
}) {
  const [view, setView] = useState<ProjectView>()
  const [busy, setBusy] = useState(true)
  const [error, setError] = useState<string>()
  const [filters, setFilters] = useState<Filters>({})
  const [selected, setSelected] = useState<number>()
  const life = useRef(new AbortController())
  const working = useRef(false)
  const settled = useRef(0)
  const shown = useRef(active)
  shown.current = active
  useEffect(() => () => life.current.abort(), [])
  const load = async (path: string, method: 'GET' | 'POST') => {
    const signal = life.current.signal
    if (signal.aborted || working.current) return
    working.current = true
    setBusy(true)
    setError(undefined)
    try {
      const next = await api<ProjectView>(path, method === 'POST' ? { method, body: '{}', signal } : { signal })
      if (!signal.aborted) { setView(next); onListed(next.open, next.error) }
    } catch (e) {
      if (!signal.aborted) setError(errorText(e))
    } finally {
      working.current = false
      settled.current = Date.now()
      if (!signal.aborted) setBusy(false)
    }
  }
  const refresh = () => void load(`/api/projects/${entry.key}/refresh`, 'POST')
  usePoll(async () => {
    if (settled.current === 0) {
      await load(`/api/projects/${entry.key}`, 'GET')
    } else {
      const wait = settled.current + 300_000 - Date.now()
      if (wait > 0) return wait
      // Nothing is read while another tab shows; check again in a minute.
      if (!shown.current) return 60_000
    }
    await load(`/api/projects/${entry.key}/refresh`, 'POST')
    return 300_000
  }, [entry.key])

  const all = view ? [...view.groups.flatMap((g) => g.rows)] : []
  const keep = (row: ProjectRow) =>
    (!filters.place || row.place === filters.place) &&
    (!filters.label || row.labels.includes(filters.label)) &&
    (!filters.author || row.author === filters.author) &&
    (!filters.reviewer || row.reviewers.some((r) => r.username === filters.reviewer)) &&
    (filters.stack === undefined || row.stack?.root === filters.stack)
  const groups = (view?.groups ?? []).map((g) => ({ ...g, rows: g.rows.filter(keep) })).filter((g) => g.rows.length > 0)
  const visible = groups.flatMap((g) => g.rows)
  const current = all.find((r) => r.ref.number === selected) ?? visible[0]
  const values = (f: (r: ProjectRow) => string[]) => [...new Set(all.flatMap(f))].sort()
  const stacks = [...new Set(all.filter((r) => r.stack).map((r) => r.stack!.root))].sort((a, b) => a - b)

  const keys = useRef<(e: KeyboardEvent) => void>()
  keys.current = (e) => {
    const target = e.target as HTMLElement | null
    if (!active || e.metaKey || e.ctrlKey || e.altKey || (target && /^(INPUT|TEXTAREA|SELECT)$/.test(target.tagName))) return
    const at = current ? visible.findIndex((r) => r.ref.number === current.ref.number) : -1
    const move = (to: number) => { const row = visible[Math.max(0, Math.min(visible.length - 1, to))]; if (row) setSelected(row.ref.number) }
    switch (e.key) {
      case 'j': move(at + 1); break
      case 'k': move(at - 1); break
      case 'Enter': if (current && !opening) onOpen(current.web_url); break
      case 'o': if (current) window.open(current.web_url, '_blank', 'noreferrer'); break
      case 'r': if (!busy) refresh(); break
      default: return
    }
    e.preventDefault()
  }
  useEffect(() => {
    const on = (e: KeyboardEvent) => keys.current?.(e)
    window.addEventListener('keydown', on)
    return () => window.removeEventListener('keydown', on)
  }, [])

  const select = (name: string, label: string, value: string | undefined, options: string[], set: (v?: string) => void) => (
    <label class="project-filter">{label}
      <select aria-label={name} value={value ?? ''} onChange={(e) => set((e.target as HTMLSelectElement).value || undefined)}>
        <option value="">Any</option>
        {options.map((o) => <option key={o} value={o}>{o}</option>)}
      </select>
    </label>
  )

  return (
    <div class="inbox project-view" aria-busy={busy}>
      <div class="project-head">
        <h2><a href={entry.web_url} target="_blank" rel="noreferrer">{entry.name || entry.path}</a></h2>
        <span class="muted">{forgeName(entry.kind)} · {view ? `${view.open} open` : `${entry.open} open`}</span>
        <span class="muted">{view && knownTime(view.last_success) ? `refreshed ${ago(view.last_success)}` : 'No successful refresh yet'}</span>
        <span class="project-actions">
          <button disabled={busy} onClick={refresh}>{busy ? 'Refreshing…' : 'Refresh'}</button>
          <button class="link" onClick={onRemove}>Remove from the list</button>
        </span>
      </div>
      {error && <p class="alert" role="alert">{error}</p>}
      {view?.error && <p class="alert" role="status">Cached results · {view.error}</p>}
      {view && !view.error && view.warning && <p class="inbox-stale" role="status">⚠ {view.warning}</p>}
      {view && !view.dependencies && view.open > 0 && <p class="muted project-note">This forge did not say which changes depend on others, so stacks come from target branches alone.</p>}
      {!view && <p class="muted">Loading the project…</p>}
      {view && <>
        <div class="inbox-summary" role="group" aria-label="Where changes sit">
          {view.groups.map((g) => (
            <button key={g.place} class={`chip ${filters.place === g.place ? 'on' : ''}`} aria-pressed={filters.place === g.place}
              onClick={() => setFilters((f) => ({ ...f, place: f.place === g.place ? undefined : g.place }))}>
              <strong>{g.rows.length}</strong> {placeTitles[g.place].toLowerCase()}
            </button>
          ))}
          {select('Label', 'Label', filters.label, values((r) => r.labels), (label) => setFilters((f) => ({ ...f, label })))}
          {select('Author', 'Author', filters.author, values((r) => [r.author]), (author) => setFilters((f) => ({ ...f, author })))}
          {select('Reviewer', 'Reviewer', filters.reviewer, values((r) => r.reviewers.map((x) => x.username)), (reviewer) => setFilters((f) => ({ ...f, reviewer })))}
          {stacks.length > 0 && select('Stack', 'Stack', filters.stack === undefined ? undefined : String(filters.stack), stacks.map(String),
            (s) => setFilters((f) => ({ ...f, stack: s === undefined ? undefined : Number(s) })))}
          {Object.values(filters).some((v) => v !== undefined) && <button class="link" onClick={() => setFilters({})}>Show all</button>}
        </div>
        {view.open > 0 && <ProjectTriage projectKey={entry.key} kind={entry.kind} rows={visible}
          onView={(next) => { setView(next); onListed(next.open, next.error) }} />}
        {view.open === 0 && <p class="muted">No open changes.</p>}
        <div class="inbox-layout">
          <div class="inbox-list">
            {groups.map((g) => (
              <section key={g.place} class="inbox-lane" aria-label={placeTitles[g.place]}>
                <h3>{placeTitles[g.place]} <span class="muted">{g.rows.length}</span></h3>
                <ul class="inbox-rows">
                  {g.rows.map((row) => {
                    const on = current?.ref.number === row.ref.number
                    return (
                      <li key={row.ref.number}>
                        <button class={`inbox-row ${on ? 'selected' : ''}`} aria-current={on ? 'true' : undefined}
                          onClick={() => setSelected(row.ref.number)} onDblClick={() => !opening && onOpen(row.web_url)}>
                          <AgeBadge since={row.since} />
                          <span class="inbox-title">{row.title}{row.draft && <span class="badge">Draft</span>}</span>
                          <span class="inbox-ref">{changeNumber(row.kind, row.ref.number)}</span>
                          <span class="inbox-why"><RowFacts row={row} /></span>
                        </button>
                      </li>
                    )
                  })}
                </ul>
              </section>
            ))}
            {view.unread.length > 0 && <section class="inbox-unread" aria-label="Could not read">
              <h2>Could not read <span class="badge">{view.unread.length}</span></h2>
              <p class="muted">Listed on the forge, but not read yet, so where they sit is unknown.</p>
              <ul class="inbox-rows">{view.unread.map((row) => <li key={row.ref.number} class="inbox-unread-row">
                <span class="mono">{changeNumber(row.kind, row.ref.number)}</span> <p class="inbox-stale">⚠ {row.error}</p>
              </li>)}</ul>
            </section>}
          </div>
          <aside class="inbox-detail" aria-label="Selected change">
            {current ? <Detail row={current} all={all} opening={opening} onOpen={onOpen} onSelect={setSelected} projectKey={entry.key}
              onView={(next) => { setView(next); onListed(next.open, next.error) }} />
              : <p class="muted">Pick a change to see what holds it.</p>}
            <p class="inbox-keys muted"><kbd>j</kbd>/<kbd>k</kbd> move · <kbd>↵</kbd> open · <kbd>o</kbd> open on the forge · <kbd>r</kbd> refresh</p>
          </aside>
        </div>
      </>}
    </div>
  )
}

// RowFacts is a row's second line: who it waits on and what a tech lead
// checks first.
function RowFacts({ row }: { row: ProjectRow }) {
  const parts: string[] = [row.author]
  if (row.holder.length > 0 && row.place !== 'draft') parts.push(`waits on ${row.holder.join(', ')}`)
  if (row.reviewers.length > 0) parts.push(row.reviewers.map((r) => `${reviewerMark[r.state][0]} ${r.username}`).join(' '))
  if (row.pipeline) parts.push(pipelineMark(row.pipeline).join(' '))
  if (row.conflicts) parts.push('⚠ conflicts')
  if (row.threads.unresolved > 0) parts.push(`💬 ${row.threads.unresolved} unresolved`)
  if (row.stack) parts.push(row.stack.on.length > 0 ? `⛓ on ${row.stack.on.map((n) => changeNumber(row.kind, n)).join(', ')}` : `⛓ base of ${row.stack.above.length}`)
  if (row.since === null && row.opened) parts.push(`opened ${ago(row.opened)}`)
  return <>{parts.join(' · ')}{row.error && <span class="inbox-stale"> ⚠ Last known state.</span>}</>
}

// Detail is the selected change: why it sits there, who holds it, and its
// stack drawn from the bottom up.
function Detail({ row, all, opening, onOpen, onSelect, projectKey, onView }: {
  row: ProjectRow; all: ProjectRow[]; opening: boolean; onOpen: (url: string) => void; onSelect: (n: number) => void
  projectKey: string; onView: (v: ProjectView) => void
}) {
  const edit = useEdit(projectKey, row, onView)
  const can = row.can ?? { reviewers: false, assignees: false, labels: false, draft: false }
  const chain = row.stack ? all.filter((r) => r.stack?.root === row.stack!.root).sort((a, b) => a.stack!.depth - b.stack!.depth || a.ref.number - b.ref.number) : []
  const waitsOnStackOnly = row.place === 'blocked' && row.holder.length === 0 && (row.stack?.on.length ?? 0) > 0
  return (
    <div class="inbox-detail-body">
      <h2>{row.title}</h2>
      <div class="inbox-meta">
        <span class="mono">{changeNumber(row.kind, row.ref.number)}</span>
        <span>by {row.author}</span>
        <span class="mono">{row.source_branch} → {row.target_branch}</span>
        {row.opened && <span>opened {ago(row.opened)}</span>}
        {row.draft && <span class="badge">Draft</span>}
      </div>
      <p><strong>{placeTitles[row.place]}</strong> · {row.since ? `for ${ago(row.since).replace(/ ago$/, '')}` : 'since an unrecorded time'}</p>
      {waitsOnStackOnly ? <p>It waits only on {row.stack!.on.map((n) => changeNumber(row.kind, n)).join(', ')} beneath it: reviewing that unblocks this one.</p>
        : row.holder.length > 0 ? <p>Waiting on {row.holder.join(', ')}.</p>
        : row.place === 'unreviewed' ? <p>Nobody holds it until it has a reviewer.</p> : null}
      <div class="actions">
        <button class="primary" disabled={opening} onClick={() => onOpen(row.web_url)}>Open in vuoro</button>
        <a class="button" href={row.web_url} target="_blank" rel="noreferrer">{row.place === 'approved' ? 'Merge on' : 'Open on'} {forgeName(row.kind)} ↗</a>
        {can.draft && <button disabled={edit.busy} onClick={() => edit.apply({ draft: !row.draft })}>{row.draft ? 'Mark ready' : 'Mark draft'}</button>}
        <StartUpdate url={row.web_url} target={row.target_branch} onOpen={onOpen} />
      </div>
      {edit.error && <p class="alert" role="alert">{edit.error}</p>}
      {row.error && <p class="inbox-stale" role="status">⚠ Last known state. {row.error}</p>}
      <ul class="inbox-reasons">{row.reasons.map((r, i) => <li key={i}>{r.text}{r.since && <span class="muted"> · {ago(r.since)}</span>}</li>)}</ul>
      <h3>Reviewers</h3>
      {row.reviewers.length === 0 ? <p class="muted">None.</p> : <ul class="project-people">
        {row.reviewers.map((r) => <li key={r.username}>{reviewerMark[r.state][0]} {r.username} <span class="muted">{reviewerMark[r.state][1]}</span>
          {can.reviewers && <Remove what={`reviewer ${r.username}`} busy={edit.busy} onClick={() => edit.apply({ remove_reviewers: [r.username] })} />}</li>)}
      </ul>}
      {can.reviewers && <Adder label="Add a reviewer" busy={edit.busy} options={edit.members} onFocus={edit.loadMembers} onAdd={(u) => edit.apply({ add_reviewers: [u] })} />}
      <h3>Assignees</h3>
      {row.assignees.length === 0 ? <p class="muted">None.</p> : <ul class="project-people">
        {row.assignees.map((u) => <li key={u}>{u}{can.assignees && <Remove what={`assignee ${u}`} busy={edit.busy} onClick={() => edit.apply({ remove_assignees: [u] })} />}</li>)}
      </ul>}
      {can.assignees && <Adder label="Assign" busy={edit.busy} options={edit.members} onFocus={edit.loadMembers} onAdd={(u) => edit.apply({ add_assignees: [u] })} />}
      <h3>Labels</h3>
      <p>{row.labels.length ? row.labels.map((l) => <span key={l} class="badge">{l}{can.labels && <Remove what={`label ${l}`} busy={edit.busy} onClick={() => edit.apply({ remove_labels: [l] })} />}</span>) : <span class="muted">None.</span>}</p>
      {can.labels && <Adder label="Add a label" busy={edit.busy} options={edit.labels} onFocus={edit.loadLabels} onAdd={(l) => edit.apply({ add_labels: [l] })} />}
      <p><span class="muted">Discussion:</span> {row.threads.total === 0 ? 'none' : `${row.threads.total} threads, ${row.threads.unresolved} unresolved` +
        (row.threads.unresolved ? ` (${row.threads.author} waiting on the author, ${row.threads.reviewers} on reviewers)` : '')}</p>
      {row.pipeline && <p><span class="muted">Pipeline:</span> {pipelineMark(row.pipeline).join(' ')}</p>}
      {chain.length > 1 && <>
        <h3>Stack</h3>
        <ol class="project-stack" aria-label="Stack, bottom first">
          {chain.map((r) => (
            <li key={r.ref.number} class={r.ref.number === row.ref.number ? 'on' : ''} style={{ marginLeft: `${r.stack!.depth * 12}px` }}>
              <button class="link" onClick={() => onSelect(r.ref.number)}>{changeNumber(r.kind, r.ref.number)} {r.title}</button>
              <span class="muted"> · {placeTitles[r.place].toLowerCase()}</span>
            </li>
          ))}
        </ol>
      </>}
    </div>
  )
}

// useEdit makes the person's edits to one change, one at a time, and loads
// the project's members and labels for the pickers when first asked.
function useEdit(projectKey: string, row: ProjectRow, onView: (v: ProjectView) => void) {
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string>()
  const [members, setMembers] = useState<string[]>()
  const [labels, setLabels] = useState<string[]>()
  useEffect(() => setError(undefined), [row.ref.number])
  return {
    busy, error, members, labels,
    apply: async (changes: Changes) => {
      setBusy(true)
      setError(undefined)
      try {
        onView(await post<ProjectView>(`/api/projects/${projectKey}/administer`, { number: row.ref.number, changes }))
      } catch (e) {
        setError(errorText(e))
      } finally {
        setBusy(false)
      }
    },
    loadMembers: () => {
      if (members) return
      setMembers([])
      api<{ username: string; name: string }[]>(`/api/projects/${projectKey}/members`).then((m) => setMembers(m.map((x) => x.username))).catch((e: unknown) => setError(errorText(e)))
    },
    loadLabels: () => {
      if (labels) return
      setLabels([])
      api<string[]>(`/api/projects/${projectKey}/labels`).then(setLabels).catch((e: unknown) => setError(errorText(e)))
    },
  }
}

function Remove({ what, busy, onClick }: { what: string; busy: boolean; onClick: () => void }) {
  return <button class="link remove" disabled={busy} aria-label={`Remove ${what}`} title={`Remove ${what}`} onClick={onClick}>✕</button>
}

// Adder is a field with the project's suggestions and an Add button.
function Adder({ label, busy, options, onFocus, onAdd }: { label: string; busy: boolean; options?: string[]; onFocus: () => void; onAdd: (v: string) => void }) {
  const [value, setValue] = useState('')
  const id = `adder-${label.replace(/\W+/g, '-').toLowerCase()}`
  return (
    <form class="project-adder" onSubmit={(e) => { e.preventDefault(); const v = value.trim(); if (v) { onAdd(v); setValue('') } }}>
      <input list={id} value={value} aria-label={label} placeholder={`${label}…`} onFocus={onFocus} onInput={(e) => setValue((e.target as HTMLInputElement).value)} />
      <datalist id={id}>{(options ?? []).map((o) => <option key={o} value={o} />)}</datalist>
      <button disabled={busy || !value.trim()}>Add</button>
    </form>
  )
}
