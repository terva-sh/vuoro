import { useEffect, useState } from 'preact/hooks'
import { errorText } from './action'
import { api, bindings, post } from './api'
import { ChangeView } from './change'
import { LocalRun } from './run'
import { Inbox } from './inbox'
import { LocalTab } from './local'
import { ProjectsTab } from './projects'
import { RunList } from './runs'
import { About } from './about'
import { Settings } from './settings'
import { FirstRun } from './setup'
import { applyScheme } from './scheme'
import type { Change, HomeTab, LocalView, Run, Scheme, Setup } from './types'

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
  const [about, setAbout] = useState(false)
  const [settings, setSettings] = useState(false)

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
      setSettings(false)
    } catch (e) {
      setError(errorText(e))
    } finally {
      setOpening(false)
    }
  }

  return (
    <div class="app">
      <header class="bar">
        <button class="brand link" onClick={() => { setChange(undefined); setLocalRun(undefined); setSettings(false); api<Run[]>('/api/runs').then(setRuns).catch(() => undefined) }}>vuoro</button>
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
        {hello && <button type="button" class="link version" title="About vuoro" onClick={() => setAbout(true)}>vuoro {hello.version}</button>}
        <button type="button" class={settings ? 'settings-button on' : 'settings-button'} aria-pressed={settings} onClick={() => setSettings((v) => !v)}>⚙ Settings</button>
      </header>
      {about && <About setup={setup} onClose={() => setAbout(false)} />}
      {setup?.config_error && <p class="alert" role="alert">⚠ The config file did not load: {setup.config_error}</p>}
      {error && <p class="alert" role="alert">⚠ {error}</p>}
      <main>
        {settings && setup ? (
          <Settings setup={setup} onSetup={setSetup} scheme={scheme} onScheme={setScheme} version={hello?.version}
            onAbout={() => setAbout(true)} onClose={() => setSettings(false)} />
        ) : change ? (
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
  // The tab is the one used last, once setup says which, and Reviews before.
  const [chosen, setChosen] = useState<HomeTab>()
  const tab = chosen || setup?.tab || 'reviews'
  const [counts, setCounts] = useState<{ reviews?: number; local?: number }>({})
  // Local reads every checkout, so it waits to be shown, and its count
  // comes from the view it last built until then.
  const [localShown, setLocalShown] = useState(false)
  useEffect(() => { if (tab === 'local') setLocalShown(true) }, [tab])
  // Projects reads its projects' changes, so it too waits to be shown.
  const [projectsShown, setProjectsShown] = useState(false)
  useEffect(() => { if (tab === 'projects') setProjectsShown(true) }, [tab])
  useEffect(() => {
    api<LocalView | null>('/api/local?cached=1')
      .then((v) => { if (v) setCounts((c) => ({ ...c, local: c.local ?? v.needs })) })
      .catch(() => undefined)
  }, [])
  const choose = (t: HomeTab) => {
    if (t === tab) return
    setChosen(t)
    post('/api/prefs/tab', { tab: t }).catch(() => undefined)
  }
  if (setup && firstRun) return <div class="home"><FirstRun setup={setup} onSetup={onSetup} onDone={() => setFirstRun(false)} /></div>
  const tabs: [HomeTab, string][] = [['reviews', 'Reviews'], ['local', 'Local'], ['projects', 'Projects']]
  return (
    <div class="home">
      <nav class="tabs home-tabs" role="tablist" aria-label="Home" onKeyDown={(e) => {
        if (e.key !== 'ArrowLeft' && e.key !== 'ArrowRight') return
        const step = e.key === 'ArrowRight' ? 1 : tabs.length - 1
        const next = tabs[(tabs.findIndex(([t]) => t === tab) + step) % tabs.length][0]
        choose(next)
        ;(e.currentTarget as HTMLElement).querySelector<HTMLButtonElement>(`#tab-${next}`)?.focus()
      }}>
        {tabs.map(([t, label]) => {
          const n = counts[t as 'reviews' | 'local']
          return (
            <button role="tab" id={`tab-${t}`} aria-controls={`panel-${t}`} aria-selected={tab === t} tabIndex={tab === t ? 0 : -1}
              class={tab === t ? 'tab on' : 'tab'} onClick={() => choose(t)} key={t}>
              {label}{n !== undefined && <> <span class="badge" aria-label={`${n} need you`}>{n}</span></>}
            </button>
          )
        })}
      </nav>
      <div class="home-panel" role="tabpanel" id="panel-reviews" aria-labelledby="tab-reviews" hidden={tab !== 'reviews'}>
        {/* A saved forge changes the base, and the inbox reads its accounts again. */}
        <Inbox onOpen={onOpen} opening={opening} key={setup?.base} active={tab === 'reviews'}
          onCount={(n) => setCounts((c) => ({ ...c, reviews: n }))} />
      </div>
      <div class="home-panel" role="tabpanel" id="panel-local" aria-labelledby="tab-local" hidden={tab !== 'local'}>
        {localShown && <LocalTab setup={setup} onOpen={onOpen} onOpenRun={onOpenRun} onCount={(n) => setCounts((c) => ({ ...c, local: n }))} />}
      </div>
      <div class="home-panel" role="tabpanel" id="panel-projects" aria-labelledby="tab-projects" hidden={tab !== 'projects'}>
        {/* A saved forge changes the base, and the projects read their accounts again. */}
        {projectsShown && <ProjectsTab onOpen={onOpen} opening={opening} active={tab === 'projects'} key={setup?.base} />}
      </div>
      <RunList runs={runs} onOpen={onOpen} onOpenRun={onOpenRun} />
    </div>
  )
}
