// @vitest-environment jsdom
import { render } from 'preact'
import { act } from 'preact/test-utils'
import { useRef } from 'preact/hooks'
import { describe, expect, it } from 'vitest'
import { Modal } from './modal'

function open(locked = false) {
  let closed = 0
  const root = document.createElement('div')
  document.body.append(root)
  const Probe = () => {
    const cancel = useRef<HTMLButtonElement>(null)
    return (
      <Modal title="Push the branch" focus={cancel} locked={locked} onClose={() => closed++}>
        <button>Push</button><button ref={cancel}>Cancel</button>
      </Modal>
    )
  }
  act(() => render(<Probe />, root))
  const escape = () => act(() => { window.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' })) })
  return { root, closed: () => closed, escape }
}

describe('Modal', () => {
  it('names itself by its title and focuses what the caller asks', () => {
    const m = open()
    const dialog = m.root.querySelector('[role=dialog]')!
    expect(document.getElementById(dialog.getAttribute('aria-labelledby')!)?.textContent).toBe('Push the branch')
    expect(document.activeElement?.textContent).toBe('Cancel')
  })

  it('closes on Escape and on a click outside, but not on a click inside', () => {
    const m = open()
    m.escape()
    expect(m.closed()).toBe(1)
    act(() => (m.root.querySelector('.modal-back') as HTMLElement).click())
    expect(m.closed()).toBe(2)
    act(() => (m.root.querySelector('.modal') as HTMLElement).click())
    expect(m.closed()).toBe(2)
  })

  it('holds while locked', () => {
    const m = open(true)
    m.escape()
    act(() => (m.root.querySelector('.modal-back') as HTMLElement).click())
    expect(m.closed()).toBe(0)
  })
})
