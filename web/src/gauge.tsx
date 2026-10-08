// The access gauge (D57): after a token's check, how far the token reaches,
// from not enough to read, through read only and full, to broader than vuoro
// needs. Read only and full are both fine; the two ends are not.
import { useEffect, useState } from 'preact/hooks'
import type { Access, AccessLevel } from './types'

const zones: { level: Exclude<AccessLevel, 'unknown'>; label: string }[] = [
  { level: 'none', label: 'Can’t read' },
  { level: 'read', label: 'Read only' },
  { level: 'full', label: 'Full' },
  { level: 'broad', label: 'Too broad' },
]

// A point on the gauge's arc, at angle degrees from the left end.
const at = (deg: number, r: number) => {
  const a = Math.PI - (deg * Math.PI) / 180
  return [100 + r * Math.cos(a), 100 - r * Math.sin(a)].map((n) => n.toFixed(2)).join(' ')
}
const arc = (from: number, to: number) => `M ${at(from, 80)} A 80 80 0 0 1 ${at(to, 80)}`

// verdict says what the level means for the person, and what to change.
export function verdict(a: Access): { title: string; detail?: string } {
  const list = (xs?: string[]) => (xs ?? []).join(', ')
  switch (a.level) {
    case 'none': return {
      title: 'This token cannot read your reviews.',
      detail: [a.missing?.length && `Make one with ${list(a.missing)}.`, a.broad?.length && `Leave out ${list(a.broad)}, which vuoro does not need.`].filter(Boolean).join(' ') || undefined,
    }
    case 'read': return { title: 'Read only: vuoro reviews, and you post by hand.', detail: a.missing?.length ? `To post from vuoro, add ${list(a.missing)}.` : undefined }
    case 'full': return { title: 'Just right: vuoro reads, and posts what you send.' }
    case 'broad': return { title: 'Broader than vuoro needs.', detail: `It also holds ${list(a.broad)}. vuoro never uses them, but anything that gets hold of the token could. Consider a token without them.` }
    default: return { title: 'The forge does not say what this token may do.' }
  }
}

export function Gauge({ access, compact = false }: { access: Access; compact?: boolean }) {
  const i = zones.findIndex((z) => z.level === access.level)
  const known = i >= 0
  // The needle sweeps up from the left once it has a reading.
  const [angle, setAngle] = useState(0)
  useEffect(() => {
    const id = requestAnimationFrame(() => setAngle(known ? i * 45 + 22.5 : 90))
    return () => cancelAnimationFrame(id)
  }, [i, known])
  const { title, detail } = verdict(access)
  return (
    <div class={`gauge ${known ? access.level : 'unknown'}${compact ? ' compact' : ''}`} role="status">
      <svg viewBox="0 0 200 112" aria-hidden="true">
        {zones.map((z, n) => <path key={z.level} class={`zone ${z.level}${n === i ? ' on' : ''}`} d={arc(n * 45 + 1, n * 45 + 44)} />)}
        {known && <g class="needle" style={{ transform: `rotate(${angle - 90}deg)` }}>
          <line x1="100" y1="100" x2="100" y2="34" />
        </g>}
        {known && <circle class="hub" cx="100" cy="100" r="6" />}
      </svg>
      <ol class="zones" aria-label="How far the token reaches">
        {zones.map((z, n) => <li key={z.level} aria-current={n === i ? 'true' : undefined}>{z.label}</li>)}
      </ol>
      <p class="verdict"><strong>{title}</strong>{detail && <> {detail}</>}</p>
      {access.note && <p class="muted">{access.note}</p>}
      {access.scopes && access.scopes.length > 0 && <p class="muted scopes">Scopes: <span class="mono">{access.scopes.join(', ')}</span></p>}
    </div>
  )
}
