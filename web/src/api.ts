// The client side of the loopback guard (docs/plan.md section 5.2). Every
// /api/ request carries the per-launch token in X-Vuoro-Token.

// The bindings tuohi gives vuoro's own page. A browser has none.
export interface Bindings {
  token(): Promise<string>
  version(): Promise<string>
  report(r: { ok: boolean; detail: string }): Promise<void>
  // chooseFolder shows the native folder chooser, and answers the folder
  // chosen, or '' when the person cancelled.
  chooseFolder?(title: string): Promise<string>
}

declare global {
  interface Window {
    vuoro?: Bindings
  }
}

export function bindings(w: Window = window): Bindings | undefined {
  return typeof w.vuoro?.token === 'function' ? w.vuoro : undefined
}

const storageKey = 'vuoro.token'

// tokenFrom finds the token. The window reads it from a binding. A browser
// gets it once in the URL fragment, from the URL `vuoro -browser` printed,
// keeps it in sessionStorage, which is scoped to this origin and port, and
// takes it out of the address bar.
export async function tokenFrom(w: Window): Promise<string> {
  const b = bindings(w)
  if (b) return b.token()
  const m = w.location.hash.match(/^#token=([A-Za-z0-9_-]+)$/)
  if (m) {
    w.sessionStorage.setItem(storageKey, m[1])
    w.history.replaceState(null, '', w.location.pathname + w.location.search)
  }
  const t = w.sessionStorage.getItem(storageKey)
  if (!t) throw new Error('no token: open the URL that vuoro -browser printed')
  return t
}

let token: Promise<string> | undefined

// api fetches a path under /api/ with the token, and returns its JSON.
export async function api<T>(path: string, init: RequestInit = {}): Promise<T> {
  token ??= tokenFrom(window)
  const headers = new Headers(init.headers)
  headers.set('X-Vuoro-Token', await token)
  if (init.body !== undefined) headers.set('Content-Type', 'application/json')
  const res = await fetch(path, { ...init, headers })
  if (!res.ok) {
    // The API explains a failure in {"error": "..."}; say that, not the status.
    const body = (await res.json().catch(() => undefined)) as { error?: string } | undefined
    throw new Error(body?.error ?? `${init.method ?? 'GET'} ${path}: ${res.status}`)
  }
  return res.json() as Promise<T>
}

// post sends v as JSON to a path under /api/.
export function post<T>(path: string, v: unknown = {}): Promise<T> {
  return api<T>(path, { method: 'POST', body: JSON.stringify(v) })
}
