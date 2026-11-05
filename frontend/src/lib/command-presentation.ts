import type { IconMeaning } from '@/components/animated-glyph'
import type { CommandID } from './commands'

export const commandIcons = {
  'import-opml':'folder','export-opml':'folder','reader-mode':'book','open-original':'book',categories:'folder',refresh:'refresh',settings:'settings',about:'circle-help',help:'circle-help',feeds:'radio-tower',ai:'ai',search:'search',palette:'search',
  all:'book',unread:'unread',favorites:'favorite',featured:'ai',list:'list',masonry:'grid',sidebar:'collapse',reader:'panel-right-open',
  'theme-system':'monitor-cog','theme-light':'sun','theme-dark':'moon',next:'chevron-right',previous:'chevron-left',favorite:'favorite',read:'read',
  close:'x',minimize:'minimize',maximize:'maximize',fullscreen:'maximize',quit:'x',
} satisfies Record<CommandID,IconMeaning>
export const textActions = {
  undo:{label:'撤销',icon:'undo'},redo:{label:'重做',icon:'redo'},cut:{label:'剪切',icon:'delete'},
  copy:{label:'复制',icon:'copy'},paste:{label:'粘贴',icon:'clipboard-check'},selectAll:{label:'全选',icon:'scan-text'},
} satisfies Record<string,{label:string;icon:IconMeaning}>
export type TextAction = keyof typeof textActions
export type Shortcut = {label:string;keys:string}
const modified=(key:string,mac:boolean,shift=false):Shortcut=>({label:mac?`${shift?'⇧':''}⌘${key}`:`Ctrl+${shift?'Shift+':''}${key}`,keys:`${mac?'Meta':'Control'}+${shift?'Shift+':''}${key}`})
/** 只展示已在 Shell 或 macOS 系统菜单注册的绑定。 */
export function commandShortcut(id:CommandID,mac:boolean,articleTargetMatches=true):Shortcut|undefined {
  switch(id) {
    case 'refresh':return modified('R',mac)
    case 'settings':return modified(',',mac)
    case 'search':return modified('F',mac)
    case 'palette':return modified('K',mac)
    case 'all':return modified('1',mac)
    case 'unread':return modified('2',mac)
    case 'favorites':return modified('3',mac)
    case 'feeds':return mac?modified('N',mac):undefined
    case 'sidebar':return mac?modified('L',mac,true):undefined
    case 'next':return {label:'J',keys:'J'}
    case 'previous':return {label:'K',keys:'K'}
    case 'favorite':return articleTargetMatches?{label:'F',keys:'F'}:undefined
    case 'read':return articleTargetMatches?{label:'M',keys:'M'}:undefined
    case 'close':return mac?modified('W',mac):undefined
    case 'quit':return mac?modified('Q',mac):undefined
    default:return undefined
  }
}
export function textShortcut(action:TextAction,mac:boolean):Shortcut {
  return action==='redo'?modified(mac?'Z':'Y',mac,mac):modified({undo:'Z',cut:'X',copy:'C',paste:'V',selectAll:'A'}[action],mac)
}
