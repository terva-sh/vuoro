// Getting started (docs/plan.md section 3.4): hints on empty and first-time
// screens, and the tour. Their only writes are the person's own progress,
// in prefs.json.
import { createContext, type ComponentChildren } from 'preact'
import { useContext, useMemo } from 'preact/hooks'
import { post } from './api'
import type { HintID, Setup, Tour } from './types'

export interface Guide {
  hints: HintID[] // the hints the person dismissed
  dismiss: (h: HintID) => void
  tour: Tour
  setTour: (t: Tour) => void
}

// Without a provider every hint shows and nothing is kept, which is what a
// fresh start looks like.
export const GuideContext = createContext<Guide>({ hints: [], dismiss: () => undefined, tour: { state: '' }, setTour: () => undefined })

export const useGuide = () => useContext(GuideContext)

// allHints are every hint, which app.Hints lists too. Before setup says
// which were dismissed, none shows, so a dismissed one does not flash.
export const allHints: HintID[] = ['reviews-no-forge', 'reviews-quiet', 'local-empty', 'projects-empty', 'change-no-runs', 'first-drafts']

// useGuideState keeps the person's progress in setup, and writes each step
// of it to prefs.json as it happens. A write that fails loses only a hint
// shown again or a tour step repeated, so it is not reported.
export function useGuideState(setup: Setup | undefined, onSetup: (f: (s: Setup | undefined) => Setup | undefined) => void): Guide {
  const hints = setup?.hints
  const tour = setup?.tour
  return useMemo(() => ({
    hints: setup ? hints ?? [] : allHints,
    dismiss: (h: HintID) => {
      onSetup((s) => s && { ...s, hints: [...(s.hints ?? []).filter((x) => x !== h), h] })
      post('/api/prefs/hint', { hint: h, dismissed: true }).catch(() => undefined)
    },
    tour: tour ?? { state: '' },
    setTour: (t: Tour) => {
      onSetup((s) => s && { ...s, tour: t })
      post('/api/prefs/tour', t).catch(() => undefined)
    },
  }), [!!setup, hints, tour])
}

// Hint says what to do next on an empty or first-time screen, until the
// person dismisses it for good. fallback shows in its place once it is
// dismissed, for a screen that would otherwise say nothing.
export function Hint({ id, children, fallback }: { id: HintID; children: ComponentChildren; fallback?: ComponentChildren }) {
  const { hints, dismiss } = useGuide()
  if (hints.includes(id)) return <>{fallback}</>
  return (
    <aside class="hint" role="note" aria-label="Getting started" data-hint={id}>
      <div class="hint-body">{children}</div>
      <button type="button" class="link" onClick={() => dismiss(id)}>Got it</button>
    </aside>
  )
}

// toolNames names the agent tools vuoro knows, for a hint that needs one
// none of which answered.
export function toolNames(setup?: Setup): string {
  const names = [...new Set(setup?.tools.map((t) => t.tool) ?? [])]
  return names.length > 0 ? names.join(' or ') : 'Claude Code or terva'
}

// NeedsTool is the sentence a hint gives when its screen needs an agent
// tool and none answered, so it names the tool rather than a control that
// would fail.
export function NeedsTool({ setup, doing }: { setup?: Setup; doing: string }) {
  return <p>{doing} needs {toolNames(setup)} installed and signed in. Settings, under Agent tools, says what vuoro found and checks again.</p>
}
