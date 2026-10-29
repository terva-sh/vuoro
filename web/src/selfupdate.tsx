import { useEffect, useState } from 'preact/hooks'
import { errorText } from './action'
import { api, post } from './api'
import type { SelfUpdateState } from './types'

// How often the window asks the server, which asks GitHub at most once a day.
const askEvery = 60 * 60 * 1000

// SelfUpdate says in the header when a newer vuoro release exists, and puts
// it in place only when the person asks (D74). The running vuoro carries on;
// the new one runs once the person reopens it.
export function SelfUpdate() {
  const [state, setState] = useState<SelfUpdateState>()
  const [updating, setUpdating] = useState(false)
  const [error, setError] = useState<string>()

  useEffect(() => {
    // A failed question says nothing: the header is no place to report that
    // GitHub was unreachable.
    const ask = () => void api<SelfUpdateState>('/api/self-update').then(setState).catch(() => {})
    ask()
    const t = setInterval(ask, askEvery)
    return () => clearInterval(t)
  }, [])

  if (state?.applied) {
    return <span class="self-update" role="status">✓ vuoro {state.applied} is installed. Quit and reopen vuoro to use it.</span>
  }
  if (!state?.newer || !state.latest) return null

  const apply = async () => {
    setUpdating(true)
    setError(undefined)
    try {
      setState(await post<SelfUpdateState>('/api/self-update'))
    } catch (e) {
      setError(errorText(e))
    } finally {
      setUpdating(false)
    }
  }
  return (
    <span class="self-update">
      <button type="button" class="link" disabled={updating} title={`Download vuoro ${state.latest}, check it, and put it in place of ${state.current}`}
        onClick={() => void apply()}>
        {updating ? `↑ Updating to ${state.latest}…` : `↑ Update to vuoro ${state.latest}`}
      </button>
      {error && <span class="alert" role="alert">⚠ {error}</span>}
    </span>
  )
}
