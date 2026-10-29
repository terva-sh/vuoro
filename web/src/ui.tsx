// Small pieces every view shares. Colour never carries meaning alone: each
// status and severity also has a symbol and a word.
import type { ComponentChildren } from 'preact'
import { Markdown } from './markdown'
import type { Confidence, Level, Severity, Status, Usage } from './types'

const statusMark: Record<Status, string> = {
  preparing: '…', working: '●', idle: '○', ended: '◌', failed: '⚠', closed: '■', waiting: '⧗', cancelled: '⊘',
}

export function StatusBadge({ status }: { status: Status }) {
  return <span class={`badge status-${status}`}>{statusMark[status]} {status}</span>
}

const severityMark: Record<Severity, string> = {
  critical: '▲▲', high: '▲', medium: '◆', low: '▽', info: 'ⓘ',
}

// The confidence marks of D31, each with its word.
export const levelMark: Record<Level, string> = { confirmed: '✅', plausible: '❔', doubtful: '⛈' }

export function ConfidenceBadge({ c, mine }: { c: Confidence; mine?: boolean }) {
  return (
    <span class={`badge confidence-${c.level}`} title={c.reason}>
      {levelMark[c.level]} {c.level}{mine ? ' (yours)' : ''}
    </span>
  )
}

export const severityOrder: Severity[] = ['critical', 'high', 'medium', 'low', 'info']

// worstSeverity is the most severe of some findings, or undefined for none.
export function worstSeverity(ds: { severity: Severity }[]): Severity | undefined {
  return severityOrder.find((s) => ds.some((d) => d.severity === s))
}

// CommentBubble marks a file with comments. Outlined in the colour of the
// worst finding still to decide, and carrying its symbol, so the colour is
// never the only sign.
export function CommentBubble({ count, severity }: { count?: number; severity?: Severity }) {
  return (
    <span class={`badge bubble ${severity ? `severity-${severity}` : ''}`}
      title={severity ? `The worst finding still to decide here is ${severity}` : 'Comments'}>
      💬{count !== undefined && ` ${count}`}{severity && ` ${severityMark[severity]}`}
    </span>
  )
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

// cost is what a run or session cost, never a guess. When a tool used tokens
// and reported no cost, the known part is a floor, and with none it is
// unknown rather than $0.00.
export const cost = (n: number, unknown?: boolean) => (!unknown ? usd(n) : n > 0 ? `at least ${usd(n)}` : 'cost unknown')

// unreported says a session's cost is not all known: a turn ended stopped or
// failed before its tool reported usage, or it used tokens and reported no
// cost.
export const unreported = ({ usage: u, cost_unknown }: { usage: Usage; cost_unknown?: boolean }) =>
  !!cost_unknown || (!u.cost_usd && u.input + u.output + u.cache_read + u.cache_write > 0)

// Prose is what a person or an agent wrote, rendered as Markdown and never
// parsed as HTML.
export function Prose({ text }: { text: string }) {
  return <Markdown text={text} class="prose" />
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
