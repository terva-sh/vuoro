// @vitest-environment jsdom
import { render } from 'preact'
import { act } from 'preact/test-utils'
import { describe, expect, it } from 'vitest'
import { collapseAt, DiffBrowser, FileView, manyFiles, useFilePicker } from './diffview'
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
    // Collapsed, it is the header alone, which carries the line count.
    expect(root.querySelector('.file')!.children.length).toBe(1)
    expect(root.querySelector('.file-head')!.textContent).toContain('Show 5001 lines')
    act(() => (root.querySelector('.file-head .show-lines') as HTMLButtonElement).click())
    expect(root.querySelectorAll('.diff tr').length).toBe(5001)
  })

  it('starts collapsed in a large change, opens when picked, and folds when another is picked', () => {
    const root = document.createElement('div')
    const view = (picked?: number) => act(() => render(<FileView file={big(10)} threads={[]} drafts={[]} collapsed picked={picked} />, root))
    const rows = () => root.querySelectorAll('.diff tr').length
    view()
    expect(rows()).toBe(0)
    view(1)
    expect(rows()).toBe(11)
    view(undefined)
    expect(rows()).toBe(0)
  })

  it('leaves a file the person opened by hand open when another is picked', () => {
    const root = document.createElement('div')
    const view = (picked?: number) => act(() => render(<FileView file={big(10)} threads={[]} drafts={[]} collapsed picked={picked} />, root))
    const rows = () => root.querySelectorAll('.diff tr').length
    view()
    act(() => (root.querySelector('.file-head .show-lines') as HTMLButtonElement).click())
    view(1)
    view(undefined)
    expect(rows()).toBe(11)
    // Picked, then closed by hand, a second pick opens it again.
    view(2)
    act(() => (root.querySelector('.file-toggle') as HTMLButtonElement).click())
    expect(rows()).toBe(0)
    view(3)
    expect(rows()).toBe(11)
  })

  it('opens a small file at once', () => {
    const root = document.createElement('div')
    act(() => render(<FileView file={big(collapseAt - 10)} threads={[]} drafts={[]} />, root))
    expect(root.querySelectorAll('.diff tr').length).toBe(collapseAt - 9)
  })
})

describe('keepText', () => {
  it('matches the server: fenced evidence, then a suggestion block over the range', async () => {
    const { keepText } = await import('./comments')
    const d = {
      id: 'd1', severity: 'high', title: 'Add subtracts', description: 'It returns a - b.', evidence: 'return a - b',
      anchor: { path: 'calc.go', side: 'head', start_line: 3, end_line: 4 }, suggestion: '\treturn a + b\n\t// sums',
      status: 'pending', revision: 1, session: 's1', drafted_by: 'x', updated: '',
    } as const
    // The same vector as app/keep_test.go.
    expect(keepText(d, true)).toBe('**Add subtracts**\n\nIt returns a - b.\n\n```\nreturn a - b\n```\n\n```suggestion:-1+0\n\treturn a + b\n\t// sums\n```')
    expect(keepText({ ...d, evidence: 'x := "```"', suggestion: undefined }, true)).toBe('**Add subtracts**\n\nIt returns a - b.\n\n````\nx := "```"\n````')
    // A forge that cannot apply a suggestion gets a plain replacement (D43).
    expect(keepText(d, true, true)).toBe('**Add subtracts**\n\nIt returns a - b.\n\n```\nreturn a - b\n```\n\n```suggestion\n\treturn a + b\n\t// sums\n```')
    expect(keepText(d, false)).toBe('**Add subtracts**\n\nIt returns a - b.\n\n```\nreturn a - b\n```\n\nSuggested replacement for lines 3–4:\n\n```\n\treturn a + b\n\t// sums\n```')
  })
})

describe('DiffBrowser', () => {
  it('lists every file, and a pick marks it and opens it when many files start collapsed', () => {
    const files: FileDiff[] = Array.from({ length: manyFiles + 1 }, (_, i) => ({ ...big(2), old_path: `f${i}.go`, new_path: `f${i}.go` }))
    const Probe = () => {
      const { picked, pick } = useFilePicker('x-')
      return <DiffBrowser files={files} picked={picked} onPick={pick} idPrefix="x-" notes={() => ({})} />
    }
    const root = document.createElement('div')
    act(() => render(<Probe />, root))
    expect(root.querySelectorAll('.file-list .file-link').length).toBe(manyFiles + 1)
    expect(root.querySelectorAll('.file.collapsed').length).toBe(manyFiles + 1)
    act(() => (root.querySelectorAll('.file-list .file-link')[3] as HTMLButtonElement).click())
    expect(root.querySelector('.file-link.current')?.getAttribute('title')).toBe('f3.go')
    expect(root.querySelector('#x-file-f3_go')?.classList.contains('collapsed')).toBe(false)
    expect(root.querySelectorAll('.file.collapsed').length).toBe(manyFiles)
  })
})
