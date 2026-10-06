import { useEffect, useState } from 'preact/hooks'
import { errorText } from './action'
import { api, bindings, post } from './api'
import { ChangeView } from './change'
import { LocalRun } from './run'
import { Inbox } from './inbox'
import { RunList } from './runs'
import { waive } from './confirm'
import { FirstRun, Forges, Tools } from './setup'
import { applyScheme, SchemePicker } from './scheme'
import type { Change, Run, Scheme, Setup } from './types'

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
  const [scheme, setScheme] = useState<Scheme>('')

  useEffect(() => applyScheme(scheme), [scheme])

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
    api<Setup>('/api/setup')
      .then((s) => {
        setSetup(s)
        setScheme(s.scheme)
      })
      .catch((e: unknown) => setError(String(e)))
    api<Run[]>('/api/runs').then(setRuns).catch(() => undefined)
  }, [])

  const open = async (target: string) => {
    setOpening(true)
    setError(undefined)
    try {
      setChange(await post<Change>('/api/changes', { url: target.trim() }))
      setURL(target.trim())
    } catch (e) {
      setError(errorText(e))
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
        <SchemePicker scheme={scheme} onScheme={setScheme} />
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

export function Home({ setup, onSetup, runs, onOpen, onOpenRun, opening }: { setup?: Setup; onSetup: (s: Setup) => void; runs: Run[]; onOpen: (url: string) => void; onOpenRun: (id: string) => void; opening: boolean }) {
  // A fresh install, with nowhere to read reviews from, starts at setup. It
  // stays until the person is done, so adding a second forge is one step on.
  const [firstRun, setFirstRun] = useState<boolean>()
  useEffect(() => {
    if (setup && firstRun === undefined) setFirstRun(setup.editable && !setup.config_error && setup.forges.length === 0)
  }, [setup])
  if (setup && firstRun) return <div class="home"><FirstRun setup={setup} onSetup={onSetup} onDone={() => setFirstRun(false)} /></div>
  return (
    <div class="home">
      {/* A saved forge changes the base, and the inbox reads its accounts again. */}
      <Inbox onOpen={onOpen} opening={opening} key={setup?.base} />
      {setup && (
        <details class="inbox-setup" onToggle={(e) => {
          // Opening the panel reads setup again, which picks up a hand edit
          // or a repaired file before anything is edited from it.
          if ((e.currentTarget as HTMLDetailsElement).open) api<Setup>('/api/setup').then(onSetup).catch(() => undefined)
        }}><summary>Accounts and review tools</summary>
        <ul class="setup">
          <li>
            Forges: <Forges setup={setup} onSetup={onSetup} />
          </li>
          <li>
            Agent tools: <Tools setup={setup} onSetup={onSetup} />
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
