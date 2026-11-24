import { useLayoutEffect, useMemo, useRef, useState } from 'react'
import type { RefObject } from 'react'
import { createPortal } from 'react-dom'
import { CodeToolbar } from '@/components/code-toolbar'
import { readerImageID } from '@/lib/reader-image'
import type { TranslationDisplayMode } from '@/lib/translation-api'
import type { CodeBlock } from './highlight-code'
import { useReaderCodeTheme } from './code-themes'
import { readingBlock } from './reading-block'

function prepareCodeBlocks(container: HTMLDivElement): CodeBlock[] {
  const blocks: CodeBlock[] = []
  for (const pre of container.querySelectorAll('pre')) {
    const target = pre.querySelector<HTMLElement>(':scope > code:only-child') ?? pre
    let text = target.textContent ?? ''
    if (target.querySelector('br')) {
      const source = target.cloneNode(true) as HTMLElement
      for (const lineBreak of source.querySelectorAll('br')) lineBreak.replaceWith('\n')
      text = source.textContent ?? ''
    }
    if (!text.trim()) continue

    const hint = `${target.className} ${pre.className}`.match(/(?:^|\s)(?:language|lang)-([\w+-]+)/)?.[1].toLowerCase()
    const wrapper = document.createElement('div')
    wrapper.className = 'reader-code-block hljs'
    const toolbar = document.createElement('div')
    toolbar.className = 'reader-code-toolbar'
    pre.classList.remove('hljs')
    target.classList.remove('hljs')
    pre.before(wrapper)
    wrapper.append(toolbar, pre)
    blocks.push({ target, toolbar, text, hint, language: hint })
  }
  return blocks
}

const innerChildTags = new Set(['LI', 'TD', 'TH'])

function hideOriginalText(source: HTMLElement, restore: Array<() => void>) {
  for (const child of Array.from(source.childNodes)) {
    if (child instanceof HTMLElement) {
      if (child.tagName === 'IMG' || child.hidden) continue
      if (child.querySelector('img')) {
        hideOriginalText(child, restore)
        continue
      }
      child.hidden = true
      restore.push(() => { child.hidden = false })
    } else if (child instanceof Text && child.data.trim()) {
      const hidden = document.createElement('span')
      hidden.hidden = true
      child.replaceWith(hidden)
      hidden.append(child)
      restore.push(() => hidden.replaceWith(child))
    }
  }
}

export function ArticleHTML({
  html,
  imagesAllowed,
  theme,
  translationBlocks,
  translationMode = 'original',
  viewport,
}: {
  html: string
  imagesAllowed: boolean
  theme: string
  translationBlocks?: Record<string, string>
  translationMode?: TranslationDisplayMode
  viewport: RefObject<HTMLDivElement | null>
}) {
  const container = useRef<HTMLDivElement>(null)
  const renderedTranslations = useRef(new Map<string, { html: string; restore: () => void }>())
  const safeHTML = useMemo(() => ({ __html: html }), [html])
  const [snapshot, setSnapshot] = useState<{ html: string; blocks: CodeBlock[] }>()
  const blocks = snapshot?.html === html ? snapshot.blocks : undefined
  const appliedTheme = useReaderCodeTheme(theme, Boolean(blocks?.length))

  useLayoutEffect(() => {
    for (const image of container.current?.querySelectorAll('img') ?? []) {
      if (!readerImageID(image)) continue
      image.tabIndex = imagesAllowed ? 0 : -1
      image.setAttribute('role', 'button')
      image.setAttribute('aria-label', image.alt ? `查看图片：${image.alt}` : '查看图片')
    }
  }, [html, imagesAllowed])

  useLayoutEffect(() => {
    const rendered = renderedTranslations.current
    return () => {
      for (const item of rendered.values()) item.restore()
      rendered.clear()
    }
  }, [html, translationMode])

  useLayoutEffect(() => {
    const root = container.current
    if (!root) return
    if (translationMode === 'original' || !translationBlocks) return
    const rendered = renderedTranslations.current
    const scroll = viewport.current
    const anchor = scroll?.scrollTop ? readingBlock(root, scroll) : undefined
    const anchorTop = anchor?.getBoundingClientRect().top

    for (const source of root.querySelectorAll<HTMLElement>('[data-tr-block]')) {
      const id = source.dataset.trBlock
      if (!id) continue
      const translatedHTML = translationBlocks[id]
      const existing = rendered.get(id)
      if (existing?.html === translatedHTML) continue
      if (existing) {
        existing.restore()
        rendered.delete(id)
      }
      if (!translatedHTML) continue
      const restore: Array<() => void> = []

      const tag = source.tagName
      const inside = innerChildTags.has(tag)

      const translatedNode = document.createElement(translationMode === 'translated' ? 'span' : 'div')
      translatedNode.className = inside ? 'reader-tr-block reader-tr-inline-child' : 'reader-tr-block'
      translatedNode.dataset.trFor = id
      translatedNode.dataset.trTag = tag.toLowerCase()
      translatedNode.dataset.trMode = translationMode
      translatedNode.innerHTML = translatedHTML

      if (translationMode === 'translated') {
        hideOriginalText(source, restore)
        source.append(translatedNode)
      } else {
        source.dataset.trSource = 'bilingual'
        restore.push(() => source.removeAttribute('data-tr-source'))
        if (inside) source.append(translatedNode)
        else source.after(translatedNode)
      }
      restore.push(() => translatedNode.remove())
      rendered.set(id, { html: translatedHTML, restore: () => {
        for (const undo of restore.reverse()) undo()
      } })
    }
    if (scroll && anchor && anchorTop !== undefined) {
      const difference = anchor.getBoundingClientRect().top - anchorTop
      if (Math.abs(difference) > .5) scroll.scrollTop += difference
    }
  }, [html, translationBlocks, translationMode, viewport])

  useLayoutEffect(() => {
    let active = true
    queueMicrotask(() => {
      if (!active || !container.current) return
      const blocks = prepareCodeBlocks(container.current)
      setSnapshot({ html, blocks })
      if (!blocks.length) return
      void import('./highlight-code').then(({ highlightCode }) => {
        if (active) setSnapshot({ html, blocks: highlightCode(blocks) })
      }).catch(() => { /* 原始代码仍可阅读和复制。 */ })
    })
    return () => { active = false }
  }, [html])

  return <>
    <div ref={container} className="reader-html" data-code-theme={appliedTheme} dangerouslySetInnerHTML={safeHTML} />
    {blocks?.map((block, index) => createPortal(<CodeToolbar text={block.text} language={block.language} />, block.toolbar, index))}
  </>
}
