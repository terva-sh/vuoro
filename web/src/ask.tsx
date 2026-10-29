// The change's Ask tab (section 6.11, D68): canned prompts and the person's
// own questions, each answered by a read-only agent they can keep talking to.
// Every conversation is kept, and nothing in it reaches the forge.
import { useEffect, useRef, useState } from 'preact/hooks'
import { errorText } from './action'
import { api, post } from './api'
import { HeadBadge, useRunHead } from './head'
import { usePoll } from './poll'
import { SessionPane } from './session'
import { TextEditor } from './textedit'
import { refKey, type Change, type Prompt, type Prompts, type Run, type Setup } from './types'
import { ago, Prose, short, StatusBadge, usd } from './ui'

type Props = { change: Change; setup?: Setup }

export function AskPanel(props: Props) {
  return <AskWorkspace {...props} key={refKey(props.change.ref)} />
}

// A prompt's scope wins over the tab's (section 6.11), so its button says so.
const promptTitle = (p: Prompt) => `${p.builtin ? '' : `Your prompt, ${p.name}.md. `}${p.scope === 'change' ? 'Always the whole change. ' : p.scope === 'since_review' ? 'Only what changed since the chosen review. ' : ''}${p.text}`

// what a conversation was asked: its prompt's title, or the question.
const asked = (r: Run) => r.ask?.title || (r.mode === 'summary' ? 'Explain this change' : 'A question')

function AskWorkspace({ change, setup }: Props) {
  const [list, setList] = useState<Run[]>([])
  const [reviews, setReviews] = useState<Run[]>([])
  const [prompts, setPrompts] = useState<Prompts>()
  const [selected, setSelected] = useState<string>()
  const [run, setRun] = useState<Run>()
  const [question, setQuestion] = useState('')
  const [scope, setScope] = useState('change')
  const [baseline, setBaseline] = useState<string>()
  const [tool, setTool] = useState<string>()
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string>()
  const mounted = useRef(true)
  const tools = setup?.tools.filter((t) => !t.error) ?? []
  const key = refKey(change.ref)

  useEffect(() => {
    mounted.current = true
    const controller = new AbortController()
    const signal = controller.signal
    api<Prompts>('/api/prompts', { signal }).then(setPrompts).catch((e: unknown) => { if (!signal.aborted) setError(errorText(e)) })
    api<Run[]>('/api/changes/asks', { method: 'POST', body: JSON.stringify({ url: change.web_url }), signal }).then((all) => {
      if (signal.aborted) return
      setList(all)
      setSelected((id) => id ?? all[0]?.id)
    }).catch((e: unknown) => { if (!signal.aborted) setError(errorText(e)) })
    api<Run[]>('/api/runs', { signal }).then((all) => {
      if (signal.aborted) return
      const done = all.filter((r) => refKey(r.ref) === key && r.kind === change.kind && !r.mode && r.review_complete)
      setReviews(done)
      setBaseline((id) => id ?? done[0]?.id)
    }).catch(() => {})
    return () => { mounted.current = false; controller.abort() }
  }, [key, change.diff.head])

  usePoll(async (signal) => {
    if (!selected) return undefined
    try {
      const next = await api<Run>(`/api/runs/${selected}`, { signal })
      if (!signal.aborted) {
        setRun(next)
        setList((all) => all.map((r) => r.id === next.id ? { ...next, ask: next.ask ?? r.ask } : r))
      }
    } catch (e) { if (!signal.aborted) setError(errorText(e)) }
    return 1500
  }, [selected])

  // starting guards itself as well as its buttons: Cmd+Enter submits the
  // question's form whatever its button says.
  const starting = useRef(false)
  const start = async (body: { prompt?: string; question?: string; scope?: string; review?: string }) => {
    if (starting.current) return
    const chosen = body.review ?? baseline
    const since = (body.scope ?? scope) === 'since_review'
    if (since && !chosen) { setError('Choose a completed Vuoro review to ask about what changed since it.'); return }
    starting.current = true; setBusy(true); setError(undefined)
    try {
      const next = await post<Run>('/api/asks', {
        url: change.web_url, prompt: body.prompt, question: body.question,
        tool: tool ?? tools[0]?.name, review_id: since ? chosen : undefined,
      })
      if (!mounted.current) return
      setList((all) => [next, ...all.filter((r) => r.id !== next.id)])
      setRun(next); setSelected(next.id)
      if (body.question) setQuestion('')
    } catch (e) { if (mounted.current) setError(errorText(e)) }
    finally { starting.current = false; if (mounted.current) setBusy(false) }
  }

  const review = reviews.find((r) => r.id === baseline)
  const noNewFor = (r: Run) => r.head === change.diff.head
  const noNew = scope === 'since_review' && !!review && noNewFor(review)
  const cannot = busy || tools.length === 0 || noNew || (scope === 'since_review' && !review)
  const current = run?.id === selected ? run : undefined

  return <div class="ask-panel">
    <p class="muted">Ask your review tool about this change. It reads the change and nothing else, and nothing here is posted to the forge.</p>
    <div class="ask-prompts" role="group" aria-label="Prompts">
      {prompts?.prompts.map((p) => (
        <button key={p.name} class={p.builtin ? undefined : 'own'} title={promptTitle(p)}
          disabled={busy || tools.length === 0 || (p.scope ?? scope) === 'since_review' && (!review || noNewFor(review))}
          onClick={() => void start({ prompt: p.name, scope: p.scope ?? scope })}>
          {p.title}
        </button>
      ))}
    </div>
    {prompts && prompts.errors.length > 0 && <details class="ask-prompt-errors"><summary>⚠ {prompts.errors.length} prompt file{prompts.errors.length === 1 ? '' : 's'} could not be used</summary>
      <ul>{prompts.errors.map((e) => <li key={e.file}><span class="mono">{e.file}</span>: {e.reason}</li>)}</ul>
      <p class="muted">Your prompts are Markdown files in <span class="mono">{prompts.dir}</span>.</p>
    </details>}
    <form class="ask-question" onSubmit={(e) => { e.preventDefault(); if (!cannot && question.trim()) void start({ question }) }}>
      <TextEditor rows={2} value={question} onInput={setQuestion} label="Your question" placeholder="Ask anything about this change" submitOn="mod-enter" />
      <button class="primary" disabled={cannot || !question.trim()}>{busy ? 'Asking…' : 'Ask'}</button>
    </form>
    <div class="toolbar">
      <label>Scope <select aria-label="Ask scope" value={scope} onChange={(e) => setScope(e.currentTarget.value)}>
        <option value="change">Whole change</option><option value="since_review" disabled={reviews.length === 0}>Since a Vuoro review</option>
      </select></label>
      {scope === 'since_review' && <label>Review <select aria-label="Review baseline" value={baseline} onChange={(e) => setBaseline(e.currentTarget.value)}>
        {reviews.map((r) => <option key={r.id} value={r.id}>{ago(r.created)} at {short(r.head)}</option>)}
      </select></label>}
      {tools.length > 0 && <label>Tool <select aria-label="Ask tool" value={tool} onChange={(e) => setTool(e.currentTarget.value)}>
        {tools.map((t) => <option key={t.name} value={t.name}>{t.tool}</option>)}
      </select></label>}
    </div>
    {noNew && <p class="muted">No new commits since this review.</p>}
    {tools.length === 0 && <p class="muted">No review tool is available. Earlier conversations are still readable.</p>}
    {error && <p class="alert" role="alert">⚠ {error}</p>}
    <div class="ask-layout">
      {list.length > 0 && <ul class="ask-list" aria-label="Conversations">
        {list.map((r) => <li key={r.id}>
          <button class={r.id === selected ? 'ask-item on' : 'ask-item'} aria-current={r.id === selected} onClick={() => { setRun(undefined); setSelected(r.id) }}>
            <span class="ask-item-title">{asked(r)}</span>
            <span class="muted">{r.ask?.prompt ? r.ask.prompt : 'question'} · {ago(r.created)} · {short(r.head)}{r.head !== change.diff.head ? ' · stale' : ''} · {usd(r.cost_usd)}</span>
          </button>
        </li>)}
      </ul>}
      {current && <Conversation run={current} change={change} busy={busy} onAgain={(r) => void start(again(r, list))} />}
    </div>
  </div>
}

