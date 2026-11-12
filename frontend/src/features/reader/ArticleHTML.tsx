import { useEffect, useLayoutEffect, useMemo, useRef, useState } from 'react'
import { createPortal } from 'react-dom'
import { Check, CodeXml, Copy } from 'lucide-react'
import { AnimatedGlyph } from '@/components/animated-glyph'
import { Button } from '@/components/ui/button'
import { native } from '@/lib/native'
import { notify } from '@/lib/notifications'
import { readerImageID } from '@/lib/reader-image'
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

function CodeToolbar({ block }: { block: CodeBlock }) {
  const [copied, setCopied] = useState(false)
  const timer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined)
  const mounted = useRef(false)
  useEffect(() => {
    mounted.current = true
    return () => {
      mounted.current = false
      clearTimeout(timer.current)
    }
  }, [])

  async function copy() {
    try {
      await native.copy(block.text)
      if (!mounted.current) return
      clearTimeout(timer.current)
      setCopied(true)
      timer.current = setTimeout(() => setCopied(false), 1600)
    } catch {
      void notify('复制代码失败', undefined, true)
    }
  }

  return <>
    <span className="reader-code-language"><CodeXml aria-hidden="true" />{block.language || '纯文本'}</span>
    <Button variant="ghost" size="icon-xs" className="reader-code-copy" aria-label="复制代码" title={copied ? '已复制' : '复制代码'}
      onClick={event => { event.stopPropagation(); void copy() }}>
      {copied ? <Check aria-hidden="true" /> : <AnimatedGlyph icon={Copy} meaning="copy" />}
    </Button>
    <span className="sr-only" role="status">{copied ? '代码已复制' : ''}</span>
  </>
}

export function ArticleHTML({ html, imagesAllowed, theme }: { html: string; imagesAllowed: boolean; theme: string }) {
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
    {blocks?.map((block, index) => createPortal(<CodeToolbar block={block} />, block.toolbar, index))}
  </>
}
