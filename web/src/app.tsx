import { useEffect, useState } from 'preact/hooks'
import { api, bindings, post } from './api'
import { ChangeView } from './change'
import { LocalRun } from './run'
import { Inbox } from './inbox'
import { waive } from './confirm'
import { changeNumber, type Change, type Run, type Setup } from './types'
import { ago, short, StatusBadge, usd } from './ui'

interface Hello {
  version: string
}

// App is vuoro's window: a change opened by URL, and the runs so far.
export function App() {
  const [hello, setHello] = useState<Hello>()
  const [setup, setSetup] = useState<Setup>()
  const [error, setError] = useState<string>()
  const [url, setURL] = useState('')
  const [change, setChange] = useState<Change>()
  const [opening, setOpening] = useState(false)
  const [runs, setRuns] = useState<Run[]>([])
  const [localRun, setLocalRun] = useState<string>()

  useEffect(() => {
    // -check loads this page and waits for the report, so it comes first.
    const b = bindings()
    api<Hello>('/api/hello')
      .then(async (h) => {
        setHello(h)
        const detail = b ? `server=${JSON.stringify(h.version)} binding=${JSON.stringify(await b.version())}` : ''
        await b?.report({ ok: true, detail })
      })
      .catch(async (e: unknown) => {
        setError(String(e))
        await b?.report({ ok: false, detail: String(e) })
      })
    api<Setup>('/api/setup').then(setSetup).catch((e: unknown) => setError(String(e)))
    api<Run[]>('/api/runs').then(setRuns).catch(() => undefined)
  }, [])

  const open = async (target: string) => {
    setOpening(true)
    setError(undefined)
    try {
      setChange(await post<Change>('/api/changes', { url: target.trim() }))
      setURL(target.trim())
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e))
    } finally {
      setOpening(false)
    }
  }

  return (
    <div class="app">
      <header class="bar">
        <button class="brand link" onClick={() => { setChange(undefined); setLocalRun(undefined); api<Run[]>('/api/runs').then(setRuns).catch(() => undefined) }}>vuoro</button>
        <form class="open" onSubmit={(e) => { e.preventDefault(); void open(url) }}>
          <input
            type="url"
            placeholder="Open a change by URL…"
            value={url}
            onInput={(e) => setURL((e.target as HTMLInputElement).value)}
            aria-label="Merge request or pull request URL"
          />
          <button class="primary" disabled={opening || !url.trim()}>{opening ? 'Opening…' : 'Open'}</button>
          {change && <button type="button" disabled={opening} onClick={() => void open(url)}>Refresh</button>}
        </form>
        <span class="muted">{hello && `vuoro ${hello.version}`}</span>
      </header>
      {setup?.config_error && <p class="alert" role="alert">⚠ The config file did not load: {setup.config_error}</p>}
      {error && <p class="alert" role="alert">⚠ {error}</p>}
      <main>
        {change ? (
          <ChangeView change={change} setup={setup} onChange={setChange} onSetup={setSetup} key={`${change.web_url}@${change.diff.head}`} />
        ) : localRun ? (
          <LocalRun id={localRun} key={localRun} />
        ) : (
          <Home setup={setup} onSetup={setSetup} runs={runs} opening={opening} onOpen={(u) => { setURL(u); void open(u) }} onOpenRun={setLocalRun} />
        )}
      </main>
    </div>
  )
}

function Home({ setup, onSetup, runs, onOpen, onOpenRun, opening }: { setup?: Setup; onSetup: (s: Setup) => void; runs: Run[]; onOpen: (url: string) => void; onOpenRun: (id: string) => void; opening: boolean }) {
  return (
    <div class="home">
      <Inbox onOpen={onOpen} opening={opening} />
      {setup && (
        <details class="inbox-setup"><summary>Accounts and review tools</summary>
        <ul class="setup">
          <li>
            Forges: {setup.forges.length === 0 ? 'none configured, so changes open anonymously and only public projects work' : setup.forges.map((f) => `${f.name} as @${f.username}`).join(', ')}
          </li>
          <li>
            Agent tools: {setup.tools.length === 0 ? 'none' : setup.tools.map((t) => (t.error ? `⚠ ${t.tool}: ${t.error}` : `✓ ${t.tool} ${t.version}`)).join(', ')}
          </li>
          <li>
            Confirmations:{' '}
            {setup.waived.length === 0 ? (
              'vuoro asks before submitting a review or approving'
            ) : (
              <>
                you turned off asking before {setup.waived.map((a) => (a === 'submit' ? 'submitting' : 'approving')).join(' and ')}.{' '}
                <button class="link" onClick={() => void waive(setup.waived, true).then(onSetup).catch(() => undefined)}>Ask again</button>
              </>
            )}
          </li>
        </ul>
        </details>
      )}
      <RunList runs={runs} onOpen={onOpen} onOpenRun={onOpenRun} />
    </div>
  )
}

// RunList is the runs so far. A run of local commits, from vuoro review, has
// no forge page to open, so it is listed without a link.
export function RunList({ runs, onOpen, onOpenRun }: { runs: Run[]; onOpen: (url: string) => void; onOpenRun?: (id: string) => void }) {
  if (runs.length === 0) return null
  return (
    <>
      <h2>Runs</h2>
      <ul class="runs">
        {runs.map((r) => (
          <li key={r.id}>
            {r.kind === 'local' ? (onOpenRun ? <button class="link" onClick={() => onOpenRun(r.id)}>{r.title}</button> : <span>{r.title}</span>) : <button class="link" onClick={() => onOpen(r.web_url)}>{r.title}</button>}{' '}
            <span class="muted">
              {r.kind === 'local' ? `local commits of ${r.ref.project}` : `${r.ref.project}${changeNumber(r.kind, r.ref.number)}`} at {short(r.head)}, {ago(r.created)}, {r.drafts.filter((d) => d.status === 'pending').length} drafts, {usd(r.cost_usd)}
            </span>{' '}
            <StatusBadge status={r.status} />
          </li>
        ))}
      </ul>
    </>
  )
}
