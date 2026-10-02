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
