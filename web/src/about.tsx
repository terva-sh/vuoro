// About vuoro: which vuoro this is, what it is built with, the agent tools it
// found, and where it keeps its files, with a copy for a bug report.
import { useEffect, useRef, useState } from 'preact/hooks'
import { errorText } from './action'
import { api } from './api'
import { Modal } from './modal'
import type { Setup } from './types'

export interface AboutInfo {
  version: string
  commit?: string
  modified?: boolean
  built?: string
  go: string
  platform: string
  modules: { path: string; version: string; replaced_by?: string }[]
  config_path?: string
  state_dir?: string
}

// aboutRows are the pane's rows, in sections, as label and value. The copy
// for a bug report is the same rows as text.
export function aboutRows(a: AboutInfo, setup?: Setup): [string, [string, string][]][] {
  const source = a.commit ? `${a.commit.slice(0, 12)}${a.modified ? ' (with uncommitted changes)' : ''}` : 'not recorded'
  const tools: [string, string][] = (setup?.tools ?? []).map((t) => [
    t.tool === 'claude' ? 'Claude Code' : t.tool,
    `${t.version ?? 'unknown version'}${t.state === 'ok' ? '' : `, ${t.state.replace('_', ' ')}`}`,
  ])
  return [
    ['vuoro', [
      ['Version', a.version],
      ['Commit', source],
      ...(a.built ? [['Committed', new Date(a.built).toLocaleString()] as [string, string]] : []),
      ['Platform', a.platform],
      ['Go', a.go],
    ]],
    ['Built with', a.modules.map((m) => [m.path, m.replaced_by ? `${m.version}, replaced by ${m.replaced_by}` : m.version])],
    ['Agent tools', tools.length > 0 ? tools : [['None found', '']]],
    ['Files', [
      ['Configuration', a.config_path ?? 'none'],
      ['State', a.state_dir ?? 'none'],
    ]],
  ]
}

export function aboutText(a: AboutInfo, setup?: Setup): string {
  return aboutRows(a, setup).map(([title, rows]) => `${title}\n${rows.map(([k, v]) => `  ${k}: ${v}`).join('\n')}`).join('\n\n')
}

export function About({ setup, onClose }: { setup?: Setup; onClose: () => void }) {
  const [about, setAbout] = useState<AboutInfo>()
  const [error, setError] = useState<string>()
  const [copied, setCopied] = useState(false)
  // The clipboard can be missing or refuse, and then the text is shown,
  // selected, to copy by hand.
  const [byHand, setByHand] = useState(false)
  const close = useRef<HTMLButtonElement>(null)
  useEffect(() => { api<AboutInfo>('/api/about').then(setAbout).catch((e: unknown) => setError(errorText(e))) }, [])
  const copy = async () => {
    if (!about) return
    const ok = await (navigator.clipboard?.writeText(aboutText(about, setup)).then(() => true, () => false) ?? Promise.resolve(false))
    setCopied(ok)
    setByHand(!ok)
  }
  return (
    <Modal title="About vuoro" onClose={onClose} focus={close} class="about">
      {error && <p class="alert" role="alert">⚠ {error}</p>}
      {!about && !error && <p class="muted">Reading…</p>}
      {about && aboutRows(about, setup).map(([title, rows]) => (
        <section key={title}>
          <h3>{title}</h3>
          <dl>{rows.map(([k, v]) => <div key={k}><dt>{k}</dt><dd class="mono">{v}</dd></div>)}</dl>
        </section>
      ))}
      {about && byHand && <>
        <p class="warn">The clipboard is not available here, so select the text and copy it.</p>
        <textarea class="mono by-hand" readOnly rows={8} value={aboutText(about, setup)} ref={(t) => t?.select()} />
      </>}
      <div class="actions">
        <button type="button" disabled={!about} onClick={() => void copy()}>{copied ? 'Copied' : 'Copy for a bug report'}</button>
        <button type="button" class="primary" ref={close} onClick={onClose}>Close</button>
      </div>
    </Modal>
  )
}
