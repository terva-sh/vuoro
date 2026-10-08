// Setup in the window (D45). vuoro never asks for a token: the person keeps
// it where they choose, and the config names the command or the variable
// that supplies it (D29). The form tests that source against the forge
// before it saves.
import { useEffect, useRef, useState } from 'preact/hooks'
import { errorText, useAction } from './action'
import { post } from './api'
import { EditForm } from './textedit'
import { Gauge } from './gauge'
import type { Access, Background as BackgroundView, ForgeForm, Setup, TokenSource, Tool } from './types'

// splitCommand splits a command line into its words as a shell would,
// honouring single quotes, double quotes, and backslashes, and expanding
// nothing. vuoro runs the result without a shell.
export function splitCommand(line: string): string[] {
  const words: string[] = []
  let word = ''
  let started = false
  let quote: '"' | "'" | undefined
  for (let i = 0; i < line.length; i++) {
    const c = line[i]
    if (quote === "'") {
      if (c === "'") quote = undefined
      else word += c
    } else if (quote === '"') {
      if (c === '"') quote = undefined
      else if (c === '\\' && i + 1 < line.length && '"\\$`'.includes(line[i + 1])) word += line[++i]
      else word += c
    } else if (c === "'" || c === '"') {
      quote = c
      started = true
    } else if (c === '\\' && i + 1 < line.length) {
      word += line[++i]
      started = true
    } else if (/\s/.test(c)) {
      if (started) words.push(word)
      word = ''
      started = false
    } else {
      word += c
      started = true
    }
  }
  if (quote) throw new Error(`a ${quote === '"' ? 'double' : 'single'} quote is not closed`)
  if (started) words.push(word)
  return words
}

