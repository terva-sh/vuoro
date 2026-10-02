// @vitest-environment jsdom
import { render } from 'preact'
import { act } from 'preact/test-utils'
import { describe, expect, it } from 'vitest'
import { collapseAt, FileView } from './diffview'
import type { FileDiff } from './types'

const big = (n: number): FileDiff => ({
  old_path: 'big.go', new_path: 'big.go', new: false, deleted: false, renamed: false, rebuilt: false, omitted: false,
  patch: `@@ -1,${n} +1,${n} @@\n` + Array.from({ length: n }, (_, i) => `+line ${i}`).join('\n') + '\n',
})

describe('FileView', () => {
  it('builds no rows for a large file until the person opens it', () => {
    const root = document.createElement('div')
    act(() => render(<FileView file={big(5000)} threads={[]} drafts={[]} />, root))
    expect(root.querySelectorAll('.diff tr').length).toBe(0)
    expect(root.textContent).toContain('Show 5001 lines')
    act(() => (root.querySelector('.pad button') as HTMLButtonElement).click())
    expect(root.querySelectorAll('.diff tr').length).toBe(5001)
  })

  it('opens a small file at once', () => {
    const root = document.createElement('div')
    act(() => render(<FileView file={big(collapseAt - 10)} threads={[]} drafts={[]} />, root))
    expect(root.querySelectorAll('.diff tr').length).toBe(collapseAt - 9)
  })
})

describe('keepText', () => {
  it('matches the server: fenced evidence, then a suggestion block over the range', async () => {
    const { keepText } = await import('./diffview')
    const d = {
      id: 'd1', severity: 'high', title: 'Add subtracts', description: 'It returns a - b.', evidence: 'return a - b',
      anchor: { path: 'calc.go', side: 'head', start_line: 3, end_line: 4 }, suggestion: '\treturn a + b\n\t// sums',
      status: 'pending', revision: 1, session: 's1', drafted_by: 'x', updated: '',
    } as const
    // The same vector as app/keep_test.go.
    expect(keepText(d)).toBe('**Add subtracts**\n\nIt returns a - b.\n\n```\nreturn a - b\n```\n\n```suggestion:-1+0\n\treturn a + b\n\t// sums\n```')
    expect(keepText({ ...d, evidence: 'x := "```"', suggestion: undefined })).toBe('**Add subtracts**\n\nIt returns a - b.\n\n````\nx := "```"\n````')
  })
})
