// The Local tab (docs/plan.md section 6.8): the person's checkouts, and the
// branches in each that need them.
import { useEffect, useState } from 'preact/hooks'
import { useAction } from './action'
import { api, bindings, post } from './api'
import { Hint, NeedsTool } from './guide'
import { Project } from './localproject'
import type { LocalView, Setup } from './types'
import { ago } from './ui'

type Adding = 'project' | 'root'
const adding: Record<Adding, { title: string; route: string }> = {
  project: { title: 'Add a folder', route: '/api/local/projects' },
  root: { title: 'Add a folder to scan', route: '/api/local/roots' },
}

// LocalTab reads the local view, which reads every checkout, when it is
// first shown. onCount hears how many branches need the person each time
// the view is read. Every edit answers with the view, read again. onOpen
// opens a change's page, and onOpenRun a run.
export function LocalTab({ setup, onCount, onOpen, onOpenRun }: {
  setup?: Setup; onCount?: (n: number) => void; onOpen?: (url: string) => void; onOpenRun?: (id: string) => void
}) {
  const [view, setView] = useState<LocalView>()
  const [preset, setPreset] = useState<string>()
  const presets = setup?.presets ?? []
  const tools = setup?.tools.filter((t) => !t.error) ?? []
  const unavailable = setup && tools.length === 0
    ? setup.tools.map((t) => `${t.tool}: ${t.error}`).join('; ') || 'No agent tool is available.'
    : undefined
  const [typing, setTyping] = useState<Adding>()
  const read = useAction()
  const apply = async (fn: () => Promise<LocalView>) => {
    const v = await read.run(fn)
    if (v) {
      setView(v)
      onCount?.(v.needs)
    }
    return v
  }
  const edit = (path: string, body: object) => apply(() => post<LocalView>(path, body))
  useEffect(() => { void apply(() => api<LocalView>('/api/local')) }, [])

  // The window asks with the native folder chooser, which only answers a
  // path. A browser has no chooser, and takes a typed path instead.
  const add = async (what: Adding) => {
    const choose = bindings()?.chooseFolder
    if (!choose) {
      setTyping(what)
      return
    }
    const path = await choose(adding[what].title)
    if (path) await edit(adding[what].route, { path })
  }
  const none = view && view.added.length + view.roots.length === 0

  return (
    <section class="local" aria-label="Local projects" aria-busy={read.busy}>
      <header class="local-head">
        <h2>Local projects</h2>
        {view && <span class="muted">read {ago(view.built)}</span>}
        <span class="local-actions">
          {presets.length > 1 && (
            <label>
              Review with{' '}
              <select value={preset ?? presets[0].name} onChange={(e) => setPreset((e.target as HTMLSelectElement).value)}>
                {presets.map((p) => <option value={p.name} key={p.name}>{p.name}</option>)}
              </select>
            </label>
          )}
          <button disabled={read.busy} onClick={() => void add('project')}>Add a folder</button>
          <button disabled={read.busy} onClick={() => void add('root')}>Add a folder to scan</button>
          <button disabled={read.busy} onClick={() => void apply(() => post<LocalView>('/api/local/refresh'))}>{read.busy ? 'Reading…' : 'Refresh'}</button>
        </span>
      </header>
      {typing && (
        <PathForm title={adding[typing].title} busy={read.busy} onCancel={() => setTyping(undefined)}
          onAdd={async (path) => { if (await edit(adding[typing].route, { path })) setTyping(undefined) }} />
      )}
      {read.error && <p class="alert" role="alert">⚠ {read.error}</p>}
      {!view ? (
        read.busy && <p class="muted">Reading your checkouts…</p>
      ) : (
        <>
          {none ? (
            <Hint id="local-empty" fallback={<p class="muted">
              No local projects yet. Add a checkout, or a folder such as ~/Workspace for vuoro to scan for checkouts.
            </p>}>
              <p><strong>Review your work before you push it.</strong> Add a checkout, or a folder such as ~/Workspace for vuoro to scan for checkouts, and each branch shows what it needs. A review here is yours alone and posts nothing.</p>
              {unavailable && <NeedsTool setup={setup} doing="Reviewing a branch" />}
            </Hint>
          ) : view.projects.length === 0 ? (
            <p class="muted">No checkouts to show in the folders you added.</p>
          ) : null}
          {view.stopped && <p class="muted">⚠ {view.stopped}; some checkouts may be missing.</p>}
          {view.projects.map((p) => (
            <Project project={p} quietDays={view.quiet_days} busy={read.busy} key={p.key}
              reviewing={{ preset: preset ?? presets[0]?.name, unavailable }} onOpen={onOpen} onOpenRun={onOpenRun}
              onHide={() => void edit('/api/local/hide', { key: p.key, hidden: true })}
              onBase={async (base) => !!(await edit('/api/local/base', { key: p.key, base }))} />
          ))}
          {!none && <Folders view={view} busy={read.busy} onEdit={(path, body) => void edit(path, body)} />}
        </>
      )}
    </section>
  )
}

// PathForm takes a typed absolute path, for a browser, which has no chooser.
function PathForm({ title, busy, onAdd, onCancel }: { title: string; busy: boolean; onAdd: (path: string) => Promise<void>; onCancel: () => void }) {
  const [path, setPath] = useState('')
  return (
    <form class="local-path" onSubmit={(e) => { e.preventDefault(); void onAdd(path.trim()) }}>
      <label>
        {title}:{' '}
        <input type="text" autoFocus value={path} placeholder="/Users/you/Workspace or ~/Workspace" aria-label={`${title}, as an absolute path`}
          onInput={(e) => setPath((e.target as HTMLInputElement).value)} />
      </label>
      <button class="primary" disabled={busy || !path.trim()}>Add</button>
      <button type="button" onClick={onCancel}>Cancel</button>
    </form>
  )
}

// Folders are what the person gave vuoro, each with a way to take it back:
// the checkouts added, the folders scanned, and the projects hidden.
function Folders({ view, busy, onEdit }: { view: LocalView; busy: boolean; onEdit: (path: string, body: object) => void }) {
  const broken = new Map(view.unreadable.map((u) => [u.path, u.error]))
  const parts = [
    `${view.added.length} added`,
    `${view.roots.length} scanned`,
    ...(view.hidden.length > 0 ? [`${view.hidden.length} hidden`] : []),
  ]
  return (
    <details class="local-folders" open={broken.size > 0}>
      <summary>Folders · {parts.join(', ')}</summary>
      <ul>
        {view.added.map((path) => (
          <li key={`a:${path}`}>
            <code>{path}</code> <span class="muted">checkout</span>
            {broken.has(path) && <span class="muted"> · ⚠ {broken.get(path)}</span>}{' '}
            <button class="link" disabled={busy} onClick={() => onEdit('/api/local/projects/remove', { path })}>Remove</button>
          </li>
        ))}
        {view.roots.map((path) => (
          <li key={`r:${path}`}>
            <code>{path}</code> <span class="muted">scanned for checkouts</span>{' '}
            <button class="link" disabled={busy} onClick={() => onEdit('/api/local/roots/remove', { path })}>Remove</button>
          </li>
        ))}
        {view.hidden.map((p) => (
          <li key={`h:${p.key}`}>
            {p.name} <span class="muted">{p.path} · hidden</span>{' '}
            <button class="link" disabled={busy} onClick={() => onEdit('/api/local/hide', { key: p.key, hidden: false })}>Show</button>
          </li>
        ))}
      </ul>
    </details>
  )
}