// joinCommand writes argv as a command line splitCommand reads back.
export function joinCommand(argv: string[]): string {
  return argv.map((a) => (a !== '' && /^[A-Za-z0-9_@%+=:,./~-]+$/.test(a) ? a : `'${a.replace(/'/g, `'\\''`)}'`)).join(' ')
}

const fill = (s: string, host: string) => s.replaceAll('{host}', host || 'HOST')

// hostOf is an https address's host, when it is a plain host name with an
// optional port. Anything else is refused, so a host can only ever be a word
// of the commands setup shows and runs.
export function hostOf(url: string): string {
  try {
    const u = new URL(url.trim())
    const bare = !u.username && !u.password && (u.pathname === '/' || u.pathname === '') && !u.search && !u.hash
    return u.protocol === 'https:' && bare && /^[a-z0-9]([a-z0-9.-]*[a-z0-9])?(:[0-9]{1,5})?$/.test(u.host) ? u.host : ''
  } catch {
    return ''
  }
}

const envName = (name: string) => `VUORO_${name.toUpperCase().replace(/[^A-Z0-9]+/g, '_').replace(/^_|_$/g, '')}_TOKEN`

// tokenPage is where the forge makes a personal access token, and the access
// vuoro needs from it: reading reviews, and posting the ones the person sends.
function tokenPage(kind: string, url: string): { href: string; scopes: string } | undefined {
  const base = url.trim().replace(/\/+$/, '')
  if (!hostOf(base)) return undefined
  if (kind === 'gitlab') return { href: `${base}/-/user_settings/personal_access_tokens?name=vuoro&scopes=api`, scopes: 'the api scope' }
  if (kind === 'github') return { href: `${base}/settings/tokens/new?description=vuoro&scopes=repo`, scopes: 'the repo scope' }
  return { href: `${base}/user/settings/applications`, scopes: 'read and write on repository and issue, and read on user' }
}

// useForgeForm is one forge being added or changed: where it is, where its
// token is kept, and the test that proves the two together. The guided first
// run and the accounts panel's editor both show it.
function useForgeForm(setup: Setup, initial?: ForgeForm, original?: string) {
  // The form's fields come from the file as it was when the form opened, so
  // its save carries that file's hash: a file changed since, even one the
  // window has read again meanwhile, is refused rather than overwritten.
  const [base] = useState(setup.base)
  const [kind, setKind] = useState(initial?.kind ?? 'forgejo')
  const [url, setURL] = useState(initial?.url ?? '')
  const [name, setName] = useState(initial?.name ?? '')
  const [username, setUsername] = useState(initial?.username ?? '')
  const [clone, setClone] = useState(initial?.clone || 'ssh')
  const [sourceID, setSourceID] = useState(initial ? (initial.token_env ? 'env' : 'command') : (setup.sources[0]?.id ?? 'env'))
  const [command, setCommand] = useState(initial?.token_command ? joinCommand(initial.token_command) : '')
  const [env, setEnv] = useState(initial?.token_env ?? '')
  const [result, setResult] = useState<{ username: string; key: string; access?: Access }>()
  // Testing and saving are apart, so each button says which one runs.
  const testing = useAction()
  const saving = useAction()
  const busy = testing.busy || saving.busy

  const host = hostOf(url)
  const source: TokenSource | undefined = setup.sources.find((s) => s.id === sourceID)
  const isEnv = sourceID === 'env'
  // A ready-made source fills the command in for this host; the person can
  // still change it, as for a 1Password reference.
  useEffect(() => {
    if (source?.read) setCommand(joinCommand(source.read.map((w) => fill(w, host))))
  }, [sourceID, host])

  let argv: string[] = []
  let commandError: string | undefined
  if (!isEnv) {
    try {
      argv = splitCommand(command)
    } catch (e) {
      commandError = errorText(e)
    }
  }
  const effectiveName = name.trim() || host
  const effectiveEnv = env.trim() || envName(effectiveName || 'forge')
  const form: ForgeForm = {
    name: effectiveName, kind, url: url.trim(), username: username.trim(), clone,
    ...(isEnv ? { token_env: effectiveEnv } : { token_command: argv }),
  }
  // The test proves this source on this forge; changing either asks again.
  const key = JSON.stringify([kind, form.url, form.token_command, form.token_env])
  const tested = result && result.key === key ? result : undefined
  const mismatch = !!tested && !!form.username && tested.username.toLowerCase() !== form.username.toLowerCase()
  // A token that cannot read reviews is not saved; one that reaches further
  // than vuoro needs is, with the gauge's warning (D57).
  const unreadable = tested?.access?.level === 'none'
  const canSave = !!tested && !!form.username && !mismatch && !!form.name && !unreadable
  const ready = !!host && (isEnv || (argv.length > 0 && !commandError))

  const test = async () => {
    saving.setError(undefined)
    const r = await testing.run(() => post<{ username: string; matches: boolean; access?: Access }>('/api/setup/test', form))
    if (!r) {
      setResult(undefined)
      return
    }
    setResult({ username: r.username, key, access: r.access })
    if (!form.username) setUsername(r.username)
  }
  const save = (onSaved: (s: Setup) => void) => {
    testing.setError(undefined)
    void saving.run(async () => onSaved(await post<Setup>('/api/setup/forges', { base, original: original ?? '', forge: form })))
  }
  return {
    kind, setKind, url, setURL, name, setName, username, setUsername, clone, setClone,
    sourceID, setSourceID: (id: string) => { setSourceID(id); setResult(undefined) },
    command, setCommand, env, setEnv, host, source, isEnv, argv, commandError, effectiveEnv,
    form, tested, mismatch, canSave, ready, test, save, testing, saving, busy,
    error: testing.error ?? saving.error,
  }
}
type Forge = ReturnType<typeof useForgeForm>

// Copy is a command to run in a terminal, with a button that copies it. The
// clipboard can refuse, and the command can still be selected by hand.
function Copy({ text }: { text: string }) {
  const [copied, setCopied] = useState(false)
  useEffect(() => setCopied(false), [text])
  return (
    <div class="copy">
      <code>{text}</code>
      <button type="button" onClick={() => void navigator.clipboard?.writeText(text).then(() => setCopied(true), () => undefined)}>{copied ? 'Copied' : 'Copy'}</button>
    </div>
  )
}

// TokenSteps walks the person through making a token, keeping it, and
// checking that vuoro can read it and the forge accepts it.
function TokenSteps({ f, setup }: { f: Forge; setup: Setup }) {
  const page = tokenPage(f.kind, f.url)
  const where = f.host || 'the forge'
  return (
    <ol class="token-steps">
      <li>
        <h4>Make a token</h4>
        {page ? (
          <p><a class="button" href={page.href} target="_blank" rel="noreferrer">Make a token on {f.host} ↗</a> It needs {page.scopes}.</p>
        ) : <p class="muted">Enter the forge's address first, and vuoro links to the page that makes one.</p>}
      </li>
      <li>
        <h4>Keep it somewhere safe</h4>
        <p class="muted">vuoro never asks for the token. You keep it in a store you trust, and vuoro asks that store for it when it needs it.</p>
        <fieldset class="sources">
          <legend>Keep it in</legend>
          {setup.sources.map((s) => (
            <label key={s.id} class={f.sourceID === s.id ? 'chosen' : ''}>
              <input type="radio" name="source" value={s.id} checked={f.sourceID === s.id} onChange={() => f.setSourceID(s.id)} /> {s.label}
            </label>
          ))}
        </fieldset>
        {f.source?.store && <>
          <p>Run this in a terminal, and paste the token when it asks:</p>
          <Copy text={joinCommand(f.source.store.map((w) => fill(w, f.host)))} />
        </>}
        {f.source?.note && <p class="muted">{fill(f.source.note, f.host)}</p>}
        {f.isEnv ? (
          <label class="field">Variable
            <input class="mono" placeholder={f.effectiveEnv} value={f.env} onInput={(e) => f.setEnv((e.target as HTMLInputElement).value)} />
          </label>
        ) : f.source?.read ? (
          <details class="advanced">
            <summary>Change how vuoro reads it</summary>
            <ReadCommand f={f} />
          </details>
        ) : <ReadCommand f={f} />}
        {f.commandError && <p class="warn">The command: {f.commandError}.</p>}
      </li>
      <li>
        <h4>Check that it works</h4>
        <p>
          <button type="button" class={f.ready && !f.tested ? 'primary' : ''} disabled={f.busy || !f.ready} onClick={() => void f.test()}>{f.testing.busy ? 'Checking…' : 'Check the token'}</button>{' '}
          {f.tested && !f.mismatch && <span class="ok">✓ Signed in to {where} as @{f.tested.username}</span>}
        </p>
        {f.mismatch && <p class="warn">The token is @{f.tested!.username}'s, not @{f.form.username}'s.</p>}
        {f.tested?.access && !f.mismatch && <Gauge access={f.tested.access} />}
      </li>
    </ol>
  )
}

function ReadCommand({ f }: { f: Forge }) {
  return (
    <label class="field">vuoro reads it with
      <input class="mono" value={f.command} placeholder="a program and its arguments" onInput={(e) => f.setCommand((e.target as HTMLInputElement).value)} />
      <span class="muted">vuoro runs it without a shell, and reads the token from what it prints.</span>
    </label>
  )
}

// MoreOptions are what a forge needs that the test fills in or a default
// covers: whose account it is, its name in vuoro, and how code is fetched.
function MoreOptions({ f, open }: { f: Forge; open?: boolean }) {
  return (
    <details class="advanced" open={open}>
      <summary>More options</summary>
      <div class="row">
        <label class="field">Your username
          <input value={f.username} placeholder="from the token check" onInput={(e) => f.setUsername((e.target as HTMLInputElement).value)} />
        </label>
        <label class="field">Name in vuoro
          <input value={f.name} placeholder={f.host || 'the forge’s host'} onInput={(e) => f.setName((e.target as HTMLInputElement).value)} />
        </label>
        <label class="field">Fetch code over
          <select value={f.clone} onChange={(e) => f.setClone((e.target as HTMLSelectElement).value)}>
            <option value="ssh">SSH, with your keys</option>
            <option value="https">HTTPS, with your credential helper</option>
          </select>
        </label>
      </div>
    </details>
  )
}

const kinds = [
  { id: 'forgejo', label: 'Forgejo or Gitea' },
  { id: 'gitlab', label: 'GitLab' },
  { id: 'github', label: 'GitHub' },
]

// ForgeEditor adds a forge, or changes the one named original, in the
// accounts panel.
export function ForgeEditor({ setup, initial, original, onSaved, onCancel }: {
  setup: Setup; initial?: ForgeForm; original?: string; onSaved: (s: Setup) => void; onCancel?: () => void
}) {
  const f = useForgeForm(setup, initial, original)
  const label = original ? 'Save' : 'Add this forge'
  return (
    <EditForm class="forge-editor" onSave={() => f.save(onSaved)} onCancel={onCancel} busy={f.busy} canSave={f.canSave}
      save={label} saving={f.saving.busy ? 'Saving…' : label} hint={f.canSave ? undefined : f.tested ? 'This token cannot be saved' : 'Check the token first'}
      error={f.error}>
      <div class="row">
        <label class="field">Forge
          <select value={f.kind} onChange={(e) => f.setKind((e.target as HTMLSelectElement).value)}>
            {kinds.map((k) => <option value={k.id} key={k.id}>{k.label}</option>)}
          </select>
        </label>
        <label class="field grow">Address
          <input type="url" placeholder={{ gitlab: 'https://gitlab.com', github: 'https://github.com' }[f.kind] ?? 'https://codeberg.org'} value={f.url} onInput={(e) => f.setURL((e.target as HTMLInputElement).value)} />
        </label>
      </div>
      {f.url.trim() && !f.host && <p class="warn">Use the forge's https address, such as https://gitlab.com, with nothing after the host.</p>}
      <TokenSteps f={f} setup={setup} />
      <MoreOptions f={f} open={!!original} />
    </EditForm>
  )
}

// signedIn words how a tool is signed in. Claude Code reports its login
// method and plan as words of its own, such as oauth_token max.
export function signedIn(account?: string): string {
  const words = (account ?? '').split(/\s+/).filter(Boolean)
  const how: Record<string, string> = { 'claude.ai': 'with a Claude account', oauth_token: 'with a Claude token', api_key: 'with an API key', apikey: 'with an API key' }
  const method = words.map((w) => how[w.toLowerCase()]).find(Boolean)
  const plan = words.find((w) => !how[w.toLowerCase()])
  return ['signed in', method, plan && `(${plan[0].toUpperCase()}${plan.slice(1)} plan)`].filter(Boolean).join(' ')
}

// toolHelp is what to do about a tool in each state.
function toolHelp(t: Tool): string {
  const name = t.tool === 'claude' ? 'Claude Code' : t.tool
  switch (t.state) {
    case 'ok': return `✓ ${name} ${t.version ?? ''}, ${signedIn(t.account)}`
    case 'missing': return `${name} is not installed. ${t.error ?? ''}`
    case 'too_old': return `${name} needs updating${t.tool === 'claude' ? ': run claude update' : ''}. ${t.error ?? ''}`
    case 'signed_out': return `${name} ${t.version ?? ''} is installed but not signed in${t.tool === 'claude' ? ': run claude in a terminal and sign in with /login' : ''}.`
    default: return `⚠ ${name}: ${t.error ?? 'did not answer'}`
  }
}

// Tools shows what each agent tool's probe found, and checks again on demand.
export function Tools({ setup, onSetup }: { setup: Setup; onSetup: (s: Setup) => void }) {
  const { busy, error, run } = useAction()
  const again = () => run(async () => onSetup(await post<Setup>('/api/setup/tools', {})))
  return (
    <div class="tools">
      {setup.tools.length === 0 ? <p>vuoro knows no agent tool on this machine.</p> : (
        <ul>{setup.tools.map((t) => <li key={t.name} class={t.state === 'ok' ? '' : 'warn'}>{toolHelp(t)}</li>)}</ul>
      )}
      {setup.tools.some((t) => t.state !== 'ok') && <p class="muted">Reviews need a tool that is installed and signed in. You can still open changes and read your inbox without one.</p>}
      {error && <p class="alert" role="alert">⚠ The check did not run, so this is what vuoro found before: {error}</p>}
      <button type="button" disabled={busy} onClick={() => void again()}>{busy ? 'Checking…' : 'Check again'}</button>
    </div>
  )
}

function describeSource(f: ForgeForm & { env_set?: boolean }): string {
  if (f.token_env) return `$${f.token_env}${f.env_set ? '' : ', which is not set'}`
  return f.token_command ? joinCommand(f.token_command) : 'nothing'
}

// Forges lists the configured forges, with editing where the file allows it.
export function Forges({ setup, onSetup }: { setup: Setup; onSetup: (s: Setup) => void }) {
  const [editing, setEditing] = useState<string>() // a forge's name, or '' for a new one
  const [removing, setRemoving] = useState<string>()
  const { busy, error, run } = useAction()
  const remove = (name: string) => run(async () => {
    onSetup(await post<Setup>('/api/setup/forges/remove', { base: setup.base, name }))
    setRemoving(undefined)
  })
  const saved = (s: Setup) => { setEditing(undefined); onSetup(s) }
  return (
    <div class="forges">
      {setup.forges.length === 0 && <p>No forge yet, so changes open anonymously and only public projects work.</p>}
      <ul>
        {setup.forges.map((f) => (
          <li key={f.name}>
            {editing === f.name ? (
              <ForgeEditor setup={setup} initial={f} original={f.name} onSaved={saved} onCancel={() => setEditing(undefined)} />
            ) : (
              <>
                <strong>{f.name}</strong> as @{f.username} <span class="muted">· token from <span class="mono">{describeSource(f)}</span></span>
                {setup.editable && <>
                  {' '}<button class="link" onClick={() => setEditing(f.name)}>Edit</button>
                  {removing === f.name ? (
                    <> · Remove {f.name}? <button class="link" disabled={busy} onClick={() => void remove(f.name)}>{busy ? 'Removing…' : 'Remove'}</button> <button class="link" disabled={busy} onClick={() => setRemoving(undefined)}>Keep</button></>
                  ) : <> <button class="link" onClick={() => setRemoving(f.name)}>Remove</button></>}
                </>}
              </>
            )}
          </li>
        ))}
      </ul>
      {error && <p class="alert" role="alert">⚠ {error}</p>}
      {setup.editable && (editing === '' ? (
        <ForgeEditor setup={setup} onSaved={saved} onCancel={() => setEditing(undefined)} />
      ) : (
        <button type="button" onClick={() => setEditing('')}>Add a forge</button>
      ))}
      {setup.config_path && <p class="muted">Saved in <span class="mono">{setup.config_path}</span>, which you can also edit by hand.</p>}
      {!setup.editable && setup.config_error && <p class="warn">Setup cannot save until the file loads again. Fix it by hand, then reopen this panel.</p>}
    </div>
  )
}

// Background sets how vuoro runs with its window closed (D55): how often it
// refreshes the inbox, whether it notifies, and whether it starts at login.
// The first two are the configuration file's; start at login is the
// system's. Each change is saved as it is made, and onSaving is given each
// save as it starts, answering whether it succeeded, so a caller about to
// leave can wait for it and stay when it failed.
export function Background({ setup, onSetup, onSaving }: { setup: Setup; onSetup: (s: Setup) => void; onSaving?: (saved: Promise<boolean>) => void }) {
  const b: BackgroundView = setup.background
  const [minutes, setMinutes] = useState(String(b.refresh_minutes))
  const { busy, error, run } = useAction()
  useEffect(() => setMinutes(String(b.refresh_minutes)), [b.refresh_minutes])
  const n = Number(minutes)
  const valid = Number.isInteger(n) && n >= 1 && n <= 1440
  const track = (path: string, body: object) => {
    const saved = run(async () => { onSetup(await post<Setup>(path, body)); return true }).then((ok) => {
      // A save that failed leaves the file as it was, so the field shows
      // what is saved rather than what was tried.
      if (!ok) setMinutes(String(b.refresh_minutes))
      return ok === true
    })
    onSaving?.(saved)
  }
  const save = (refresh: number, notify: boolean) => {
    if (refresh !== b.refresh_minutes || notify !== b.notify) track('/api/setup/background', { base: setup.base, refresh_minutes: refresh, notify })
  }
  const login = (enabled: boolean) => track('/api/setup/autostart', { enabled })
  const off = !setup.editable || busy
  return (
    <div class="background">
      <label>Refresh the inbox every{' '}
        <input type="number" min={1} max={1440} step={1} inputMode="numeric" value={minutes} disabled={off}
          onInput={(e) => setMinutes((e.currentTarget as HTMLInputElement).value)}
          onChange={() => { if (valid) save(n, b.notify) }} /> minutes, with the window open or closed
      </label>
      {!valid && <p class="warn">Choose from 1 to 1440 minutes.</p>}
      <label><input type="checkbox" checked={b.notify} disabled={off}
        onChange={(e) => save(b.refresh_minutes, (e.currentTarget as HTMLInputElement).checked)} /> Notify me when a change becomes my turn, or a long review finishes</label>
      {b.login ? (
        <label><input type="checkbox" checked={b.autostart} disabled={busy}
          onChange={(e) => login((e.currentTarget as HTMLInputElement).checked)} /> Start vuoro in the tray when I log in</label>
      ) : <p class="muted">vuoro cannot start at login on this system.</p>}
      {b.login_error && <p class="warn">{b.login_error}</p>}
      {error && <p class="alert" role="alert">⚠ {error}</p>}
    </div>
  )
}

// presets are the public forges most people review on, so choosing one
// fills in its kind and address; a forge of the person's own is the last.
const presets = [
  { id: 'github', label: 'GitHub', detail: 'github.com', kind: 'github', url: 'https://github.com' },
  { id: 'gitlab', label: 'GitLab', detail: 'gitlab.com', kind: 'gitlab', url: 'https://gitlab.com' },
  { id: 'codeberg', label: 'Codeberg', detail: 'codeberg.org', kind: 'forgejo', url: 'https://codeberg.org' },
  { id: 'own', label: 'Your own', detail: 'Forgejo, Gitea, GitLab, or GitHub Enterprise', kind: 'forgejo', url: '' },
]

type Step = 'forge' | 'token' | 'tools' | 'background'
const steps: { id: Step; label: string }[] = [
  { id: 'forge', label: 'Forge' },
  { id: 'token', label: 'Token' },
  { id: 'tools', label: 'Agent tools' },
  { id: 'background', label: 'Background' },
]

// FirstRun walks a fresh install through setup one step at a time: the
// forge, its token, the agent tools, and how vuoro runs in the background.
// The forge is saved at the end of its token step, so leaving setup early
// keeps what was finished.
export function FirstRun({ setup, onSetup, onDone }: { setup: Setup; onSetup: (s: Setup) => void; onDone: () => void }) {
  const [step, setStep] = useState<Step>('forge')
  // Each forge added gets a fresh form.
  const [form, setForm] = useState(0)
  // Going on without a forge skips the forge's steps rather than doing them.
  const [skipped, setSkipped] = useState(false)
  // The background step's last save. Leaving the step waits for it, and a
  // save that failed keeps the step open once, with its error showing. The
  // step then shows what is saved, so the next Back or Finish leaves with it.
  const saving = useRef<Promise<boolean>>()
  const [leaving, setLeaving] = useState(false)
  const leave = async (then: () => void) => {
    setLeaving(true)
    const ok = await (saving.current ?? Promise.resolve(true))
    saving.current = undefined
    setLeaving(false)
    if (ok) then()
  }
  const go = (to: Step) => {
    if (to === 'forge' || to === 'token') setSkipped(false)
    setStep(to)
  }
  const at = steps.findIndex((s) => s.id === step)
  return (
    <section class="first-run" aria-labelledby="first-run-title">
      <h2 id="first-run-title">Set up vuoro</h2>
      <ol class="stepper" aria-label="Setup steps">
        {steps.map((s, i) => {
          const state = i < at && i < 2 && skipped ? 'skipped' : i < at ? 'done' : i === at ? 'current' : ''
          return (
            <li key={s.id} class={state} aria-current={i === at ? 'step' : undefined}>
              <span class="dot">{state === 'done' ? '✓' : state === 'skipped' ? '–' : i + 1}</span> {s.label}
            </li>
          )
        })}
      </ol>
      {step === 'forge' || step === 'token' ? (
        <ForgeSteps key={form} setup={setup} step={step} onStep={go} onDone={onDone}
          onSkip={() => { setSkipped(setup.forges.length === 0); setStep('tools') }}
          onSaved={(s) => { onSetup(s); setForm((n) => n + 1); setStep('tools') }} />
      ) : step === 'tools' ? (
        <div class="step">
          <h3>Check your agent tools</h3>
          <p class="lead">vuoro runs reviews with the agent tools you already have. You can read your inbox and open changes without one.</p>
          <Tools setup={setup} onSetup={onSetup} />
          <div class="step-actions">
            <button type="button" onClick={() => go('forge')}>Back</button>
            <span class="spacer" />
            <button type="button" class="primary" onClick={() => setStep('background')}>Next</button>
          </div>
        </div>
      ) : (
        <div class="step">
          <h3>When the window is closed</h3>
          <p class="lead">vuoro can keep your inbox fresh from the tray, and tell you when it is your turn. Each choice is saved as you make it.</p>
          <Background setup={setup} onSetup={onSetup} onSaving={(p) => { saving.current = p }} />
          <div class="step-actions">
            <button type="button" disabled={leaving} onClick={() => void leave(() => setStep('tools'))}>Back</button>
            <span class="spacer" />
            <button type="button" class="primary" disabled={leaving} onClick={() => void leave(onDone)}>{leaving ? 'Saving…' : setup.forges.length > 0 ? 'Show my inbox' : 'Finish'}</button>
          </div>
        </div>
      )}
    </section>
  )
}

// ForgeSteps are the first run's first two steps: choosing a forge, and
// connecting to it with a token.
function ForgeSteps({ setup, step, onStep, onSaved, onSkip, onDone }: {
  setup: Setup; step: Step; onStep: (s: Step) => void; onSaved: (s: Setup) => void; onSkip: () => void; onDone: () => void
}) {
  const f = useForgeForm(setup)
  const [preset, setPreset] = useState<string>()
  const choose = (p: typeof presets[number]) => {
    setPreset(p.id)
    f.setKind(p.kind)
    f.setURL(p.url)
  }
  const added = setup.forges
  if (step === 'forge') {
    return (
      <div class="step">
        <h3>{added.length > 0 ? 'Add another forge?' : 'Where do you review code?'}</h3>
        {added.length > 0 ? (
          <ul class="added">{added.map((a) => <li key={a.name}>✓ <strong>{a.name}</strong> as @{a.username}</li>)}</ul>
        ) : <p class="lead">vuoro gathers the reviews that wait for you from each forge you add. You can add more later.</p>}
        <div class="presets" role="group" aria-label="Forge">
          {presets.map((p) => (
            <button type="button" key={p.id} class="preset" aria-pressed={preset === p.id} onClick={() => choose(p)}>
              <strong>{p.label}</strong><span class="muted">{p.detail}</span>
            </button>
          ))}
        </div>
        {preset === 'own' && (
          <div class="row own">
            <label class="field">Software
              <select value={f.kind} onChange={(e) => f.setKind((e.target as HTMLSelectElement).value)}>
                {kinds.map((k) => <option value={k.id} key={k.id}>{k.label}</option>)}
              </select>
            </label>
            <label class="field grow">Address
              <input type="url" placeholder="https://git.example.com" value={f.url} autoFocus onInput={(e) => f.setURL((e.target as HTMLInputElement).value)} />
            </label>
          </div>
        )}
        {preset === 'own' && f.url.trim() && !f.host && <p class="warn">Use the forge's https address, such as https://git.example.com, with nothing after the host.</p>}
        <div class="step-actions">
          <button type="button" class="link" onClick={onDone}>{added.length > 0 ? 'Finish later' : 'Skip setup for now'}</button>
          <span class="spacer" />
          <button type="button" onClick={onSkip}>{added.length > 0 ? 'No, continue' : 'Continue without a forge'}</button>
          <button type="button" class="primary" disabled={!f.host} onClick={() => onStep('token')}>Next</button>
        </div>
        {added.length === 0 && <p class="muted">Without a forge you can still open a public change by its URL above.</p>}
      </div>
    )
  }
  return (
    <form class="step" onSubmit={(e) => { e.preventDefault(); if (f.canSave && !f.busy) f.save(onSaved) }}>
      <h3>Connect to {f.host}</h3>
      <p class="lead">vuoro reads your reviews and posts the ones you send with a personal access token of yours.</p>
      <TokenSteps f={f} setup={setup} />
      <MoreOptions f={f} />
      {f.error && <p class="alert" role="alert">⚠ {f.error}</p>}
      <div class="step-actions">
        <button type="button" disabled={f.busy} onClick={() => onStep('forge')}>Back</button>
        <span class="spacer" />
        <button class="primary" disabled={f.busy || !f.canSave} title={f.canSave ? undefined : f.tested ? 'This token cannot be saved' : 'Check the token first'}>{f.saving.busy ? 'Saving…' : 'Save and continue'}</button>
      </div>
    </form>
  )
}
