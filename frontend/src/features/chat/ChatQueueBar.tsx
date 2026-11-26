import { useState } from 'react'
import { Check, CornerDownRight, Paperclip, Pencil, Trash2, X } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import type { SendAttachment } from '../../../bindings/github.com/coolerks/Fealdhord/internal/chat/models'

export type QueuedChatMessage = {
  id: string
  text: string
  attachments: SendAttachment[]
  skills?: string[]
  planMode: boolean
}

export function ChatQueueBar({ items, busySteer, onSteer, onEdit, onDelete }: {
  items: QueuedChatMessage[]
  busySteer: boolean
  onSteer: (item: QueuedChatMessage) => void
  onEdit: (id: string, text: string) => void
  onDelete: (id: string) => void
}) {
  const [editingId, setEditingId] = useState<string | null>(null)
  const [draft, setDraft] = useState('')

  if (items.length === 0) return null

  function startEdit(item: QueuedChatMessage) {
    setEditingId(item.id)
    setDraft(item.text)
  }

  function saveEdit(id: string) {
    const trimmed = draft.trim()
    const target = items.find(item => item.id === id)
    if (!trimmed && (!target || target.attachments.length === 0)) return
    onEdit(id, trimmed)
    setEditingId(null)
    setDraft('')
  }

  return <section className="chat-queue-bar" aria-label="待发送队列">
    <div className="flex items-center justify-between px-1 text-[11px] text-muted-foreground">
      <span>排队中（{items.length}）· 当前回复完成后自动发送</span>
    </div>
    {items.map(item => (
      <div key={item.id} className="chat-queue-item">
        {editingId === item.id ? (
          <form className="flex min-w-0 flex-1 items-center gap-1" onSubmit={event => {
            event.preventDefault()
            saveEdit(item.id)
          }}>
            <Input
              aria-label="编辑排队消息"
              className="h-7 text-xs"
              value={draft}
              onChange={event => setDraft(event.target.value)}
              onKeyDown={event => {
                if (event.key === 'Escape') {
                  event.preventDefault()
                  setEditingId(null)
                }
              }}
              autoFocus
            />
            <Button type="submit" size="icon-xs" variant="ghost" aria-label="保存编辑">
              <Check className="size-3.5" />
            </Button>
            <Button type="button" size="icon-xs" variant="ghost" aria-label="取消编辑" onClick={() => setEditingId(null)}>
              <X className="size-3.5" />
            </Button>
          </form>
        ) : (
          <>
            <span className="min-w-0 flex-1 truncate text-foreground" title={item.text || '（仅附件）'}>
              {item.text || '（仅附件）'}
            </span>
            {item.attachments.length > 0 && (
              <span className="inline-flex shrink-0 items-center gap-0.5 text-[11px] text-muted-foreground">
                <Paperclip className="size-3" aria-hidden="true" />
                {item.attachments.length}
              </span>
            )}
            <div className="flex shrink-0 items-center gap-0.5">
              <Tooltip>
                <TooltipTrigger asChild>
                  <Button type="button" size="xs" variant="ghost" className="h-6 gap-1 px-1.5 text-[11px]"
                    aria-label="切换到引导" disabled={busySteer} onClick={() => onSteer(item)}>
                    <CornerDownRight className="size-3" />
                    <span>引导</span>
                  </Button>
                </TooltipTrigger>
                <TooltipContent>切换到引导，在 AI 下一轮次立即接收</TooltipContent>
              </Tooltip>
              <Tooltip>
                <TooltipTrigger asChild>
                  <Button type="button" size="icon-xs" variant="ghost" aria-label="编辑排队消息" onClick={() => startEdit(item)}>
                    <Pencil className="size-3" />
                  </Button>
                </TooltipTrigger>
                <TooltipContent>编辑</TooltipContent>
              </Tooltip>
              <Tooltip>
                <TooltipTrigger asChild>
                  <Button type="button" size="icon-xs" variant="ghost" aria-label="删除排队消息" onClick={() => onDelete(item.id)}>
                    <Trash2 className="size-3" />
                  </Button>
                </TooltipTrigger>
                <TooltipContent>删除</TooltipContent>
              </Tooltip>
            </div>
          </>
        )}
      </div>
    ))}
  </section>
}
