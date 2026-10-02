import { describe, expect, it } from 'vitest'
import { shownDrafts, sourceLabel } from './run'
import type { Draft, Run } from './types'

const draft = (id: string, session: string, sources?: string[]): Draft => ({
  id, session, sources, severity: 'low', title: id, description: '', status: 'pending',
  revision: 1, drafted_by: 'x', updated: '',
})

describe('merged drafts', () => {
  const d1 = draft('d1', 's1'), d2 = draft('d2', 's2'), d3 = draft('d3', 's2')
  const byID = new Map([d1, d2, d3].map((d) => [d.id, d]))
  const profile = new Map([['s1', 'code'], ['s2', 'documentation']])

  it('names every reviewer a merge came from', () => {
    expect(sourceLabel(draft('d4', 's3', ['d1', 'd2']), byID, profile)).toBe('merged from d1, d2 (code, documentation)')
  })
  it('labels a finding only one reviewer raised', () => {
    expect(sourceLabel(draft('d5', 's3', ['d3']), byID, profile)).toBe('◇ raised by one reviewer, documentation, from d3')
  })
  it('shows the synthesizer’s drafts once there are any', () => {
    const run = { sessions: [{ id: 's1', role: 'reviewer' }, { id: 's3', role: 'synthesizer' }], drafts: [d1, draft('d4', 's3', ['d1'])] } as unknown as Run
    expect(shownDrafts(run).map((d) => d.id)).toEqual(['d4'])
    expect(shownDrafts({ ...run, drafts: [d1] }).map((d) => d.id)).toEqual(['d1'])
  })
})
