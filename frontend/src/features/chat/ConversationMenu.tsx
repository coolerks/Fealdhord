import type { ReactElement } from 'react'
import { Archive, GitBranch, Pencil, Pin, PinOff, Trash2 } from 'lucide-react'
import { ContextMenu, ContextMenuContent, ContextMenuGroup, ContextMenuItem, ContextMenuSeparator, ContextMenuTrigger } from '@/components/ui/context-menu'
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Field, FieldError, FieldLabel } from '@/components/ui/field'
import { ConfirmDialog } from '@/features/settings/settings-primitives'
import type { Conversation } from '../../../bindings/github.com/coolerks/Fealdhord/internal/chat/models'

export type ConversationAction = 'rename' | 'pin' | 'branch' | 'archive' | 'delete'
export const archivedConversationsKey = ['chat', 'archived-conversations'] as const
export const globalConversationsKey = ['chat', 'conversations', 'global', ''] as const

export function conversationActionError(error: unknown) {
  const code = error instanceof Error ? error.message : ''
  if (code.includes('Conflict')) return '请先停止回复或等待回复结束。'
  if (code.includes('ChatBranchUnavailable')) return '此会话暂无可用于分支的回复。'
  if (code.includes('ValidationError')) return '请输入有效的会话名称。'
  if (code.includes('ChatConversationUnavailable')) return '会话状态已变化，请刷新后重试。'
  return '会话操作失败，请重试。'
}

export function ConversationMenu({ item, active, busy, onAction, children }: {
  item: Conversation
  active: boolean
  busy: boolean
  onAction: (action: ConversationAction, item: Conversation) => void
  children: ReactElement
}) {
  return <ContextMenu>
    <ContextMenuTrigger asChild disabled={busy} data-conversation-menu onKeyDown={event => {
      if (busy || (event.key !== 'ContextMenu' && !(event.shiftKey && event.key === 'F10'))) return
      event.preventDefault()
      event.stopPropagation()
      const { left, bottom } = event.currentTarget.getBoundingClientRect()
      event.currentTarget.dispatchEvent(new MouseEvent('contextmenu', {
        bubbles: true, cancelable: true, clientX: left, clientY: bottom,
      }))
    }}>{children}</ContextMenuTrigger>
    <ContextMenuContent aria-label="会话操作" className="action-menu">
      <ContextMenuGroup>
        <ContextMenuItem onSelect={() => onAction('rename', item)}><Pencil />重命名</ContextMenuItem>
        <ContextMenuItem onSelect={() => onAction('pin', item)}>
          {item.pinned ? <><PinOff />取消置顶</> : <><Pin />置顶</>}
        </ContextMenuItem>
        <ContextMenuItem disabled={active} onSelect={() => onAction('branch', item)}><GitBranch />分支到新聊天</ContextMenuItem>
      </ContextMenuGroup>
      <ContextMenuSeparator />
      <ContextMenuGroup>
        <ContextMenuItem disabled={active} onSelect={() => onAction('archive', item)}><Archive />归档</ContextMenuItem>
        <ContextMenuItem disabled={active} variant="destructive" onSelect={() => onAction('delete', item)}><Trash2 />删除</ContextMenuItem>
      </ContextMenuGroup>
    </ContextMenuContent>
  </ContextMenu>
}

export function ConversationConfirmation({ action, item, busy, error, onClose, onConfirm }: {
  action: 'archive' | 'delete'
  item: Conversation | null
  busy: boolean
  error?: string
  onClose: () => void
  onConfirm: () => void
}) {
  const archive = action === 'archive'
  return <ConfirmDialog open={Boolean(item)} title={item ? `${archive ? '归档' : '删除'}“${item.title}”？` : ''}
    description={archive ? '归档后可在设置的「已归档会话」中恢复。' : '会话及其中的消息将永久删除。'}
    confirmLabel={archive ? '确认归档' : '确认删除'} destructive={!archive} busy={busy} error={error}
    onOpenChange={open => { if (!open) onClose() }} onConfirm={onConfirm} />
}

export function RenameConversationDialog({ item, title, busy, error, onChange, onClose, onConfirm }: {
  item: Conversation | null
  title: string
  busy: boolean
  error?: string
  onChange: (title: string) => void
  onClose: () => void
  onConfirm: () => void
}) {
  const valid = new TextEncoder().encode(title.trim()).length <= 320 && Boolean(title.trim())
  return <Dialog open={Boolean(item)} onOpenChange={open => { if (!open && !busy) onClose() }}>
    {item && <DialogContent aria-describedby={undefined}>
      <DialogHeader><DialogTitle>重命名会话</DialogTitle></DialogHeader>
      <form className="flex flex-col gap-4" onSubmit={event => {
        event.preventDefault()
        if (valid && !busy) onConfirm()
      }}>
        <Field>
          <FieldLabel htmlFor="conversation-name">会话名称</FieldLabel>
          <Input id="conversation-name" value={title} disabled={busy} onChange={event => onChange(event.target.value)} />
        </Field>
        {error && <FieldError>{error}</FieldError>}
        <DialogFooter>
          <Button type="button" variant="outline" disabled={busy} onClick={onClose}>取消</Button>
          <Button type="submit" disabled={busy || !valid}>保存</Button>
        </DialogFooter>
      </form>
    </DialogContent>}
  </Dialog>
}
