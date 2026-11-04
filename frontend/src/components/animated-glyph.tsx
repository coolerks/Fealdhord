import { createContext, useContext, useEffect, useRef, useState } from 'react'
import {BookText,Bookmark,CheckCheck,Layers,PanelLeftClose,PanelLeftOpen,CircleDashed} from 'lucide-react'
import type { LucideIcon } from 'lucide-react'
import { SettingsIcon } from './icons/settings'
import { SearchIcon } from './icons/search'
import { BookmarkIcon } from './icons/bookmark'
import { FolderOpenIcon } from './icons/folder-open'
import { LayoutGridIcon } from './icons/layout-grid'
import { SparklesIcon } from './icons/sparkles'
import { PanelLeftCloseIcon } from './icons/panel-left-close'
import { PanelLeftOpenIcon } from './icons/panel-left-open'
import { CheckCheckIcon } from './icons/check-check'
import { BookTextIcon } from './icons/book-text'
import { LayersIcon } from './icons/layers'
import { RefreshCWIcon } from './icons/refresh-cw'
import { PlusIcon } from './icons/plus'
import { CircleDashedIcon } from './icons/circle-dashed'

export const IconMotionContext=createContext<'full'|'subtle'|'off'>('full')
type Handle={startAnimation:()=>void;stopAnimation:()=>void}
const icons={settings:SettingsIcon,search:SearchIcon,favorite:BookmarkIcon,folder:FolderOpenIcon,grid:LayoutGridIcon,ai:SparklesIcon,collapse:PanelLeftCloseIcon,expand:PanelLeftOpenIcon,read:CheckCheckIcon,book:BookTextIcon,list:LayersIcon,refresh:RefreshCWIcon,add:PlusIcon,unread:CircleDashedIcon}
const staticIcons:Partial<Record<keyof typeof icons,LucideIcon>>={book:BookText,favorite:Bookmark,read:CheckCheck,list:Layers,collapse:PanelLeftClose,expand:PanelLeftOpen,unread:CircleDashed}
export type IconMeaning=keyof typeof icons
export function AnimatedGlyph({icon:Fallback,meaning='book'}:{icon:LucideIcon;meaning?:string}) {
 const mode=useContext(IconMotionContext)
 const holder=useRef<HTMLSpanElement>(null),handle=useRef<Handle>(null)
 const [mounted,setMounted]=useState(false)
 const Component=Object.hasOwn(icons,meaning)?icons[meaning as IconMeaning]:undefined
 const Static=staticIcons[meaning as IconMeaning]??Fallback
 useEffect(()=>{
  const trigger=holder.current?.closest<HTMLElement>('button,[role=radio],[data-animated-trigger]')
  if(!trigger||!Component||mode!=='full')return
  let timer:ReturnType<typeof setTimeout>|undefined
  const enter=()=>{if(trigger.hasAttribute('disabled'))return;clearTimeout(timer);setMounted(true);handle.current?.startAnimation()}
  const leave=()=>{if(trigger.contains(document.activeElement))return;handle.current?.stopAnimation();timer=setTimeout(()=>setMounted(false),350)}
  trigger.addEventListener('mouseenter',enter);trigger.addEventListener('mouseleave',leave);trigger.addEventListener('focusin',enter);trigger.addEventListener('focusout',leave);trigger.addEventListener('pointerdown',enter)
  return()=>{clearTimeout(timer);trigger.removeEventListener('mouseenter',enter);trigger.removeEventListener('mouseleave',leave);trigger.removeEventListener('focusin',enter);trigger.removeEventListener('focusout',leave);trigger.removeEventListener('pointerdown',enter)}
 },[Component,mode])
 useEffect(()=>{if(mounted&&mode==='full')handle.current?.startAnimation()},[mounted,mode])
 return <span ref={holder} className="animated-glyph" data-icon="inline-start" data-meaning={meaning} data-animating={mounted&&mode==='full'} aria-hidden="true">{Component&&mounted&&mode==='full'?<Component ref={handle} size={20}/>:<Static/>}</span>
}
