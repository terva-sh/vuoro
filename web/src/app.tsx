import { useEffect, useState } from 'preact/hooks'
import { api, bindings } from './api'

interface Hello {
  version: string
}

// App is vuoro's window. For now it proves the page reached the server and,
// in the window, Go; the change view replaces it.
export function App() {
  const [hello, setHello] = useState<Hello>()
  const [error, setError] = useState<string>()

  useEffect(() => {
    const b = bindings()
    api<Hello>('/api/hello')
      .then(async (h) => {
        setHello(h)
        const detail = b ? `server=${JSON.stringify(h.version)} binding=${JSON.stringify(await b.version())}` : ''
        await b?.report({ ok: true, detail })
      })
      .catch(async (e: unknown) => {
        setError(String(e))
        await b?.report({ ok: false, detail: String(e) })
      })
  }, [])

  return (
    <main>
      <h1>vuoro</h1>
      {error && <p role="alert">{error}</p>}
      {hello && <p>vuoro {hello.version}</p>}
    </main>
  )
}
