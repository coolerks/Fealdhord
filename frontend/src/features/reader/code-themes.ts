import { useEffect, useState } from 'react'

export const defaultCodeTheme = 'app'

const styles = import.meta.glob<string>([
  '/node_modules/highlight.js/styles/**/*.css',
  '!/node_modules/highlight.js/styles/**/*.min.css',
], { query: '?inline', import: 'default', exhaustive: true })

const themes = new Map(Object.entries(styles).map(([path, load]) => [
  path.split('/highlight.js/styles/')[1].slice(0, -4), load,
]))

export function availableCodeTheme(theme: string) {
  return themes.has(theme) ? theme : defaultCodeTheme
}

export const codeThemeOptions = [
  { value: defaultCodeTheme, label: '跟随应用' },
  ...Array.from(themes.keys()).sort().map(value => {
    const name = (value.split('/').at(-1) ?? value).replace(/(^|-)([a-z])/g, (_, boundary: string, letter: string) =>
      `${boundary ? ' ' : ''}${letter.toUpperCase()}`)
    return { value, label: value.startsWith('base16/') ? `Base16 · ${name}` : name, keywords: [value] }
  }),
]

function scopeRules(rules: CSSRuleList, scope: string) {
  for (const rule of rules) {
    if (rule instanceof CSSStyleRule) {
      rule.selectorText = rule.selectorText.split(',').map(selector => `${scope} ${selector.trim()}`).join(', ')
    } else if (rule instanceof CSSMediaRule) {
      scopeRules(rule.cssRules, scope)
    }
  }
}

export function useReaderCodeTheme(theme: string, hasBlocks: boolean) {
  const [loaded, setLoaded] = useState(defaultCodeTheme)

  useEffect(() => {
    const load = themes.get(theme)
    if (!hasBlocks || !load) return

    let active = true
    const style = document.createElement('style')
    // 应用选择器之前先保持禁用，避免正文以外的样式短暂变化。
    style.media = 'not all'
    style.dataset.readerCodeTheme = theme
    void load().then(css => {
      if (!active || !css.trim()) return
      style.textContent = css
      document.head.append(style)
      if (!style.sheet) {
        style.remove()
        return
      }
      scopeRules(style.sheet.cssRules, `.reader-html[data-code-theme="${theme}"]`)
      style.textContent = Array.from(style.sheet.cssRules, rule => rule.cssText).join('\n')
      style.media = 'all'
      setLoaded(theme)
    }).catch(() => style.remove())

    return () => {
      active = false
      style.remove()
    }
  }, [hasBlocks, theme])

  return hasBlocks && loaded === theme ? theme : defaultCodeTheme
}
