import { useMemo, useState } from 'preact/hooks'
import { fileID, FileView, ThreadView } from './diffview'
import { RunPanel } from './run'
import type { Change, Draft, Reviewer, Setup } from './types'
import { ago, Prose, Section, short } from './ui'

const reviewMark: Record<Reviewer['state'], string> = {
  unreviewed: '○ not yet reviewed',
  reviewed: '◐ reviewed',
  requested_changes: '✗ requested changes',
  approved: '✓ approved',
}

type Tab = 'overview' | 'changes' | 'review'

export function ChangeView({ change, setup }: { change: Change; setup?: Setup }) {
  const [tab, setTab] = useState<Tab>('changes')
  // The drafts of the run the review tab shows, placed on the diff too.
  const [drafts, setDrafts] = useState<Draft[]>([])
  const files = change.files ?? []
  const threads = change.threads ?? []
  const general = threads.filter((t) => !t.anchor && t.notes.some((n) => !n.system))
  const open = threads.filter((t) => t.resolvable && !t.resolved).length

  const byFile = useMemo(() => {
    const m = new Map<string, typeof threads>()
    for (const t of threads) {
      if (!t.anchor) continue
      m.set(t.anchor.path, [...(m.get(t.anchor.path) ?? []), t])
    }
    return m
  }, [threads])

  const pick = (path: string) => {
    setTab('changes')
    requestAnimationFrame(() => document.getElementById(fileID(path))?.scrollIntoView({ block: 'start' }))
  }

  return (
    <div class="change">
      <header class="change-head">
        <h1>
          {change.draft && <span class="badge">draft</span>} {change.title}
        </h1>
        <div class="change-meta">
          <a href={change.web_url} target="_blank" rel="noreferrer">{change.ref.project}!{change.ref.number}</a>
          <span class="badge">{change.state}</span>
          <span>@{change.author.username}</span>
          <span class="mono">{change.source_branch} → {change.target_branch}</span>
          <span class="mono muted">head {short(change.diff.head)}</span>
          <span class="muted">updated {ago(change.updated)}</span>
          {change.mergeability && <span class="muted">merge: {change.mergeability.replace(/_/g, ' ').toLowerCase()}</span>}
        </div>
        {(change.reviewers?.length ?? 0) > 0 && (
          <ul class="reviewers">
            {change.reviewers!.map((r) => (
              <li key={r.username} class={`review-${r.state}`}>
                @{r.username}{r.bot && ' (bot)'}: {reviewMark[r.state]}
              </li>
            ))}
          </ul>
        )}
      </header>
      <nav class="tabs" role="tablist">
        {(
          [
            ['overview', `Overview${general.length ? ` (${general.length})` : ''}`],
            ['changes', `Changes (${files.length} files, ${open} open threads)`],
            ['review', `Review${drafts.length ? ` (${drafts.filter((d) => d.status === 'pending').length} drafts)` : ''}`],
          ] as [Tab, string][]
        ).map(([t, label]) => (
          <button role="tab" aria-selected={tab === t} class={tab === t ? 'tab on' : 'tab'} onClick={() => setTab(t)} key={t}>
            {label}
          </button>
        ))}
      </nav>
      {tab === 'overview' && (
        <>
          <Section title="Description">
            {change.description.trim() ? <Prose text={change.description} /> : <p class="muted">No description.</p>}
          </Section>
          <Section title="Discussion">
            {general.length === 0 ? <p class="muted">No general discussion.</p> : general.map((t) => <ThreadView thread={t} key={t.id} />)}
          </Section>
        </>
      )}
      {tab === 'changes' && (
        <div class="changes">
          <nav class="file-list">
            {files.map((f) => (
              <button class="link" key={f.new_path} onClick={() => pick(f.new_path)}>
                <span class="mono">{f.new_path}</span>
                {(byFile.get(f.new_path)?.length ?? 0) + drafts.filter((d) => d.anchor?.path === f.new_path).length > 0 && ' 💬'}
              </button>
            ))}
          </nav>
          <div class="files">
            {files.map((f) => (
              <FileView
                key={f.new_path}
                file={f}
                threads={byFile.get(f.new_path) ?? (f.old_path !== f.new_path ? byFile.get(f.old_path) ?? [] : [])}
                drafts={drafts.filter((d) => d.status === 'pending' && d.anchor?.path === f.new_path)}
              />
            ))}
          </div>
        </div>
      )}
      <div hidden={tab !== 'review'}>
        <RunPanel change={change} setup={setup} onPick={pick} onDrafts={setDrafts} />
      </div>
    </div>
  )
}
