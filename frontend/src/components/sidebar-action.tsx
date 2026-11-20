import type { ComponentProps } from 'react'
import { AnimatedGlyph } from '@/components/animated-glyph'
import type { IconMeaning } from '@/components/animated-glyph'
import { ShortcutKeys } from '@/components/shortcut-keys'
import { Button } from '@/components/ui/button'

export function SidebarAction({ label, meaning, shortcut, ...props }: Omit<ComponentProps<typeof Button>, 'children'> & {
  label: string; meaning: IconMeaning; shortcut: string
}) {
  return <Button type="button" variant="outline" className="nav-item sidebar-search"
    aria-label={label} aria-keyshortcuts={shortcut} {...props}>
    <AnimatedGlyph meaning={meaning} />
    <span>{label}</span>
    <ShortcutKeys keys={shortcut} menu />
  </Button>
}
