import { describe, expect, it } from 'vitest'
import { tokenFrom } from './api'

// fakeWindow is the part of Window tokenFrom reads.
function fakeWindow(hash: string, binding?: string) {
  const store = new Map<string, string>()
  const w = {
    location: { hash, pathname: '/', search: '' },
    sessionStorage: {
      getItem: (k: string) => store.get(k) ?? null,
      setItem: (k: string, v: string) => void store.set(k, v),
    },
    history: { replaceState: (_s: unknown, _t: string, url: string) => { w.location.hash = ''; w.replaced = url } },
    replaced: '',
    vuoro: binding === undefined ? undefined : { token: async () => binding, version: async () => 'dev', report: async () => {} },
  }
  return w
}

describe('tokenFrom', () => {
  it('reads the binding in the window', async () => {
    expect(await tokenFrom(fakeWindow('', 'from-binding') as unknown as Window)).toBe('from-binding')
  })

  it('takes the token from the fragment once, and out of the address bar', async () => {
    const w = fakeWindow('#token=abc_DEF-123')
    expect(await tokenFrom(w as unknown as Window)).toBe('abc_DEF-123')
    expect(w.replaced).toBe('/')
    expect(w.location.hash).toBe('')
    // A reload finds it in sessionStorage.
    expect(await tokenFrom(w as unknown as Window)).toBe('abc_DEF-123')
  })

  it('refuses without a token', async () => {
    await expect(tokenFrom(fakeWindow('') as unknown as Window)).rejects.toThrow(/no token/)
  })

  it('ignores a fragment that is not a token', async () => {
    await expect(tokenFrom(fakeWindow('#token=<script>') as unknown as Window)).rejects.toThrow(/no token/)
  })
})
