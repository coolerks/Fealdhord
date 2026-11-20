import hljs from 'highlight.js/lib/common'
import powershell from 'highlight.js/lib/languages/powershell'

hljs.registerLanguage('powershell', powershell)

const autoLanguages = ['javascript', 'typescript', 'python', 'go', 'bash', 'json', 'xml', 'css', 'sql', 'powershell']

export type CodeBlock = {
  target: HTMLElement
  toolbar: HTMLElement
  text: string
  hint?: string
  language?: string
}

export function highlightText(text: string, hint?: string, limit = 8_000) {
  const definition = hint ? hljs.getLanguage(hint) : undefined
  const language = definition?.name || hint
  if (text.length > limit || (hint && !definition)) return { language }

  const result = hint
    ? hljs.highlight(text, { language: hint, ignoreIllegals: true })
    : hljs.highlightAuto(text, autoLanguages)
  if (!result.language || (!hint && result.relevance < 2)) return { language }
  return { language: hljs.getLanguage(result.language)?.name || result.language, html: result.value, id: result.language }
}

export function highlightCode(blocks: CodeBlock[]) {
  let remaining = 48_000
  return blocks.map(block => {
    const { target, text, hint } = block
    if (target.querySelector(':not(span, br)')) {
      return { ...block, language: hint ? hljs.getLanguage(hint)?.name || hint : undefined }
    }
    const result = highlightText(text, hint, Math.min(8_000, remaining))
    if (text.length <= Math.min(8_000, remaining) && (!hint || hljs.getLanguage(hint))) remaining -= text.length
    if (result.html && result.id) {
      target.innerHTML = result.html
      target.classList.add(`language-${result.id}`)
      target.dataset.highlighted = 'yes'
    }
    return { ...block, language: result.language }
  })
}
