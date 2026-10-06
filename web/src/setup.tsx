// Setup in the window (D45). vuoro never asks for a token: the person keeps
// it where they choose, and the config names the command or the variable
// that supplies it (D29). The form tests that source against the forge
// before it saves.
import { useEffect, useState } from 'preact/hooks'
import { errorText, useAction } from './action'
import { post } from './api'
import { EditForm } from './textedit'
import type { ForgeForm, Setup, TokenSource, Tool } from './types'

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
  return kind === 'gitlab'
    ? { href: `${base}/-/user_settings/personal_access_tokens?name=vuoro&scopes=api`, scopes: 'the api scope' }
    : { href: `${base}/user/settings/applications`, scopes: 'read and write on repository and issue, and read on user' }
}

// ForgeEditor adds a forge, or changes the one named original.
export function ForgeEditor({ setup, initial, original, onSaved, onCancel }: {
  setup: Setup; initial?: ForgeForm; original?: string; onSaved: (s: Setup) => void; onCancel?: () => void
}) {
  // The editor's fields come from the file as it was when the editor opened,
  // so its save carries that file's hash: a file changed since, even one the
  // window has read again meanwhile, is refused rather than overwritten.
  const [base] = useState(setup.base)
  const [kind, setKind] = useState(initial?.kind ?? 'forgejo')
  const [url, setURL] = useState(initial?.url ?? '')
  const [name, setName] = useState(initial?.name ?? '')
  const [username, setUsername] = useState(initial?.username ?? '')
  const [clone, setClone] = useState(initial?.clone || 'ssh')
  const commandSources = setup.sources.filter((s) => s.id !== 'env')
  const [sourceID, setSourceID] = useState(initial ? (initial.token_env ? 'env' : 'command') : (setup.sources[0]?.id ?? 'env'))
  const [command, setCommand] = useState(initial?.token_command ? joinCommand(initial.token_command) : '')
  const [env, setEnv] = useState(initial?.token_env ?? '')
  const [result, setResult] = useState<{ username: string; key: string }>()
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
  const tested = result && result.key === key
  const canSave = tested && !!form.username && result.username.toLowerCase() === form.username.toLowerCase() && !!form.name
  const ready = !!host && (isEnv || (argv.length > 0 && !commandError))

  const test = async () => {
    saving.setError(undefined)
    const r = await testing.run(() => post<{ username: string; matches: boolean }>('/api/setup/test', form))
    if (!r) {
      setResult(undefined)
      return
    }
    setResult({ username: r.username, key })
    if (!form.username) setUsername(r.username)
  }
  const save = () => {
    testing.setError(undefined)
    void saving.run(async () => onSaved(await post<Setup>('/api/setup/forges', { base, original: original ?? '', forge: form })))
  }
  const page = tokenPage(kind, url)
  const label = original ? 'Save' : 'Add this forge'
  return (
    <EditForm class="forge-editor" onSave={save} onCancel={onCancel} busy={busy} canSave={!!canSave}
      save={label} saving={saving.busy ? 'Saving…' : label} hint={canSave ? undefined : 'Test the token first'}
      error={testing.error ?? saving.error}>
      <div class="row">
        <label class="field">Forge
          <select value={kind} onChange={(e) => setKind((e.target as HTMLSelectElement).value)}>
            <option value="forgejo">Forgejo</option>
            <option value="gitlab">GitLab</option>
          </select>
        </label>
        <label class="field grow">Address
          <input type="url" placeholder={kind === 'gitlab' ? 'https://gitlab.com' : 'https://codeberg.org'} value={url} onInput={(e) => setURL((e.target as HTMLInputElement).value)} />
        </label>
      </div>
      {url.trim() && !host && <p class="warn">Use the forge's https address, such as https://gitlab.com, with nothing after the host.</p>}
      <ol class="steps">
        <li>
          Make a personal access token{page ? <> on <a href={page.href} target="_blank" rel="noreferrer">{host}</a></> : ' on the forge'}, with {page?.scopes ?? 'the access vuoro needs'}. vuoro never asks for the token itself.
        </li>
        <li>
          <label class="field">Keep it in
            <select value={sourceID} onChange={(e) => { setSourceID((e.target as HTMLSelectElement).value); setResult(undefined) }}>
              {setup.sources.map((s) => <option value={s.id} key={s.id}>{s.label}</option>)}
            </select>
          </label>
          {source?.store && <p>In a terminal, store it with <code class="copyable">{joinCommand(source.store.map((w) => fill(w, host)))}</code></p>}
          {source?.note && <p class="muted">{fill(source.note, host)}</p>}
          {isEnv ? (
            <label class="field">Variable
              <input class="mono" placeholder={effectiveEnv} value={env} onInput={(e) => setEnv((e.target as HTMLInputElement).value)} />
            </label>
          ) : (
            <label class="field">vuoro reads it with
              <input class="mono" value={command} placeholder={commandSources.length > 1 ? '' : 'a program and its arguments'} onInput={(e) => setCommand((e.target as HTMLInputElement).value)} />
            </label>
          )}
          {commandError && <p class="warn">The command: {commandError}.</p>}
          {!isEnv && argv.length > 0 && !commandError && <p class="muted">vuoro will run <span class="mono">{argv.map((a) => `‹${a}›`).join(' ')}</span>, without a shell, whenever it needs the token.</p>}
        </li>
        <li>
          <button type="button" disabled={busy || !ready} onClick={() => void test()}>{testing.busy ? 'Testing…' : 'Test'}</button>{' '}
          {tested && <span class="ok">✓ Signed in as @{result.username}</span>}
          {tested && form.username && result.username.toLowerCase() !== form.username.toLowerCase() && <p class="warn">The token is @{result.username}'s, not @{form.username}'s.</p>}
        </li>
      </ol>
      <div class="row">
        <label class="field">Your username
          <input value={username} placeholder="filled in by the test" onInput={(e) => setUsername((e.target as HTMLInputElement).value)} />
        </label>
        <label class="field">Name
          <input value={name} placeholder={host || 'shown in vuoro'} onInput={(e) => setName((e.target as HTMLInputElement).value)} />
        </label>
        <label class="field">Fetch code over
          <select value={clone} onChange={(e) => setClone((e.target as HTMLSelectElement).value)}>
            <option value="ssh">SSH, with your keys</option>
            <option value="https">HTTPS, with your credential helper</option>
          </select>
        </label>
      </div>
    </EditForm>
  )
}

// toolHelp is what to do about a tool in each state.
function toolHelp(t: Tool): string {
  const name = t.tool === 'claude' ? 'Claude Code' : t.tool
  switch (t.state) {
    case 'ok': return `✓ ${name} ${t.version ?? ''}${t.account ? `, signed in (${t.account})` : ''}`
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

// FirstRun walks a fresh install through adding its forges and checking its
// agent tools.
export function FirstRun({ setup, onSetup, onDone }: { setup: Setup; onSetup: (s: Setup) => void; onDone: () => void }) {
  return (
    <section class="first-run" aria-labelledby="first-run-title">
      <h2 id="first-run-title">Set up vuoro</h2>
      <p>Add the forges you review on, and vuoro finds the reviews that wait for you. It reads and posts with a token of yours, kept where you choose; vuoro's configuration names that place, never the token.</p>
      <h3>1. Your forges</h3>
      {setup.forges.length === 0 ? (
        <ForgeEditor setup={setup} onSaved={onSetup} />
      ) : (
        <Forges setup={setup} onSetup={onSetup} />
      )}
      <h3>2. Agent tools</h3>
      <Tools setup={setup} onSetup={onSetup} />
      <div class="actions">
        <button class={setup.forges.length > 0 ? 'primary' : ''} onClick={onDone}>{setup.forges.length > 0 ? 'Done, show my inbox' : 'Skip for now'}</button>
      </div>
      {setup.forges.length === 0 && <p class="muted">Without a forge you can still open a public change by its URL above.</p>}
    </section>
  )
}
