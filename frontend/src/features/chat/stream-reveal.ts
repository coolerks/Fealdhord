import type { Element, ElementContent, Root, Text } from 'hast'

export type RevealBatch = { start: number; end: number; receivedAt: number }

const animationDuration = 160
const staggerWindow = 72
const segmenter = typeof Intl.Segmenter === 'function'
  ? new Intl.Segmenter(undefined, { granularity: 'grapheme' })
  : undefined

export function addRevealBatch(previous: readonly RevealBatch[], batch: RevealBatch): RevealBatch[] {
  return [
    ...previous.filter(item => batch.receivedAt - item.receivedAt < animationDuration + staggerWindow).slice(-7),
    batch,
  ]
}

export function revealMarkdown(source: string, batches: readonly RevealBatch[]) {
  return () => (tree: Root) => {
    function visit(parent: Root | Element) {
      for (let index = 0; index < parent.children.length; index++) {
        const child = parent.children[index]
        if (child.type === 'element' && child.tagName !== 'pre') visit(child)
        if (child.type === 'text') {
          const parts = fragments(child, source, batches)
          if (parts[0] === child) continue
          parent.children.splice(index, 1, ...parts)
          index += parts.length - 1
        }
      }
    }
    visit(tree)
  }
}

function fragments(node: Text, source: string, batches: readonly RevealBatch[]): ElementContent[] {
  const start = node.position?.start.offset
  const end = node.position?.end.offset
  if (!segmenter || start === undefined || end === undefined || end <= batches[0].start || !node.value.trim()) return [node]

  // Escapes and entities can change length. Match the literal suffix when only part of such a node is new.
  const raw = source.slice(start, end)
  let common = raw === node.value ? raw.length : 0
  while (common < raw.length && common < node.value.length &&
    raw.at(-common - 1) === node.value.at(-common - 1)) common++
  const literalStart = end - common
  const toTextIndex = (offset: number) => offset <= start ? 0
    : offset >= end ? node.value.length
      : offset >= literalStart ? node.value.length - (end - offset) : undefined
  const parts: ElementContent[] = []
  let cursor = 0
  for (const batch of batches) {
    if (batch.end <= start || batch.start >= end) continue
    const from = toTextIndex(batch.start)
    const to = toTextIndex(batch.end)
    if (from === undefined || to === undefined || to <= from) continue
    if (from > cursor) parts.push({ type: 'text', value: node.value.slice(cursor, from) })

    const value = node.value.slice(from, to)
    const graphemes = Array.from(segmenter.segment(value))
    const groupSize = Math.max(1, Math.ceil(graphemes.length / 8))
    for (let i = 0; i < graphemes.length; i += groupSize) {
      const begin = graphemes[i].index
      const next = graphemes[i + groupSize]?.index ?? value.length
      const text: Text = { type: 'text', value: value.slice(begin, next) }
      parts.push(text.value.trim() ? {
        type: 'element',
        tagName: 'span',
        properties: {
          dataChatRevealAt: batch.receivedAt + Math.min(staggerWindow, Math.max(0, start + from - batch.start) * 2 + i / groupSize * 9),
          dataChatRevealKey: start + from + begin,
        },
        children: [text],
      } : text)
    }
    cursor = to
  }
  if (parts.length === 0) return [node]
  if (cursor < node.value.length) parts.push({ type: 'text', value: node.value.slice(cursor) })
  return parts
}

export function animateReplyFragment(element: HTMLElement, revealAt: number) {
  if (!element.animate || window.matchMedia?.('(prefers-reduced-motion: reduce)').matches) return
  const elapsed = performance.now() - revealAt
  if (elapsed >= animationDuration) return
  const animation = element.animate([{ opacity: 0.2 }, { opacity: 1 }], {
    duration: animationDuration, easing: 'ease-out', fill: 'backwards',
    delay: Math.max(0, -elapsed),
  })
  if (elapsed > 0) animation.currentTime = elapsed
  return () => animation.cancel()
}
