// The Local tab (docs/plan.md section 6.8): the person's checkouts, and the
// branches in each that need them.
import { useEffect, useState } from 'preact/hooks'
import { useAction } from './action'
import { api, post } from './api'
import type { LocalProject, LocalView } from './types'
import { ago } from './ui'

// LocalTab reads the local view, which reads every checkout, when it is
// first shown. onCount hears how many branches need the person each time
// the view is read.
export function LocalTab({ onCount }: { onCount?: (n: number) => void }) {
  const [view, setView] = useState<LocalView>()
  const read = useAction()
  const load = async (path: string, refresh: boolean) => {
    const v = await read.run(() => (refresh ? post<LocalView>(path) : api<LocalView>(path)))
    if (v) {
      setView(v)
      onCount?.(v.needs)
    }
  }
  useEffect(() => { void load('/api/local', false) }, [])

  return (
    <section class="local" aria-label="Local projects" aria-busy={read.busy}>
      <header class="local-head">
        <h2>Local projects</h2>
        {view && <span class="muted">read {ago(view.built)}</span>}
        <button disabled={read.busy} onClick={() => void load('/api/local/refresh', true)}>{read.busy ? 'Reading…' : 'Refresh'}</button>
      </header>
      {read.error && <p class="alert" role="alert">⚠ {read.error}</p>}
      {!view ? (
        read.busy && <p class="muted">Reading your checkouts…</p>
      ) : view.projects.length === 0 ? (
        <p class="muted">
          No local projects yet. vuoro lists the checkouts you add, and the ones it finds in the folders you ask it to scan.
        </p>
      ) : (
        <>
          {view.stopped && <p class="muted">⚠ {view.stopped}; some checkouts may be missing.</p>}
          {view.projects.map((p) => <Project project={p} key={p.key} />)}
        </>
      )}
    </section>
  )
}

function Project({ project: p }: { project: LocalProject }) {
  const needing = p.branches.filter((b) => b.needs.length > 0)
  const quiet = p.branches.length - needing.length
  return (
    <section class="local-project" aria-label={p.name}>
      <h3>
        {p.name} <span class="badge">{needing.length}</span>{' '}
        <span class="muted">{p.path}{p.base && ` · against ${p.base}`}</span>
      </h3>
      {p.error ? (
        <p class="muted">⚠ {p.error}</p>
      ) : (
        <ul class="local-branches">
          {needing.map((b) => (
            <li key={b.name}>
              <code>{b.name}</code> <span class="muted">{b.ahead} ahead · {ago(b.committed)}</span> — {b.needs.join('; ')}
            </li>
          ))}
          {quiet > 0 && <li class="muted">{quiet === 1 ? '1 more branch needs' : `${quiet} more branches need`} nothing</li>}
          {p.branches.length === 0 && <li class="muted">No branches beyond {p.base}.</li>}
        </ul>
      )}
    </section>
  )
}
