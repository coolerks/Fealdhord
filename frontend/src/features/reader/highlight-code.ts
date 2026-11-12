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

export function highlightCode(blocks: CodeBlock[]) {
  let remaining = 48_000
  return blocks.map(block => {
    const { target, text, hint } = block
    const definition = hint ? hljs.getLanguage(hint) : undefined
    const language = definition?.name || hint
    if (target.querySelector(':not(span, br)') || text.length > Math.min(8_000, remaining) || (hint && !definition)) {
      return { ...block, language }
    }

    remaining -= text.length
    const result = hint
      ? hljs.highlight(text, { language: hint, ignoreIllegals: true })
      : hljs.highlightAuto(text, autoLanguages)
    if (!result.language || (!hint && result.relevance < 2)) return { ...block, language }
    target.innerHTML = result.value
    target.classList.add(`language-${result.language}`)
    target.dataset.highlighted = 'yes'
    return { ...block, language: hljs.getLanguage(result.language)?.name || result.language }
  })
}
