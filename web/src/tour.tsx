// The getting-started tour (docs/plan.md section 3.4): a popover on each of
// the window's main surfaces in turn, anchored to a data-tour marker on the
// real element. It only shows. It never starts a run, opens a change, or
// writes to a forge, and its one write is the person's progress.
import { useEffect, useLayoutEffect, useRef, useState } from 'preact/hooks'
import { keepKeys, useGuide } from './guide'

export type Surface = 'home' | 'change'

export interface TourStep { id: string; surface: Surface; title: string; text: string }

// tourSteps are the tour, in order; app.TourSteps lists the same IDs. The
// change part waits for a change the person opened themselves.
export const tourSteps: TourStep[] = [
  { id: 'home-tabs', surface: 'home', title: 'Three places to start',
    text: 'Reviews is your inbox across every forge, ordered by whose turn it is. Local is your own branches, reviewed before you push. Projects is every open change in a project you follow.' },
  { id: 'open-url', surface: 'home', title: 'Open any change',
    text: 'Paste a merge request or pull request URL here to open it, whether or not it is in your inbox. In the inbox, / jumps here.' },
  { id: 'settings', surface: 'home', title: 'Settings',
    text: 'Your forges, agent tools, presets, and this tour live here. Open a change when you are ready, and the tour picks up there.' },
  { id: 'change-tabs', surface: 'change', title: 'A change',
    text: 'The description and discussion, the diff with the threads on each line, and the review each have a tab here.' },
  { id: 'review-tab', surface: 'change', title: 'Reviewing with agents',
    text: 'On the Review tab you pick a preset and start a review, which costs money on your agent tool. Its reviewers read the change in parallel, and you can talk to each one between its turns. What they find comes back as drafts for you to keep, edit, or drop.' },
  { id: 'submit', surface: 'change', title: 'Submit is yours',
    text: 'Nothing an agent writes reaches the forge on its own. Your kept drafts wait here as pending comments, and they are posted only when you press Submit.' },
]

// shown finds a step's element, if it is on the screen. An element inside
// a hidden tab is not.
function shown(id: string): HTMLElement | undefined {
  const el = document.querySelector<HTMLElement>(`[data-tour="${id}"]`)
  return el && !el.closest('[hidden]') ? el : undefined
}

// nextStep is the step to show on this surface after the last one passed.
// A step of this surface whose element is absent is skipped, and steps of
// another surface wait for it.
export function nextStep(surface: Surface, passed?: string): { step: TourStep; el: HTMLElement } | undefined {
  const from = passed ? tourSteps.findIndex((s) => s.id === passed) + 1 : 0
  for (const step of tourSteps.slice(from)) {
    if (step.surface !== surface) continue
    const el = shown(step.id)
    if (el) return { step, el }
  }
  return undefined
}

interface Place { top: number; left: number; above: boolean }

// place puts the popover under its element, or above it where there is no
// room below, and inside the window.
function place(el: HTMLElement): Place {
  const r = el.getBoundingClientRect()
  const width = Math.min(360, window.innerWidth - 16)
  const left = Math.round(Math.max(8, Math.min(r.left, window.innerWidth - width - 8)))
  const above = r.bottom + 220 > window.innerHeight && r.top > 220
  return { top: Math.round(above ? r.top - 8 : r.bottom + 8), left, above }
}

// Tour shows the tour's next step on this surface while the tour is on. It
// looks again as the page changes, since an element appears once what it
// shows has loaded.
export function Tour({ surface }: { surface: Surface }) {
  const { tour, setTour } = useGuide()
  const on = tour.state === 'on'
  const [at, setAt] = useState<{ step: TourStep; el: HTMLElement; place: Place }>()
  useEffect(() => {
    if (!on) {
      setAt(undefined)
      return
    }
    const look = () => {
      const found = nextStep(surface, tour.step)
      setAt((cur) => {
        if (!found) return undefined
        const p = place(found.el)
        if (cur && cur.step === found.step && cur.el === found.el && cur.place.top === p.top && cur.place.left === p.left) return cur
        return { ...found, place: p }
      })
    }
    look()
    const timer = setInterval(look, 300)
    return () => clearInterval(timer)
  }, [on, surface, tour.step])

  // The element the step is about is outlined while it shows.
  useEffect(() => {
    if (!at) return
    at.el.classList.add('tour-target')
    at.el.scrollIntoView?.({ block: 'nearest' })
    return () => at.el.classList.remove('tour-target')
  }, [at?.el])

  const next = useRef<HTMLButtonElement>(null)
  useLayoutEffect(() => { next.current?.focus() }, [at?.step])

  const end = () => at && setTour({ state: 'off', step: at.step.id })
  useEffect(() => {
    if (!at) return
    const onKey = (e: KeyboardEvent) => {
      // A key pressed in the popover is the tour's alone. The inbox and
      // project lists hear keys on the window, and Enter on Next would
      // otherwise open their selected change. The button still gets its
      // own Enter and Space, since nothing prevents the default.
      const inTour = e.target instanceof Node && !!(e.target as Element).closest?.('.tour')
      if (inTour) e.stopPropagation()
      // Esc ends the tour at any step, unless a modal is open, which
      // closes first.
      if (e.key !== 'Escape' || document.querySelector('[aria-modal="true"]')) return
      e.stopPropagation()
      end()
    }
    window.addEventListener('keydown', onKey, true)
    return () => window.removeEventListener('keydown', onKey, true)
  }, [at?.step])

  if (!at) return null
  const i = tourSteps.indexOf(at.step)
  // The tour is over when nothing after this step can still show: no step
  // of another surface, and none here with its element on the screen.
  const last = !tourSteps.slice(i + 1).some((s) => s.surface !== surface) && !nextStep(surface, at.step.id)
  return (
    <div class={at.place.above ? 'tour above' : 'tour'} role="dialog" aria-label="Tour of vuoro" aria-describedby="tour-text"
      style={{ top: `${at.place.top}px`, left: `${at.place.left}px` }} data-step={at.step.id}>
      <p class="tour-count muted">Tour · {i + 1} of {tourSteps.length}</p>
      <h2>{at.step.title}</h2>
      <p id="tour-text">{at.step.text}</p>
      <div class="actions">
        <button type="button" class="link" onClick={end}>End the tour</button>
        <span class="spacer" />
        <button type="button" class="primary" ref={next}
          onClick={() => setTour({ state: last ? 'done' : 'on', step: at.step.id })}>{last ? 'Finish' : 'Next'}</button>
      </div>
    </div>
  )
}

// TourOffer asks whether the person wants the tour, until they answer.
// Either answer is kept, so it is not asked again; Settings replays it.
export function TourOffer() {
  const { tour, setTour } = useGuide()
  if (tour.state !== '') return null
  return (
    <aside class="hint tour-offer" role="note" aria-label="Tour of vuoro" onKeyDown={keepKeys}>
      <div class="hint-body">
        <p><strong>New to vuoro?</strong> A short tour shows where things are in this window. It only points at them: nothing starts, opens, or posts.</p>
      </div>
      <button type="button" class="primary" onClick={() => setTour({ state: 'on' })}>Show me around</button>
      <button type="button" onClick={() => setTour({ state: 'off' })}>No thanks</button>
    </aside>
  )
}
