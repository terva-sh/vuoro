import { useMemo, useState } from 'preact/hooks'
import { DraftCard, FileList, fileID, FileView, manyFiles, MyDraft, ThreadView } from './diffview'
import { HeadBadge, HeadContext, useRunHead } from './head'
import { RunPanel } from './run'
import { TriagePanel } from './triage'
import { SummaryPanel } from './summary'
import { changeNumber, type Change, type Draft, type Reviewer, type Setup } from './types'
import { ago, CommentBubble, Prose, Section, short, worstSeverity } from './ui'
import { SubmitBar, waive } from './confirm'
import { projectURL, RefsContext } from './markdown'
import { Composer, makeWriter, WriteContext } from './write'

const reviewMark: Record<Reviewer['state'], string> = {
  unreviewed: '○ not yet reviewed',
  reviewed: '◐ reviewed',
  requested_changes: '✗ requested changes',
  approved: '✓ approved',
}

type Tab = 'overview' | 'changes' | 'review' | 'threads' | 'summary'

export function ChangeView({ change, setup, onChange, onSetup }: {
  change: Change; setup?: Setup; onChange: (c: Change) => void; onSetup: (s: Setup) => void
}) {
  const [chosenTab, setTab] = useState<Tab>()
  const [error, setError] = useState<string>()
  const [runID, setRunID] = useState<string>()
  const [reviewedHead, setReviewedHead] = useState<string>()
  const headState = useRunHead(runID, reviewedHead, change)
  const moved = !!reviewedHead && reviewedHead !== change.diff.head
  const [commenting, setCommenting] = useState(false)
  const writer = useMemo(() => makeWriter(change.web_url, (c) => { setError(undefined); onChange(c) }, setError), [change.web_url, onChange])
  // Writing needs the person's token: a forge in the config with this host.
  const me = setup?.forges.find((f) => { try { return new URL(f.url).host === change.ref.host } catch { return false } })
  const isAuthor = !!me && me.username.toLowerCase() === change.author.username.toLowerCase()
  const tab = chosenTab ?? (isAuthor ? 'threads' : 'changes')
  const canDraft = !!me && change.capabilities.drafts
  const mine = change.drafts ?? []
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

  // picked is the file last picked from the list or a finding, and how many
  // picks there have been, so picking it again opens it again.
  const [picked, setPicked] = useState<{ path: string; n: number }>()
  const pick = (path: string) => {
    setTab('changes')
    setPicked((p) => ({ path, n: (p?.n ?? 0) + 1 }))
    requestAnimationFrame(() => document.getElementById(fileID(path))?.scrollIntoView({ block: 'start' }))
  }

  const approved = !!me && (change.reviewers ?? []).some((r) => r.username === me.username && r.state === 'approved')
  const act = (path: string, body: Record<string, unknown>) => void writer.act(path, body).catch(() => undefined)

  return (
    <RefsContext.Provider value={{ base: projectURL(change.web_url), kind: change.kind }}>
    <WriteContext.Provider value={canDraft ? writer : undefined}>
    <HeadContext.Provider value={headState}>
    <div class="change">
      <header class="change-head">
        <h1>
          {change.draft && <span class="badge">draft</span>} {change.title}
        </h1>
        <div class="change-meta">
          <a href={change.web_url} target="_blank" rel="noreferrer">{change.ref.project}{changeNumber(change.kind, change.ref.number)}</a>
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
        {me && canDraft ? (
          <SubmitBar change={change} me={me.username} pending={mine.length} approved={approved} waived={setup?.waived ?? []} act={act}
            onWaive={(actions) => void waive(actions, false).then(onSetup).catch(() => undefined)} />
        ) : (
          <p class="muted">{change.kind === 'forgejo' ? 'Forgejo posting is not available yet. You can read the change and run agent reviews.' : 'Read-only: add this forge with a token_command to the config file to comment and approve.'}</p>
        )}
        {error && <p class="alert" role="alert">⚠ {error}</p>}
      </header>
      <nav class="tabs" role="tablist">
        {(
          [
            ...(isAuthor ? [['threads', 'Threads']] : []),
            ['overview', `Overview${general.length + mine.length ? ` (${general.length + mine.length})` : ''}`],
            ['changes', `Changes (${files.length} files, ${open} open threads)`],
            ['review', `Review${drafts.length ? ` (${drafts.length} drafts)` : ''}`],
            ['summary', 'Summary'],
          ] as [Tab, string][]
        ).map(([t, label]) => (
          <button role="tab" aria-selected={tab === t} class={tab === t ? 'tab on' : 'tab'} onClick={() => setTab(t)} key={t}>
            {label}
          </button>
        ))}
      </nav>
      {tab === 'summary' && <SummaryPanel change={change} setup={setup} />}
      {tab === 'threads' && isAuthor && <TriagePanel change={change} setup={setup} onPick={pick} />}
      {tab === 'overview' && (
        <div class="overview">
          <Section title="Description">
            {change.description.trim() ? <Prose text={change.description} /> : <p class="muted">No description.</p>}
          </Section>
          <Section title="Discussion">
            {general.length === 0 ? <p class="muted">No general discussion.</p> : general.map((t) => <ThreadView thread={t} key={t.id} />)}
          </Section>
          <Section title="My pending comments">
            {mine.length === 0 && <p class="muted">None yet. Comment on a line from Changes, or here on the whole change.</p>}
            {mine.map((d) => (
              <MyDraft draft={d} key={d.id} where={d.anchor ? `${d.anchor.path}:${d.anchor.new_line || d.anchor.old_line}` : d.thread_id ? undefined : 'on the change'} />
            ))}
            {canDraft && (commenting ? (
              <Composer placeholder="A comment on the whole change" submit={(text) => writer.act('/api/changes/drafts', { body: text })} onDone={() => setCommenting(false)} />
            ) : (
              <button onClick={() => setCommenting(true)}>Comment on the change</button>
            ))}
          </Section>
        </div>
      )}
      {tab === 'changes' && (
        <>
        {moved && drafts.length > 0 && <section class="section">
          <h2>Drafts from the reviewed head</h2><HeadBadge state={headState} />
          <p class="muted">These drafts retain their original locations. They are shown separately from the current diff; anchored findings need a new review before you keep them.</p>
          {drafts.map((d) => <DraftCard key={d.id} draft={d} runID={runID} />)}
        </section>}
        <div class="changes">
          <FileList files={files} onPick={pick} badge={(f) =>
            (byFile.get(f.new_path)?.length ?? 0) + drafts.filter((d) => d.anchor?.path === f.new_path).length > 0 && (
              <> <CommentBubble severity={worstSeverity(drafts.filter((d) => d.status === 'pending' && !d.decision && d.anchor?.path === f.new_path))} /></>
            )} />
          <div class="files">
            {files.map((f) => (
              <FileView
                key={f.new_path}
                file={f}
                threads={byFile.get(f.new_path) ?? (f.old_path !== f.new_path ? byFile.get(f.old_path) ?? [] : [])}
                drafts={moved ? [] : drafts.filter((d) => d.status === 'pending' && !d.decision && d.anchor?.path === f.new_path)}
                mine={mine.filter((d) => d.anchor?.path === f.new_path && !d.thread_id)}
                runID={runID}
                collapsed={files.length > manyFiles}
                picked={picked?.path === f.new_path ? picked.n : undefined}
              />
            ))}
          </div>
        </div>
        </>
      )}
      <div hidden={tab !== 'review'}>
        <RunPanel change={change} setup={setup} onPick={pick} onDrafts={(ds, id, head) => { setDrafts(ds); setRunID(id); setReviewedHead(head) }} />
      </div>
    </div>
    </HeadContext.Provider>
    </WriteContext.Provider>
    </RefsContext.Provider>
  )
}
