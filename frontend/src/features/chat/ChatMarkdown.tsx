import { createContext, memo, useContext, useEffect, useLayoutEffect, useMemo, useRef, useState } from 'react'
import Markdown from 'react-markdown'
import type { Components } from 'react-markdown'
import remarkGfm from 'remark-gfm'
import { CodeToolbar } from '@/components/code-toolbar'
import { Checkbox } from '@/components/ui/checkbox'
import { native } from '@/lib/native'
import { notify } from '@/lib/notifications'
import type { highlightText } from '@/features/reader/highlight-code'
import { animateReplyFragment, revealMarkdown } from './stream-reveal'
import type { RevealBatch } from './stream-reveal'

const Streaming = createContext(false)
const remarkPlugins = [remarkGfm]

const ChatCode = memo(function ChatCode({ text, hint }: { text: string; hint?: string }) {
  const streaming = useContext(Streaming)
  const [snapshot, setSnapshot] = useState<{ text: string; hint?: string; result: ReturnType<typeof highlightText> }>()
  const result = snapshot?.text === text && snapshot.hint === hint ? snapshot.result : undefined
  useEffect(() => {
    if (streaming || !text.trim() || text.length > 8_000) return
    let current = true
    void import('@/features/reader/highlight-code').then(({ highlightText }) => {
      if (current) setSnapshot({ text, hint, result: highlightText(text, hint) })
    }).catch(() => { /* 保留纯文本代码。 */ })
    return () => { current = false }
  }, [streaming, text, hint])

  return <div className="reader-code-block hljs">
    <div className="reader-code-toolbar"><CodeToolbar text={text} language={result?.language ?? hint} /></div>
    <pre>{result?.html
      ? <code className={`language-${result.id}`} dangerouslySetInnerHTML={{ __html: result.html }} />
      : <code>{text}</code>}</pre>
  </div>
})

function externalURL(url: string) {
  try {
    const parsed = new URL(url)
    return parsed.protocol === 'https:' || parsed.protocol === 'http:' ? parsed.href : ''
  } catch {
    return ''
  }
}

const components: Components = {
  span: ({ node, children }) => {
    const revealAt = node?.properties.dataChatRevealAt
    const offset = node?.properties.dataChatRevealKey
    return typeof revealAt === 'number' && typeof offset === 'number'
      ? <RevealFragment at={revealAt} offset={offset}>{children}</RevealFragment>
      : <span>{children}</span>
  },
  a: ({ href, children }) => href ? <a href={href} onClick={event => {
    event.preventDefault()
    void native.openURL(href).catch(() => notify('无法打开链接', undefined, true))
  }}>{children}</a> : <span>{children}</span>,
  img: ({ alt }) => alt ? <span>{alt}</span> : null,
  input: ({ type, checked }) => type === 'checkbox'
    ? <Checkbox aria-label={checked ? '已完成' : '未完成'} checked={checked} disabled />
    : null,
  pre: ({ node, children }) => {
    const code = node?.children.find(child => child.type === 'element' && child.tagName === 'code')
    if (!code || code.type !== 'element') return <pre>{children}</pre>
    const className = code.properties.className
    const hint = Array.isArray(className)
      ? className.join(' ').match(/(?:^|\s)language-([\w+-]+)/)?.[1].toLowerCase()
      : undefined
    const text = code.children.map(child => child.type === 'text' ? child.value : '').join('').replace(/\n$/, '')
    return <ChatCode text={text} hint={hint} />
  },
}

function RevealFragment({ at, offset, children }: { at: number; offset: number; children: React.ReactNode }) {
  const ref = useRef<HTMLSpanElement>(null)
  useLayoutEffect(() => {
    if (ref.current) return animateReplyFragment(ref.current, at)
  }, [at, offset])
  return <span ref={ref} data-chat-reveal="">{children}</span>
}

export const ChatMarkdown = memo(function ChatMarkdown({ text, streaming, reveal }: {
  text: string; streaming: boolean; reveal?: readonly RevealBatch[]
}) {
  const rehypePlugins = useMemo(() => streaming && reveal?.length ? [revealMarkdown(text, reveal)] : [], [text, streaming, reveal])
  return <div className="reader-prose chat-answer">
    <Streaming.Provider value={streaming}>
      <Markdown skipHtml urlTransform={externalURL} remarkPlugins={remarkPlugins} rehypePlugins={rehypePlugins} components={components}>
        {text}
      </Markdown>
    </Streaming.Provider>
  </div>
})
