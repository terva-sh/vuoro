// Running a request on the person's behalf, and saying what went wrong.
import { useEffect, useRef, useState } from 'preact/hooks'

// errorText is what an error says to the person: an Error's message, or the
// thing thrown.
export function errorText(e: unknown): string {
  return e instanceof Error ? e.message : String(e)
}

export interface Action {
  busy: boolean
  error: string | undefined
  setError: (e: string | undefined) => void
  // run clears the error, marks the action busy until fn settles, and keeps
  // what fn threw as the error. It answers with fn's result, or undefined
  // when fn threw.
  run: <T>(fn: () => Promise<T>) => Promise<T | undefined>
}

// useAction keeps the busy state and the error of a request the person
// started. Nothing is set after the component unmounts.
export function useAction(): Action {
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string>()
  const mounted = useRef(true)
  useEffect(() => () => { mounted.current = false }, [])
  const run = async <T,>(fn: () => Promise<T>): Promise<T | undefined> => {
    setBusy(true)
    setError(undefined)
    try {
      return await fn()
    } catch (e) {
      if (mounted.current) setError(errorText(e))
      return undefined
    } finally {
      if (mounted.current) setBusy(false)
    }
  }
  return { busy, error, setError, run }
}
