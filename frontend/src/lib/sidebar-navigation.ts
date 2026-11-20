import type { KeyboardEvent } from 'react'

export function moveSidebarFocus(event: KeyboardEvent<HTMLElement>) {
  const buttons = Array.from(event.currentTarget.querySelectorAll<HTMLButtonElement>('button[data-navigation]:not(:disabled)'))
    .filter(button => !button.closest('[inert]'))
  const index = buttons.findIndex(button => button === document.activeElement)
  if (index < 0) return

  let next: number
  switch (event.key) {
    case 'ArrowDown': next = (index + 1) % buttons.length; break
    case 'ArrowUp': next = (index - 1 + buttons.length) % buttons.length; break
    case 'Home': next = 0; break
    case 'End': next = buttons.length - 1; break
    default: return
  }
  event.preventDefault()
  buttons[next]?.focus()
}
