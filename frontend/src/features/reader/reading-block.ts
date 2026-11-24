export function readingBlock(root: HTMLElement | null, viewport: HTMLElement | null) {
  if (!root || !viewport) return
  const { top, bottom } = viewport.getBoundingClientRect()
  return Array.from(root.querySelectorAll<HTMLElement>('[data-tr-block]')).find(block => {
    const rect = block.getBoundingClientRect()
    return rect.bottom > top && rect.top < bottom
  })
}
