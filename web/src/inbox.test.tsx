// @vitest-environment jsdom
import { render } from 'preact'
import { act } from 'preact/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { api } from './api'
import { Inbox, type InboxView } from './inbox'

vi.mock('./api', () => ({ api: vi.fn() }))
const view: InboxView = {
  accounts: [{ key: 'gl', name: 'Work', kind: 'gitlab', host: 'gitlab.example', username: 'me', last_attempt: '2026-10-02T00:00:00Z', last_success: '2026-10-02T00:00:00Z' }, { key: 'fj', name: 'Internal', kind: 'forgejo', host: 'forge.example', username: 'me', last_attempt: '', last_success: '', error: 'The forge rate limited refresh; try again later.' }],
  mine: [{ ref: { host: 'gitlab.example', project: 'team/repo', number: 1 }, kind: 'gitlab', account_key: 'gl', account: 'Work', title: 'Review the change', web_url: 'https://gitlab.example/team/repo/-/merge_requests/1', draft: false, roles: ['reviewer'], turn: { owner: 'mine', since: '2026-10-01T00:00:00Z', last_activity: null, reasons: [{ rule: 'R1', text: 'Your review is requested.', since: '2026-10-01T00:00:00Z' }] } }],
  theirs: [{ ref: { host: 'forge.example', project: 'team/repo', number: 2 }, kind: 'forgejo', account_key: 'fj', account: 'Internal', title: 'Waiting for an answer', web_url: 'https://forge.example/team/repo/pulls/2', draft: false, roles: ['author'], turn: { owner: 'theirs', since: null, last_activity: null, reasons: [{ rule: 'waiting', text: 'Waiting for review from reviewer.', since: null }] } }],
  snoozed: [],
}
const root = document.createElement('div')
const button = (name: string) => [...root.querySelectorAll('button')].find((b) => b.textContent === name)!
afterEach(() => { act(() => render(null, root)); vi.useRealTimers(); vi.resetAllMocks() })

describe('the combined inbox', () => {
  it('opens the workspace, links to both forges and shows stale status and unknown ages', async () => {
    vi.mocked(api).mockResolvedValue(view)
    const open = vi.fn()
    await act(async () => render(<Inbox onOpen={open} />, root))
    await act(async () => { await Promise.resolve() })
    expect(root.querySelector('[aria-label="Your turn"]')?.textContent).toContain('Your review is requested.')
    expect(root.querySelector('[aria-label="Their turn"]')?.textContent).toContain('Age unknown')
    expect(root.textContent).toContain('No successful refresh yet')
    expect(root.textContent).toContain('rate limited')
    expect(root.querySelectorAll('a').length).toBe(2)
    expect(root.querySelector('a')?.href).toBe(view.mine[0].web_url)
    act(() => button('Review the change').click())
    expect(open).toHaveBeenCalledWith(view.mine[0].web_url)
  })
  it('snoozes and restores a row with serialized actions', async () => {
    vi.mocked(api).mockResolvedValue(view)
    await act(async () => render(<Inbox onOpen={() => {}} />, root))
    await act(async () => { await Promise.resolve() })
    const sleeping = { ...view, mine: [], snoozed: [{ ...view.mine[0], snoozed_until: '2099-01-01T00:00:00Z' }] }
    vi.mocked(api).mockResolvedValueOnce(sleeping)
    await act(async () => button('Snooze 1 day').click())
    expect(api).toHaveBeenLastCalledWith('/api/inbox/snooze', expect.objectContaining({ method: 'POST', body: expect.stringContaining('"number":1') }))
    expect(root.textContent).toContain('Snoozed (1)')
    vi.mocked(api).mockResolvedValueOnce(view)
    await act(async () => button('Restore').click())
    expect(api).toHaveBeenLastCalledWith('/api/inbox/snooze', expect.objectContaining({ body: expect.stringContaining('0001-01-01T00:00:00Z') }))
    expect(root.textContent).not.toContain('Snoozed (1)')
  })
  it('never overlaps polling and aborts the active request on unmount', async () => {
    vi.useFakeTimers()
    let finish!: (value: InboxView) => void
    vi.mocked(api).mockResolvedValueOnce(view).mockImplementationOnce(() => new Promise((resolve) => { finish = resolve as typeof finish }))
    await act(async () => render(<Inbox onOpen={() => {}} />, root))
    await act(async () => { await Promise.resolve() })
    expect(api).toHaveBeenCalledTimes(2)
    expect(button('Refreshing…').disabled).toBe(true)
    await act(async () => { vi.advanceTimersByTime(180_000) })
    expect(api).toHaveBeenCalledTimes(2)
    await act(async () => finish(view))
    let signal: AbortSignal | null | undefined
    vi.mocked(api).mockImplementationOnce((_path, init) => { signal = init?.signal; return new Promise(() => {}) })
    await act(async () => { vi.advanceTimersByTime(60_000) })
    expect(api).toHaveBeenCalledTimes(3)
    act(() => render(null, root))
    expect(signal?.aborted).toBe(true)
    await act(async () => { vi.advanceTimersByTime(180_000) })
    expect(api).toHaveBeenCalledTimes(3)
  })
  it('keeps cached rows when transport refresh fails', async () => {
    vi.mocked(api).mockResolvedValueOnce(view).mockRejectedValueOnce(new Error('Refresh unavailable'))
    await act(async () => render(<Inbox onOpen={() => {}} />, root))
    await act(async () => { await Promise.resolve() })
    expect(root.textContent).toContain('Review the change')
    expect(root.textContent).toContain('Refresh unavailable')
    expect(button('Refresh inbox').disabled).toBe(false)
  })
})
