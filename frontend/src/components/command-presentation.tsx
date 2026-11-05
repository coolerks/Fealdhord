import { AnimatedGlyph } from './animated-glyph'
import { commandLabels } from '@/lib/commands'
import type { CommandID } from '@/lib/commands'
import { commandIcons, commandShortcut, textActions, textShortcut } from '@/lib/command-presentation'
import type { TextAction } from '@/lib/command-presentation'
import { native } from '@/lib/native'

export function CommandPresentation({id,label,articleTargetMatches=true}:{id:CommandID;label?:string;articleTargetMatches?:boolean}) {
 const shortcut=commandShortcut(id,native.isMac(),articleTargetMatches)
 return <><AnimatedGlyph meaning={commandIcons[id]}/><span>{label??commandLabels[id]}</span>{shortcut&&<span className="menu-shortcut" data-slot="command-shortcut" aria-hidden="true">{shortcut.label}</span>}</>
}
export function TextActionPresentation({action}:{action:TextAction}) {
 const item=textActions[action],shortcut=textShortcut(action,native.isMac())
 return <><AnimatedGlyph meaning={item.icon}/><span>{item.label}</span><span className="menu-shortcut" data-slot="command-shortcut" aria-hidden="true">{shortcut.label}</span></>
}
