import { createContext, useContext, useEffect, useRef, useState } from 'react'
import type { HTMLAttributes } from 'react'
import type { LucideIcon } from 'lucide-react'
import { animatedIcons } from './icon-catalog'
import type { AnimatedIconName } from './icon-catalog'
import stills from './icon-stills.json'
import { cn } from '@/lib/utils'

export const IconMotionContext = createContext<'full' | 'subtle' | 'off'>('off')
export type IconHandle = { cancelAnimation: () => void; startAnimation: () => Promise<unknown>; stopAnimation: () => Promise<unknown> }
const aliases = { favorite: 'bookmark', folder: 'folder-open', grid: 'layout-grid', ai: 'sparkles', collapse: 'panel-left-close', expand: 'panel-left-open', read: 'check-check', book: 'book-text', list: 'layers', refresh: 'refresh-cw', add: 'plus', unread: 'circle-dashed' } as const
export type IconMeaning = AnimatedIconName | keyof typeof aliases
const triggerSelector = 'button,[role=radio],[role=menuitem],[role=menuitemradio],[role=menuitemcheckbox],[role=option],[data-animated-trigger]'

type Props = Omit<HTMLAttributes<HTMLSpanElement>, 'children'> & { icon?: LucideIcon; meaning?: IconMeaning }
/** 仅交互时挂载库动画；静态 SVG 由相同库源生成，避免首次悬停换形。 */
export function AnimatedGlyph({ icon: Fallback, meaning = 'book', className, ...props }: Props) {
  const mode = useContext(IconMotionContext)
  return <GlyphPlayback key={`${mode}:${meaning}`} mode={mode} icon={Fallback} meaning={meaning} className={className} {...props} />
}

function GlyphPlayback({ mode, icon: Fallback, meaning = 'book', className, ...props }: Props & {mode:'full'|'subtle'|'off'}) {
  const holder = useRef<HTMLSpanElement>(null)
  const handle = useRef<IconHandle>(null)
  const busy = useRef(false)
  const [playing, setPlaying] = useState(false)
  const name: AnimatedIconName = Object.hasOwn(aliases, meaning) ? aliases[meaning as keyof typeof aliases] : meaning as AnimatedIconName
  const Component = animatedIcons[name]

  useEffect(() => {
    const element = holder.current
    const trigger = element?.closest<HTMLElement>(triggerSelector) ?? element
    if (!trigger || mode !== 'full') return
    let hovered = false
    const enter = () => {
      // Radix 用空属性表示禁用；cmdk 的可用项也带 data-disabled="false"。
      if (busy.current || trigger.matches(':disabled,[aria-disabled=true],[data-disabled]:not([data-disabled=false])')) return
      busy.current = true
      setPlaying(true)
    }
    const hover = () => { hovered = true; enter() }
    const leave = () => { hovered = false }
    const focus = () => { if (!hovered) enter() }
    // cmdk 用 aria-activedescendant 选择选项，键盘移动时焦点仍在输入框。
    const selection = trigger.getAttribute('role') === 'option' ? new MutationObserver(() => {
      if (!hovered && trigger.getAttribute('data-selected') === 'true') enter()
    }) : null
    selection?.observe(trigger, { attributes: true, attributeFilter: ['data-selected'] })
    trigger.addEventListener('mouseenter', hover)
    trigger.addEventListener('mouseleave', leave)
    trigger.addEventListener('focusin', focus)
    return () => {
      selection?.disconnect()
      trigger.removeEventListener('mouseenter', hover)
      trigger.removeEventListener('mouseleave', leave)
      trigger.removeEventListener('focusin', focus)
    }
  }, [Component, mode])

  useEffect(() => {
    if (!playing || mode !== 'full') return
    const animation = handle.current
    if (!animation) return
    let current = true
    // 图标是本轮悬停才挂载的子树。等 passive effects 与 StrictMode 重放完成，
    // 再启动真实播放；否则子树清理会停止首轮动画并悬置完成 Promise。
    queueMicrotask(() => {
      if (!current) return
      void (async () => {
        try {
          await animation.startAnimation()
          if (current) await animation.stopAnimation()
        } catch {
          // 装饰动画失败时归回静态图标，不影响原操作。
        } finally {
          if (current) { busy.current = false; setPlaying(false) }
        }
      })()
    })
    return () => { current = false; animation.cancelAnimation() }
  }, [playing, mode, Component])

  return <span {...props} ref={holder} className={cn('animated-glyph', className)} data-icon="inline-start" data-meaning={meaning} data-animating={playing && mode === 'full'} aria-hidden="true">
    {playing && mode === 'full' ? <Component ref={handle} size={20} /> : stills[name] ? <span className="icon-still" dangerouslySetInnerHTML={{ __html: stills[name] }} /> : Fallback ? <Fallback /> : null}
  </span>
}
