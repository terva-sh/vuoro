// Small pieces every view shares. Colour never carries meaning alone: each
// status and severity also has a symbol and a word.
import type { ComponentChildren } from 'preact'
import type { Severity, Status } from './types'

const statusMark: Record<Status, string> = {
  preparing: '…', working: '●', idle: '○', ended: '◌', failed: '⚠', closed: '■',
}

export function StatusBadge({ status }: { status: Status }) {
  return <span class={`badge status-${status}`}>{statusMark[status]} {status}</span>
}

const severityMark: Record<Severity, string> = {
  critical: '▲▲', high: '▲', medium: '◆', low: '▽', info: 'ⓘ',
}

export function SeverityBadge({ severity }: { severity: Severity }) {
  return <span class={`badge severity-${severity}`}>{severityMark[severity]} {severity}</span>
}

export function ago(iso: string): string {
  const s = (Date.now() - Date.parse(iso)) / 1000
  if (!Number.isFinite(s)) return ''
  if (s < 60) return 'just now'
  if (s < 3600) return `${Math.floor(s / 60)}m ago`
  if (s < 86400) return `${Math.floor(s / 3600)}h ago`
  return `${Math.floor(s / 86400)}d ago`
}

export const short = (sha: string) => sha.slice(0, 8)

export const usd = (n: number) => (n < 0.01 && n > 0 ? '<$0.01' : `$${n.toFixed(2)}`)

// Prose is shown as written, wrapped, and never parsed as HTML.
export function Prose({ text }: { text: string }) {
  return <div class="prose">{text}</div>
}

export function Section({ title, children, aside }: { title: string; children: ComponentChildren; aside?: ComponentChildren }) {
  return (
    <section class="section">
      <header class="section-head">
        <h2>{title}</h2>
        {aside}
      </header>
      {children}
    </section>
  )
}
