import { describe, expect, it } from 'vitest'
import { parsePatch, placeAll } from './diff'

const patch = '@@ -1,3 +1,4 @@\n package calc\n-old\n+new\n+more\n tail\n@@ -40 +41 @@\n-q\n+r\n\\ No newline at end of file\n'

describe('parsePatch', () => {
  it('numbers each side', () => {
    const rows = parsePatch(patch)
    expect(rows.map((r) => [r.kind, r.old, r.new])).toEqual([
      ['hunk', undefined, undefined],
      ['ctx', 1, 1],
      ['del', 2, undefined],
      ['add', undefined, 2],
      ['add', undefined, 3],
      ['ctx', 3, 4],
      ['hunk', undefined, undefined],
      ['del', 40, undefined],
      ['add', undefined, 41],
      ['note', undefined, undefined],
    ])
  })
})

describe('placeAll', () => {
  it('puts each item under its line, and keeps the rest', () => {
    const rows = parsePatch(patch)
    const items = [
      { id: 'a', side: 'head' as const, line: 3 },
      { id: 'b', side: 'base' as const, line: 2 },
      { id: 'c', side: 'base' as const, line: 3 }, // the context line, by its base number
      { id: 'd', side: 'head' as const, line: 99 },
    ]
    const { at, rest } = placeAll(rows, items, (t) => t)
    expect(at.get(4)?.map((t) => t.id)).toEqual(['a'])
    expect(at.get(2)?.map((t) => t.id)).toEqual(['b'])
    expect(at.get(5)?.map((t) => t.id)).toEqual(['c'])
    expect(rest.map((t) => t.id)).toEqual(['d'])
  })
})
