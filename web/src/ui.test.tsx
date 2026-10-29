// @vitest-environment jsdom
import { render } from 'preact'
import { describe, expect, it } from 'vitest'
import { CommentBubble, cost, unreported, worstSeverity } from './ui'

describe('CommentBubble', () => {
  it('takes the worst open finding, and shows its symbol beside the colour', () => {
    expect(worstSeverity([{ severity: 'low' }, { severity: 'high' }, { severity: 'medium' }])).toBe('high')
    expect(worstSeverity([])).toBeUndefined()
    const root = document.createElement('div')
    render(<CommentBubble count={3} severity="high" />, root)
    const b = root.querySelector('.bubble')!
    expect(b.classList.contains('severity-high')).toBe(true)
    expect(b.textContent).toBe('💬 3 ▲')
    render(<CommentBubble />, root)
    expect(root.querySelector('.bubble')!.className).not.toContain('severity-')
  })
})

describe('cost', () => {
  it('never shows a cost the tool did not report as $0.00', () => {
    const used = { input: 10, output: 5, cache_read: 0, cache_write: 0, cost_usd: 0 }
    expect(unreported(used)).toBe(true)
    expect(unreported({ ...used, cost_usd: 0.02 })).toBe(false)
    expect(unreported({ ...used, input: 0, output: 0 })).toBe(false) // nothing used yet
    expect(cost(0, unreported(used))).toBe('cost unknown')
    expect(cost(0.25, true)).toBe('at least $0.25')
    expect(cost(0.25)).toBe('$0.25')
    expect(cost(0)).toBe('$0.00')
  })
})
