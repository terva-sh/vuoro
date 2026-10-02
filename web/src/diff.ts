// Reading a forge's unified diff into rows, and placing comments on them.

export interface Row {
  kind: 'hunk' | 'ctx' | 'add' | 'del' | 'note'
  text: string
  old?: number // the line in the base, for ctx and del
  new?: number // the line in the head, for ctx and add
}

const hunkRE = /^@@ -(\d+)(?:,\d+)? \+(\d+)(?:,\d+)? @@/

// parsePatch reads the hunks of a unified diff, without the ---/+++ header.
export function parsePatch(patch: string): Row[] {
  const rows: Row[] = []
  let o = 0
  let n = 0
  const lines = patch.split('\n')
  if (lines[lines.length - 1] === '') lines.pop()
  for (const line of lines) {
    const m = hunkRE.exec(line)
    if (m) {
      o = Number(m[1])
      n = Number(m[2])
      rows.push({ kind: 'hunk', text: line })
      continue
    }
    const body = line.slice(1)
    switch (line[0]) {
      case '+':
        rows.push({ kind: 'add', text: body, new: n++ })
        break
      case '-':
        rows.push({ kind: 'del', text: body, old: o++ })
        break
      case '\\': // "\ No newline at end of file"
        rows.push({ kind: 'note', text: line })
        break
      default:
        rows.push({ kind: 'ctx', text: body, old: o++, new: n++ })
    }
  }
  return rows
}

// A place on a row: the head's line, or the base's for a removed line.
export interface Place { side: 'head' | 'base'; line: number }

// rowKeys are the places a row answers to. A context line is on both sides.
export function rowKeys(r: Row): string[] {
  const keys: string[] = []
  if (r.new !== undefined) keys.push(`head:${r.new}`)
  if (r.old !== undefined) keys.push(`base:${r.old}`)
  return keys
}

export const placeKey = (p: Place) => `${p.side}:${p.line}`

// placeAll groups items by the row they belong under. An item whose place no
// row shows is returned in rest, for the file's "not on a shown line" list.
export function placeAll<T>(rows: Row[], items: T[], place: (t: T) => Place | undefined): { at: Map<number, T[]>; rest: T[] } {
  const index = new Map<string, number>()
  rows.forEach((r, i) => {
    for (const k of rowKeys(r)) if (!index.has(k)) index.set(k, i)
  })
  const at = new Map<number, T[]>()
  const rest: T[] = []
  for (const t of items) {
    const p = place(t)
    const i = p && index.get(placeKey(p))
    if (i === undefined) {
      rest.push(t)
      continue
    }
    at.set(i, [...(at.get(i) ?? []), t])
  }
  return { at, rest }
}

// patchLines counts a patch's lines, to decide whether to collapse it.
export function patchLines(patch: string): number {
  let n = 0
  for (let i = 0; i < patch.length; i++) if (patch.charCodeAt(i) === 10) n++
  return n
}
