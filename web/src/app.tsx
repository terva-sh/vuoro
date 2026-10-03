import { useEffect, useState } from 'preact/hooks'
import { api, bindings, post } from './api'
import { ChangeView } from './change'
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
        <button class="brand link" onClick={() => { setChange(undefined); api<Run[]>('/api/runs').then(setRuns).catch(() => undefined) }}>vuoro</button>
        <form class="open" onSubmit={(e) => { e.preventDefault(); void open(url) }}>
          <input
            type="url"
            placeholder="https://gitlab.com/group/project/-/merge_requests/1"
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
        ) : (
          <Home setup={setup} onSetup={setSetup} runs={runs} onOpen={(u) => { setURL(u); void open(u) }} />
        )}
      </main>
    </div>
  )
}

function Home({ setup, onSetup, runs, onOpen }: { setup?: Setup; onSetup: (s: Setup) => void; runs: Run[]; onOpen: (url: string) => void }) {
  return (
    <div class="home">
      <p>Open a merge request or pull request by its URL to read it and review it with an agent.</p>
      {setup && (
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
      )}
      {runs.length > 0 && (
        <>
          <h2>Runs</h2>
          <ul class="runs">
            {runs.map((r) => (
              <li key={r.id}>
                <button class="link" onClick={() => onOpen(r.web_url)}>{r.title}</button>{' '}
                <span class="muted">{r.ref.project}{changeNumber(r.kind, r.ref.number)} at {short(r.head)}, {ago(r.created)}, {r.drafts.filter((d) => d.status === 'pending').length} drafts, {usd(r.cost_usd)}</span>{' '}
                <StatusBadge status={r.status} />
              </li>
            ))}
          </ul>
        </>
      )}
    </div>
  )
}
