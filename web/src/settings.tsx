// Settings: everything vuoro's configuration and preferences hold, on one
// page with a section for each. It reads setup again as it opens, which picks
// up a hand edit or a repaired file before anything is edited from it.
import { useEffect, useState } from 'preact/hooks'
import { api, post } from './api'
import { waive } from './confirm'
import { Presets } from './presets'
import { SchemePicker } from './scheme'
import { Background, Forges, Tools } from './setup'
import type { Scheme, Setup } from './types'

const sections = [
  { id: 'settings-forges', label: 'Forges' },
  { id: 'settings-tools', label: 'Agent tools' },
  { id: 'settings-background', label: 'Background' },
  { id: 'settings-appearance', label: 'Appearance' },
  { id: 'settings-confirmations', label: 'Confirmations' },
  { id: 'settings-getting-started', label: 'Getting started' },
  { id: 'settings-about', label: 'About' },
]

export function Settings({ setup, onSetup, scheme, onScheme, version, onAbout, onClose, onTour }: {
  setup: Setup; onSetup: (s: Setup) => void; scheme: Scheme; onScheme: (s: Scheme) => void
  version?: string; onAbout: () => void; onClose: () => void
  onTour: () => void // replays the getting-started tour
}) {
  // Presets is a page of its own rather than one more section, because a
  // preset holds a list of reviewers with three fields each and two optional
  // roles besides, which does not sit in a settings column.
  const [page, setPage] = useState<'' | 'presets'>('')
  useEffect(() => { api<Setup>('/api/setup').then(onSetup).catch(() => undefined) }, [])
  // A section only exists on the settings page, so a jump from Presets waits
  // for it to render again.
  const show = (id: string) => {
    setPage('')
    setTimeout(() => document.getElementById(id)?.scrollIntoView?.({ behavior: 'smooth' }), 0)
  }
  return (
    <div class="settings">
      <nav class="settings-nav" aria-label="Settings sections">
        <button type="button" class="link back" onClick={onClose}>← Back</button>
        <h2>Settings</h2>
        <ul>
          {sections.map((s) => <li key={s.id}><a href={`#${s.id}`} onClick={(e) => { e.preventDefault(); show(s.id) }}>{s.label}</a></li>)}
          <li>
            <button type="button" class={page === 'presets' ? 'link on' : 'link'} aria-current={page === 'presets' ? 'page' : undefined}
              onClick={() => setPage('presets')}>Presets</button>
          </li>
        </ul>
      </nav>
      {page === 'presets' ? (
        <div class="settings-body">
          <section aria-labelledby="settings-presets-title">
            <h2 id="settings-presets-title">Presets</h2>
            <p class="lead">The reviewers a run starts. vuoro ships three, and you can copy one to make it your own.</p>
            <Presets setup={setup} onSetup={onSetup} />
          </section>
        </div>
      ) : (
      <div class="settings-body">
        <section id="settings-forges" aria-labelledby="settings-forges-title">
          <h2 id="settings-forges-title">Forges</h2>
          <p class="lead">Where vuoro reads your reviews, and how far each token reaches.</p>
          <Forges setup={setup} onSetup={onSetup} />
        </section>
        <section id="settings-tools" aria-labelledby="settings-tools-title">
          <h2 id="settings-tools-title">Agent tools</h2>
          <p class="lead">vuoro runs reviews with the agent tools you already have.</p>
          <Tools setup={setup} onSetup={onSetup} />
        </section>
        <section id="settings-background" aria-labelledby="settings-background-title">
          <h2 id="settings-background-title">Background</h2>
          <p class="lead">How vuoro runs with its window closed. Each choice is saved as you make it.</p>
          <Background setup={setup} onSetup={onSetup} />
        </section>
        <section id="settings-appearance" aria-labelledby="settings-appearance-title">
          <h2 id="settings-appearance-title">Appearance</h2>
          <label class="field">Colour scheme <SchemePicker scheme={scheme} onScheme={onScheme} /></label>
        </section>
        <section id="settings-confirmations" aria-labelledby="settings-confirmations-title">
          <h2 id="settings-confirmations-title">Confirmations</h2>
          {setup.waived.length === 0 ? (
            <p>vuoro asks before submitting a review or approving.</p>
          ) : (
            <p>
              You turned off asking before {setup.waived.map((a) => (a === 'submit' ? 'submitting' : 'approving')).join(' and ')}.{' '}
              <button type="button" onClick={() => void waive(setup.waived, true).then(onSetup).catch(() => undefined)}>Ask again</button>
            </p>
          )}
        </section>
        <section id="settings-getting-started" aria-labelledby="settings-getting-started-title">
          <h2 id="settings-getting-started-title">Getting started</h2>
          <p class="lead">A short tour of the window, and a hint on each empty screen. Neither starts a run or posts anything.</p>
          <p>
            <button type="button" onClick={onTour}>Replay the tour</button>{' '}
            <button type="button" disabled={(setup.hints ?? []).length === 0}
              onClick={() => void post<{ hints: Setup['hints'] }>('/api/prefs/hints/reset').then((p) => onSetup({ ...setup, hints: p.hints })).catch(() => undefined)}>
              Show dismissed hints again
            </button>
          </p>
        </section>
        <section id="settings-about" aria-labelledby="settings-about-title">
          <h2 id="settings-about-title">About</h2>
          <p>vuoro {version ?? ''} <button type="button" onClick={onAbout}>Versions and files</button></p>
        </section>
      </div>
      )}
    </div>
  )
}
