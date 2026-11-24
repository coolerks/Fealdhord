import { useLayoutEffect, useMemo, useRef, useState } from 'react'
import { createPortal } from 'react-dom'
import { CodeToolbar } from '@/components/code-toolbar'
import { readerImageID } from '@/lib/reader-image'
import type { TranslationDisplayMode } from '@/lib/translation-api'
import type { CodeBlock } from './highlight-code'
import { useReaderCodeTheme } from './code-themes'

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

export function ArticleHTML({
  html,
  imagesAllowed,
  theme,
  translationBlocks,
  translationMode = 'original',
}: {
  html: string
  imagesAllowed: boolean
  theme: string
  translationBlocks?: Record<string, string>
  translationMode?: TranslationDisplayMode
}) {
  const container = useRef<HTMLDivElement>(null)
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
    const root = container.current
    if (!root) return
    // 清理旧译文节点与状态属性。
    for (const existing of root.querySelectorAll('.reader-tr-block')) existing.remove()
    for (const source of root.querySelectorAll<HTMLElement>('[data-tr-block]')) {
      source.removeAttribute('data-tr-source')
      source.removeAttribute('data-tr-hidden')
    }
    if (translationMode === 'original' || !translationBlocks) return

    for (const source of root.querySelectorAll<HTMLElement>('[data-tr-block]')) {
      const id = source.dataset.trBlock
      if (!id) continue
      const translatedHTML = translationBlocks[id]
      if (!translatedHTML) continue

      const tag = source.tagName
      const inside = innerChildTags.has(tag)
      const hasImage = Boolean(source.querySelector('img'))

      const translatedNode = document.createElement('div')
      translatedNode.className = inside ? 'reader-tr-block reader-tr-inline-child' : 'reader-tr-block'
      translatedNode.dataset.trFor = id
      translatedNode.dataset.trTag = tag.toLowerCase()
      translatedNode.dataset.trMode = translationMode
      translatedNode.innerHTML = translatedHTML

      if (translationMode === 'translated' && !hasImage && !inside) {
        source.dataset.trHidden = 'true'
      } else if (translationMode === 'bilingual') {
        source.dataset.trSource = 'bilingual'
      } else if (translationMode === 'translated' && inside && !hasImage) {
        source.dataset.trSource = 'replaced-child'
      }

      if (inside) {
        source.append(translatedNode)
      } else {
        source.after(translatedNode)
      }
    }
  }, [html, translationBlocks, translationMode])

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
