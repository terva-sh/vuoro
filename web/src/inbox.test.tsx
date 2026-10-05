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
  it('shows the row and its detail, links to the forge, and shows stale status and unknown ages', async () => {
    vi.mocked(api).mockResolvedValue(view)
    const open = vi.fn()
    await act(async () => render(<Inbox onOpen={open} />, root))
    await act(async () => { await Promise.resolve() })
    expect(root.querySelector('[aria-label="Your turn"]')?.textContent).toContain('Your review is requested.')
    expect(root.textContent).toContain('No successful refresh yet')
    expect(root.textContent).toContain('rate limited')
    // The first row is selected, and the pane carries its forge link.
    const pane = root.querySelector('[aria-label="Selected change"]')!
    expect(pane.textContent).toContain('Review the change')
    expect(pane.querySelector('a')?.href).toBe(view.mine[0].web_url)
    act(() => button('Open in vuoro').click())
    expect(open).toHaveBeenCalledWith(view.mine[0].web_url)
    // Their turn is folded until asked for, and its ages may be unknown.
    const theirs = root.querySelector('details[aria-label="Their turn"]') as HTMLDetailsElement
    expect(theirs.open).toBe(false)
    await act(async () => { theirs.open = true; theirs.dispatchEvent(new Event('toggle')) })
    act(() => (theirs.querySelector('.inbox-row') as HTMLButtonElement).click())
    expect(root.querySelector('[aria-label="Selected change"]')!.textContent).toContain('Age unknown')
    expect(root.querySelector('[aria-label="Selected change"] a')?.getAttribute('href')).toBe(view.theirs[0].web_url)
  })
  it('snoozes until it changes and restores a row with serialized actions', async () => {
    vi.mocked(api).mockResolvedValue(view)
    await act(async () => render(<Inbox onOpen={() => {}} />, root))
    await act(async () => { await Promise.resolve() })
    const sleeping = { ...view, mine: [], snoozed: [{ ...view.mine[0], snoozed_until: '9999-12-31T00:00:00Z' }] }
    vi.mocked(api).mockResolvedValueOnce(sleeping)
    await act(async () => button('Until it changes').click())
    expect(api).toHaveBeenLastCalledWith('/api/inbox/snooze', expect.objectContaining({ method: 'POST', body: expect.stringContaining('9999-12-31') }))
    const snoozed = root.querySelector('details[aria-label="Snoozed"]') as HTMLDetailsElement
    expect(snoozed.textContent).toContain('Snoozed 1')
    await act(async () => { snoozed.open = true; snoozed.dispatchEvent(new Event('toggle')) })
    act(() => (snoozed.querySelector('.inbox-row') as HTMLButtonElement).click())
    expect(root.textContent).toContain('Snoozed until it changes.')
    vi.mocked(api).mockResolvedValueOnce(view)
    await act(async () => button('Restore').click())
    expect(api).toHaveBeenLastCalledWith('/api/inbox/snooze', expect.objectContaining({ body: expect.stringContaining('0001-01-01T00:00:00Z') }))
    expect(root.querySelector('details[aria-label="Snoozed"]')).toBeNull()
  })
  it('sorts your turn into lanes under a summary that filters, and moves by keyboard', async () => {
    const row = view.mine[0]
    const reply = { ...row, ref: { ...row.ref, number: 3 }, title: 'Answer them', roles: ['author'], lane: 'replies' as const,
      turn: { ...row.turn, reasons: [{ rule: 'A1', text: 'alice is waiting', since: null, who: 'alice' }, { rule: 'A1', text: 'bo is waiting', since: null, who: 'bo' }] },
      summary: ['2 threads await your answer: alice, bo.'],
      waiting: [{ rule: 'A1', who: 'alice', where: 'a.go:3', text: 'Why this?', since: null }, { rule: 'A1', who: 'bo', where: 'general', text: 'And this?', since: null }] }
    const merge = { ...row, ref: { ...row.ref, number: 4 }, title: 'Merge it', roles: ['author'], lane: 'yours' as const,
      turn: { ...row.turn, reasons: [{ rule: 'A3', text: 'Approved and mergeable; you can merge this change.', since: null }] } }
    vi.mocked(api).mockResolvedValue({ ...view, mine: [{ ...row, lane: 'reviews' }, reply, merge] })
    const open = vi.fn()
    await act(async () => render(<Inbox onOpen={open} />, root))
    await act(async () => { await Promise.resolve() })
    const lanes = [...root.querySelectorAll('.inbox-lane')].map((l) => l.getAttribute('aria-label'))
    expect(lanes).toEqual(['Your changes', 'Replies owed', 'Reviews'])
    const strip = root.querySelector('[aria-label="What needs you"]')!.textContent!
    expect(strip).toContain('1 ready to merge')
    expect(strip).toContain('1 replies owed')
    expect(strip).toContain('0 changes requested on yours')
    // The pane opens on the first row, the merge; j moves to the replies.
    expect(root.querySelector('[aria-label="Selected change"] h2')!.textContent).toBe('Merge it')
    act(() => { window.dispatchEvent(new KeyboardEvent('keydown', { key: 'j' })) })
    const pane = root.querySelector('[aria-label="Selected change"]')!
    expect(pane.textContent).toContain('2 threads await your answer: alice, bo.')
    expect(pane.textContent).toContain('a.go:3')
    expect(pane.textContent).toContain('Why this?')
    act(() => { window.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter' })) })
    expect(open).toHaveBeenCalledWith(reply.web_url)
    // A count filters the list to its kind.
    act(() => { [...root.querySelectorAll('.chip')].find((b) => b.textContent!.includes('reviews requested'))!.dispatchEvent(new MouseEvent('click', { bubbles: true })) })
    expect([...root.querySelectorAll('.inbox-lane')].map((l) => l.getAttribute('aria-label'))).toEqual(['Reviews'])
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
  it('shows a partial refresh: the account warning, a stale row, and a change it could not read', async () => {
    const partial: InboxView = {
      ...view,
      accounts: [{ ...view.accounts[0], warning: '2 changes could not be read; they show their last known state.' }],
      mine: [{ ...view.mine[0], error: 'The account may not read this change.' }],
      theirs: [],
      unread: [{ ...view.mine[0], ref: { host: 'gitlab.example', project: 'team/other', number: 9 }, title: '', web_url: '', roles: ['author'], error: 'The forge\'s answer for this change could not be read.' }],
    }
    vi.mocked(api).mockResolvedValue(partial)
    await act(async () => render(<Inbox onOpen={() => {}} />, root))
    await act(async () => { await Promise.resolve() })
    expect(root.textContent).toContain('2 changes could not be read')
    expect(root.textContent).not.toContain('Cached results')
    expect(root.querySelector('[aria-label="Your turn"]')?.textContent).toContain('Last known state.')
    expect(root.querySelector('[aria-label="Selected change"]')?.textContent).toContain('Last known state. The account may not read this change.')
    const unread = root.querySelector('[aria-label="Could not read"]')!
    expect(unread.textContent).toContain('team/other!9')
    expect(unread.textContent).toContain('could not be read')
    expect(unread.querySelectorAll('a, button').length).toBe(0)
  })
})
