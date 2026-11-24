import { useRef, useState } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import { AnimatedGlyph } from '@/components/animated-glyph'
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Command, CommandEmpty, CommandGroup, CommandInput, CommandItem, CommandList, CommandSeparator } from '@/components/ui/command'
import { Kbd } from '@/components/ui/kbd'
import { useWorkspace } from '@/stores/workspace'
import { runCommand } from '@/lib/commands'
import type { CommandID } from '@/lib/commands'
import { CommandPresentation } from '@/components/command-presentation'
import { commandShortcut } from '@/lib/command-presentation'
import { askGlobalAI, useChatTarget } from '@/features/chat/chat-scope'
import { native } from '@/lib/native'

const entries: CommandID[] = ['all', 'favorites', 'search', 'masonry', 'settings', 'refresh', 'feeds', 'categories', 'import-opml', 'export-opml', 'reader-mode', 'open-original', 'ai', 'theme-light', 'theme-dark', 'help']

export function CommandPalette() {
  const open = useWorkspace(state => state.paletteOpen)
  const client = useQueryClient()
  const [query, setQuery] = useState('')
  const listRef = useRef<HTMLDivElement>(null)
  const trimmed = query.trim()

  function closePalette() {
    setQuery('')
    useWorkspace.setState({ paletteOpen: false })
  }

  function askAI() {
    if (!trimmed) return
    closePalette()
    if (useWorkspace.getState().settingsOpen) useWorkspace.getState().closeSettings()
    askGlobalAI(trimmed)
  }

  const items = (start: number, end: number) => entries.slice(start, end).map(id => (
    <CommandItem
      key={id}
      aria-keyshortcuts={commandShortcut(id, native.isMac())?.keys}
      onSelect={() => {
        closePalette()
        runCommand(id, client)
      }}
    >
      <CommandPresentation id={id} label={id === 'settings' ? '打开设置' : undefined} />
    </CommandItem>
  ))

  return (
    <Dialog
      open={open}
      onOpenChange={value => {
        if (!value) setQuery('')
        useWorkspace.setState({ paletteOpen: value })
      }}
    >
      <DialogContent
        className="command-dialog"
        showCloseButton={false}
        onCloseAutoFocus={event => {
          event.preventDefault()
          const state = useWorkspace.getState()
          if (!state.settingsOpen && useChatTarget.getState().target?.kind !== 'global') {
            document.getElementById(state.preferences?.sidebar_collapsed ? 'sidebar-toggle' : 'open-command')?.focus()
          }
        }}
      >
        <DialogHeader className="sr-only">
          <DialogTitle>命令面板</DialogTitle>
          <DialogDescription>搜索视图、设置与操作，方向键选择，回车执行。</DialogDescription>
        </DialogHeader>
        <Command label="搜索命令">
          <CommandInput
            placeholder="搜索命令…"
            aria-label="搜索命令"
            value={query}
            onValueChange={setQuery}
            onKeyDown={event => {
              if (
                event.key === 'Enter'
                && !event.nativeEvent.isComposing
                && trimmed
                && listRef.current?.querySelector('[data-slot="command-empty"]')
              ) {
                event.preventDefault()
                askAI()
              }
            }}
          />
          <CommandList ref={listRef} aria-label="可用命令">
            <CommandEmpty className={trimmed ? 'p-1.5 text-left' : undefined}>
              {trimmed ? (
                <div className="space-y-1">
                  <p className="px-2 pt-1 text-xs text-muted-foreground">没有找到匹配命令</p>
                  <button
                    type="button"
                    onClick={askAI}
                    className="flex w-full items-center gap-2 rounded-lg bg-muted/60 px-2 py-1.5 text-left text-sm text-foreground transition-colors hover:bg-muted focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                  >
                    <AnimatedGlyph meaning="ai" className="size-4 shrink-0 text-primary [&_svg]:size-4" />
                    <span className="min-w-0 flex-1 truncate">询问 AI：“{trimmed}”</span>
                    <Kbd className="ml-auto shrink-0">Enter</Kbd>
                  </button>
                </div>
              ) : '没有找到匹配命令'}
            </CommandEmpty>
            <CommandGroup heading="阅读与导航">{items(0, 4)}</CommandGroup>
            <CommandSeparator />
            <CommandGroup heading="应用">{items(4, entries.length)}</CommandGroup>
          </CommandList>
        </Command>
      </DialogContent>
    </Dialog>
  )
}
