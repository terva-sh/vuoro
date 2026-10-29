// The runs so far, on the home page: a folded card, one row per change, with
// the change's earlier runs under its latest one.
import { useState } from 'preact/hooks'
import { changeNumber, refKey, type Run } from './types'
import { ago, cost, short, StatusBadge } from './ui'

export interface RunGroup { key: string; runs: Run[] } // newest run first

// groupRuns gathers runs by the change they reviewed, keeping the order of
// each change's newest run. Local runs group by repository and title, since
// they have no change.
export function groupRuns(runs: Run[]): RunGroup[] {
  const groups = new Map<string, Run[]>()
  for (const r of [...runs].sort((a, b) => b.created.localeCompare(a.created))) {
    const key = r.kind === 'local' ? `local:${r.ref.project}:${r.title}` : `${r.kind}:${refKey(r.ref)}`
    groups.set(key, [...(groups.get(key) ?? []), r])
  }
  return [...groups].map(([key, runs]) => ({ key, runs }))
}

const where = (r: Run) => (r.kind === 'local' ? `local commits of ${r.ref.project}` : `${r.ref.project}${changeNumber(r.kind, r.ref.number)}`)
const pending = (r: Run) => r.drafts.filter((d) => d.status === 'pending').length

// RunList is the runs so far. A run of local commits, from vuoro review, has
// no forge page to open, so it opens by its own id, or is listed without a
// link where nothing can open it.
export function RunList({ runs, onOpen, onOpenRun }: { runs: Run[]; onOpen: (url: string) => void; onOpenRun?: (id: string) => void }) {
  const [unfolded, setUnfolded] = useState<Record<string, boolean>>({})
  if (runs.length === 0) return null
  const groups = groupRuns(runs)
  const open = runs.filter((r) => r.status !== 'closed').length
  const total = runs.reduce((t, r) => t + r.cost_usd, 0)
  const title = (r: Run) =>
    r.kind === 'local'
      ? onOpenRun ? <button class="link" onClick={() => onOpenRun(r.id)}>{r.title}</button> : <span>{r.title}</span>
      : <button class="link" onClick={() => onOpen(r.web_url)}>{r.title}</button>
  return (
    <details class="runs-card">
      <summary>
        <h2>Runs</h2>
        <span class="badge">{runs.length}</span>
        <span class="muted">
          across {groups.length} change{groups.length === 1 ? '' : 's'} · {open} not closed · {cost(total, runs.some((r) => r.cost_unknown))}
        </span>
      </summary>
      <div class="runs-scroll">
        <table class="runs-table">
          <thead>
            <tr><th>Status</th><th>Change</th><th>Runs</th><th>Drafts</th><th>Cost</th><th>Latest</th></tr>
          </thead>
          <tbody>
            {groups.map(({ key, runs: rs }) => {
              const r = rs[0]
              const more = rs.length > 1
              const shown = unfolded[key]
              return [
                <tr key={key} class="run-group">
                  <td><StatusBadge status={r.status} /></td>
                  <td class="run-change">
                    {title(r)}
                    <div class="muted mono">{where(r)} at {short(r.head)}</div>
                  </td>
                  <td>
                    {more ? (
                      <button class="link" aria-expanded={!!shown} onClick={() => setUnfolded({ ...unfolded, [key]: !shown })}>
                        {shown ? '▾' : '▸'} {rs.length}
                      </button>
                    ) : 1}
                  </td>
                  <td>{pending(r)}</td>
                  <td>{cost(rs.reduce((t, x) => t + x.cost_usd, 0), rs.some((x) => x.cost_unknown))}</td>
                  <td class="muted">{ago(r.created)}</td>
                </tr>,
                ...(more && shown ? rs.map((x) => (
                  <tr key={x.id} class="run-earlier">
                    <td><StatusBadge status={x.status} /></td>
                    <td class="mono">
                      {x.kind === 'local' && onOpenRun ? <button class="link" onClick={() => onOpenRun(x.id)}>{short(x.head)}</button> : short(x.head)}
                    </td>
                    <td />
                    <td>{pending(x)}</td>
                    <td>{cost(x.cost_usd, x.cost_unknown)}</td>
                    <td class="muted">{ago(x.created)}</td>
                  </tr>
                )) : []),
              ]
            })}
          </tbody>
        </table>
      </div>
    </details>
  )
}
