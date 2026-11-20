import { useEffect, useRef, useState } from 'react'
import { Check, CodeXml, Copy } from 'lucide-react'
import { AnimatedGlyph } from '@/components/animated-glyph'
import { Button } from '@/components/ui/button'
import { native } from '@/lib/native'
import { notify } from '@/lib/notifications'

export function CodeToolbar({ text, language }: { text: string; language?: string }) {
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
      await native.copy(text)
      if (!mounted.current) return
      clearTimeout(timer.current)
      setCopied(true)
      timer.current = setTimeout(() => setCopied(false), 1600)
    } catch {
      void notify('复制代码失败', undefined, true)
    }
  }

  return <>
    <span className="reader-code-language"><CodeXml aria-hidden="true" />{language || '纯文本'}</span>
    <Button variant="ghost" size="icon-xs" className="reader-code-copy" aria-label="复制代码" title={copied ? '已复制' : '复制代码'}
      onClick={event => { event.stopPropagation(); void copy() }}>
      {copied ? <Check aria-hidden="true" /> : <AnimatedGlyph icon={Copy} meaning="copy" />}
    </Button>
    <span className="sr-only" role="status">{copied ? '代码已复制' : ''}</span>
  </>
}
