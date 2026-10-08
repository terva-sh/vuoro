// Settings: everything vuoro's configuration and preferences hold, on one
// page with a section for each. It reads setup again as it opens, which picks
// up a hand edit or a repaired file before anything is edited from it.
import { useEffect } from 'preact/hooks'
import { api } from './api'
import { waive } from './confirm'
import { SchemePicker } from './scheme'
import { Background, Forges, Tools } from './setup'
import type { Scheme, Setup } from './types'

const sections = [
  { id: 'settings-forges', label: 'Forges' },
  { id: 'settings-tools', label: 'Agent tools' },
  { id: 'settings-background', label: 'Background' },
  { id: 'settings-appearance', label: 'Appearance' },
  { id: 'settings-confirmations', label: 'Confirmations' },
  { id: 'settings-about', label: 'About' },
]

export function Settings({ setup, onSetup, scheme, onScheme, version, onAbout, onClose }: {
  setup: Setup; onSetup: (s: Setup) => void; scheme: Scheme; onScheme: (s: Scheme) => void
  version?: string; onAbout: () => void; onClose: () => void
}) {
  useEffect(() => { api<Setup>('/api/setup').then(onSetup).catch(() => undefined) }, [])
  return (
    <div class="settings">
      <nav class="settings-nav" aria-label="Settings sections">
        <button type="button" class="link back" onClick={onClose}>← Back</button>
        <h2>Settings</h2>
        <ul>{sections.map((s) => <li key={s.id}><a href={`#${s.id}`} onClick={(e) => { e.preventDefault(); document.getElementById(s.id)?.scrollIntoView({ behavior: 'smooth' }) }}>{s.label}</a></li>)}</ul>
      </nav>
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
        <section id="settings-about" aria-labelledby="settings-about-title">
          <h2 id="settings-about-title">About</h2>
          <p>vuoro {version ?? ''} <button type="button" onClick={onAbout}>Versions and files</button></p>
        </section>
      </div>
    </div>
  )
}
