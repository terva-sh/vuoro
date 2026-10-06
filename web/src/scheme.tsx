// The colour scheme. The design stylesheet follows the system's unless the
// root element names one in data-scheme. The choice is kept by the server,
// because the window's port, and so its local storage, changes every launch.
import { post } from './api'
import type { Scheme } from './types'

const schemes: { value: Scheme; label: string }[] = [
  { value: '', label: 'System' },
  { value: 'light', label: 'Light' },
  { value: 'dark', label: 'Dark' },
]

// applyScheme sets the scheme on the root element, or with an empty one
// takes it off so the system's applies.
export function applyScheme(scheme: Scheme, root: HTMLElement = document.documentElement) {
  if (scheme) root.dataset.scheme = scheme
  else delete root.dataset.scheme
}

// SchemePicker shows the scheme and pins another. It applies the choice at
// once, and puts the old one back if the server does not keep it.
export function SchemePicker({ scheme, onScheme }: { scheme: Scheme; onScheme: (s: Scheme) => void }) {
  const pick = (next: Scheme) => {
    const prev = scheme
    onScheme(next)
    post('/api/prefs/scheme', { scheme: next }).catch(() => onScheme(prev))
  }
  return (
    <select class="scheme" aria-label="Colour scheme" value={scheme} onChange={(e) => pick((e.target as HTMLSelectElement).value as Scheme)}>
      {schemes.map((s) => <option key={s.value} value={s.value}>{s.label}</option>)}
    </select>
  )
}
