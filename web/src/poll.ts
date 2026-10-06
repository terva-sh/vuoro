// Asking the server again and again while a view is open.
import { useEffect, useRef } from 'preact/hooks'

// usePoll calls tick at once, and again each time the wait that tick
// answers has passed after it settled, so two ticks never overlap. An
// undefined answer, or a throw, stops the poll. A change in deps, or
// unmounting, stops it and aborts the signal tick was given, which a tick
// also reads to tell whether it may still set state. The latest tick runs
// each time, so it sees the latest render.
export function usePoll(tick: (signal: AbortSignal) => Promise<number | undefined>, deps: unknown[]) {
  const latest = useRef(tick)
  latest.current = tick
  useEffect(() => {
    const controller = new AbortController()
    let timer: ReturnType<typeof setTimeout> | undefined
    const loop = async () => {
      let wait: number | undefined
      try {
        wait = await latest.current(controller.signal)
      } catch {
        wait = undefined
      }
      if (wait !== undefined && !controller.signal.aborted) timer = setTimeout(() => void loop(), wait)
    }
    void loop()
    return () => {
      controller.abort()
      clearTimeout(timer)
    }
  }, deps)
}
