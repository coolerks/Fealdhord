import hljs from 'highlight.js/lib/common'

const autoLanguages = ['javascript', 'typescript', 'python', 'go', 'bash', 'json', 'xml', 'css', 'sql']

export function highlightCode(blocks: NodeListOf<HTMLElement>) {
  let remaining = 48_000
  for (const block of blocks) {
    if (block.dataset.highlighted === 'yes' || block.querySelector(':not(span)')) continue
    const source = block.textContent ?? ''
    if (!source.trim() || source.length > Math.min(8_000, remaining)) continue
    const hint = [block.className, block.parentElement?.className]
      .join(' ').match(/(?:^|\s)(?:language|lang)-([\w+-]+)/)?.[1].toLowerCase()
    if (hint && !hljs.getLanguage(hint)) continue
    remaining -= source.length
    const result = hint
      ? hljs.highlight(source, { language: hint, ignoreIllegals: true })
      : hljs.highlightAuto(source, autoLanguages)
    if (!result.language || (!hint && result.relevance < 2)) continue
    block.innerHTML = result.value
    block.dataset.highlighted = 'yes'
  }
}