// again is the same question on the change's new head: the same prompt, or
// the same words, over the same scope and review. A summary from before Ask
// is asked again as explain; only the listing names its prompt.
function again(r: Run, list: Run[]) {
  const ask = r.ask ?? list.find((x) => x.id === r.id)?.ask
  const prompt = ask?.prompt || (r.mode === 'summary' ? 'explain' : undefined)
  const scope = r.summary?.scope ?? 'change'
  return { ...(prompt ? { prompt } : { question: ask?.question ?? '' }), scope, review: r.summary?.review_id }
}

function Conversation({ run, change, busy, onAgain }: { run: Run; change: Change; busy: boolean; onAgain: (r: Run) => void }) {
  const head = useRunHead(run.id, run.head, change)
  const working = run.status === 'working' || run.status === 'preparing'
  const stale = run.head !== change.diff.head
  const summary = run.mode === 'summary'
  return <section class="section ask-conversation" aria-label={asked(run)}>
    <div class="run-head"><strong>{asked(run)}</strong><StatusBadge status={run.status} /><HeadBadge state={head} label="asked" /><span class="muted">{usd(run.cost_usd)}</span></div>
    {run.summary && <p class="muted">Range <span class="mono" title={`${run.summary.from}..${run.summary.to}`}>{short(run.summary.from)}..{short(run.summary.to)}</span>{run.summary.scope === 'since_review' ? ' · since the selected Vuoro review' : ' · whole change'}</p>}
    {stale && <p class="ask-stale">Asked at <span class="mono">{short(run.head)}</span>; the change is now at <span class="mono">{short(change.diff.head)}</span>. You can carry on here, or{' '}
      <button disabled={busy || working} onClick={() => onAgain(run)}>ask the same on the new head</button></p>}
    {run.error && <p class="alert" role="alert">⚠ {run.error}</p>}
    {summary ? <SummaryText run={run} working={working} /> : run.sessions.map((s) => <SessionPane run={run} session={s} key={s.id} />)}
  </section>
}

// SummaryText is a summary from before Ask, read only: a summary takes no
// follow-up.
function SummaryText({ run, working }: { run: Run; working: boolean }) {
  const text = run.sessions.flatMap((s) => (s.log ?? []).filter((e) => e.kind === 'assistant').map((e) => e.text ?? '')).join('\n\n')
  return <div aria-live="polite" aria-busy={working} class="summary-output">
    {text ? <Prose text={text} /> : <p class="muted">{working ? 'Waiting for summary text…' : 'No summary text was produced.'}</p>}
    <p class="muted">A summary from before the Ask tab. It takes no follow-up; ask a new question instead.</p>
  </div>
}
