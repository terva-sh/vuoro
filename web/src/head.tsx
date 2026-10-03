import { createContext } from 'preact'
import { useContext, useEffect, useState } from 'preact/hooks'
import { api } from './api'
import { short } from './ui'
import type { Change } from './types'

export interface HeadState {
  reviewed_head: string; current_head: string
  relation: 'same' | 'ahead' | 'rewritten' | 'unavailable' | 'checking' | 'outdated'
  commits_behind?: number
}
export const HeadContext = createContext<HeadState | undefined>(undefined)
export const useHead = () => useContext(HeadContext)

// A response belongs only to its exact run and pair of commits. Comparing
// history is independent of agent polling and never starts a review.
export function useRunHead(id: string | undefined, reviewed: string | undefined, change: Change): HeadState | undefined {
  const [result, setResult] = useState<{ id: string; state: HeadState }>()
  const current = change.diff.head
  const url = change.web_url
  useEffect(() => {
    if (!id || !reviewed || reviewed === current) return
    const controller = new AbortController()
    api<HeadState>(`/api/runs/${id}/head`, { method: 'POST', body: JSON.stringify({ url }), signal: controller.signal })
      .then((state) => {
        if (controller.signal.aborted) return
        // A fresh forge read may already be ahead of this workspace snapshot.
        // Finish the request visibly without attaching its count to our diff.
        setResult({ id, state: state.reviewed_head === reviewed && state.current_head === current ? state
          : { reviewed_head: reviewed, current_head: current, relation: 'outdated' } })
      })
      .catch(() => { if (!controller.signal.aborted) setResult({ id, state: { reviewed_head: reviewed, current_head: current, relation: 'unavailable' } }) })
    return () => controller.abort()
  }, [id, reviewed, current, url])
  if (!id || !reviewed) return undefined
  if (reviewed === current) return { reviewed_head: reviewed, current_head: current, relation: 'same' }
  if (result?.id === id && result.state.reviewed_head === reviewed && result.state.current_head === current) return result.state
  return { reviewed_head: reviewed, current_head: current, relation: 'checking' }
}

export function HeadBadge({ state, label = 'reviewed' }: { state?: HeadState; label?: string }) {
  if (!state) return null
  const suffix = state.relation === 'ahead' && state.commits_behind !== undefined ? `${state.commits_behind} commit${state.commits_behind === 1 ? '' : 's'} behind`
    : state.relation === 'rewritten' ? 'history rewritten'
    : state.relation === 'checking' ? 'head changed · checking commit distance'
    : state.relation === 'outdated' ? 'change advanced again · refresh to compare'
    : state.relation === 'unavailable' ? 'head changed · commit distance unavailable' : ''
  return <span class={`badge head-badge ${state.reviewed_head !== state.current_head ? 'warn' : ''}`}>{state.reviewed_head !== state.current_head && '⚠ '}{label} at {short(state.reviewed_head)}{suffix && `, ${suffix}`}</span>
}
