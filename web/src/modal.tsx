// A dialog over the page. It names itself by its title, puts the focus where
// the caller says, and closes on Escape or a click outside, unless locked.
import type { ComponentChildren, RefObject } from 'preact'
import { useEffect, useId, useRef } from 'preact/hooks'

export function Modal({ title, onClose, focus, locked = false, class: cls = '', children }: {
  title: ComponentChildren
  onClose: () => void
  focus?: RefObject<HTMLElement> // what has the focus when it opens, such as Cancel
  locked?: boolean // while something it started runs, Escape and the backdrop do nothing
  class?: string
  children: ComponentChildren
}) {
  const id = useId()
  // The latest onClose, so the key listener need not follow every render.
  const close = useRef(onClose)
  close.current = onClose
  useEffect(() => { focus?.current?.focus() }, [])
  useEffect(() => {
    if (locked) return
    const onKey = (e: KeyboardEvent) => e.key === 'Escape' && close.current()
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [locked])
  return (
    <div class="modal-back" onClick={(e) => e.target === e.currentTarget && !locked && onClose()}>
      <div class={`modal ${cls}`.trim()} role="dialog" aria-modal="true" aria-labelledby={id}>
        <h2 id={id}>{title}</h2>
        {children}
      </div>
    </div>
  )
}
