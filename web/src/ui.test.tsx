// @vitest-environment jsdom
import { render } from 'preact'
import { describe, expect, it } from 'vitest'
import { CommentBubble, worstSeverity } from './ui'

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
