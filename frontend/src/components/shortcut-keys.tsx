import { Fragment } from 'react'
import { Kbd, KbdGroup } from '@/components/ui/kbd'

const labels: Record<string, string> = { Meta: '⌘', Control: 'Ctrl', Escape: 'Esc' }

export function ShortcutKeys({ keys, menu = false }: { keys: string; menu?: boolean }) {
  return (
    <KbdGroup className={menu ? 'menu-shortcut' : undefined} data-slot={menu ? 'command-shortcut' : 'kbd-group'} aria-hidden={menu || undefined}>
      {keys.split('+').map((key, index) => (
        <Fragment key={key}>
          {index > 0 && <span>+</span>}
          <Kbd>{labels[key] ?? key}</Kbd>
        </Fragment>
      ))}
    </KbdGroup>
  )
}
