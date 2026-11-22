import { Events } from '@wailsio/runtime'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { useEffect, useMemo, useState } from 'react'
import { FealdhordChatUpdate } from '../../../bindings/events'
import { Messages } from '../../../bindings/github.com/coolerks/Fealdhord/internal/chat/api'
import type { Message } from '../../../bindings/github.com/coolerks/Fealdhord/internal/chat/models'
import { addRevealBatch } from './stream-reveal'
import type { RevealBatch } from './stream-reveal'

type LiveMessage = Message & { reveal?: readonly RevealBatch[] }

export const chatContextKey = (id: string) => ['chat-context', id] as const
export const chatMessagesKey = (id: string) => ['chat', 'messages', id] as const

export function useChatMessages(id: string) {
  const cache = useQueryClient()
  const [live, setLive] = useState<{ conversationId: string; messageId: string; text: string; reveal: RevealBatch[] }>()
  const query = useQuery({
    queryKey: chatMessagesKey(id),
    queryFn: () => Messages(id),
    enabled: Boolean(id),
    refetchInterval: query => query.state.data?.some(message => message.status === 'running') ? 1000 : false,
  })

  useEffect(() => {
    if (!id) return
    const key = chatMessagesKey(id)
    return Events.On(FealdhordChatUpdate, ({ data: event }) => {
      if (event.conversationId !== id) return
      if (event.done) {
        void cache.invalidateQueries({ queryKey: key, exact: true })
        void cache.invalidateQueries({ queryKey: chatContextKey(id) })
        return
      }
      const saved = cache.getQueryData<Message[]>(key)?.find(message => message.id === event.messageId)
      if (saved && saved.status !== 'running') return
      const receivedAt = performance.now()
      setLive(previous => {
        const message = previous?.conversationId === id && previous.messageId === event.messageId ? previous : undefined
        const text = message?.text ?? ''
        const base = saved && saved.text.length > text.length ? saved.text : text
        const overlap = base.length - event.offset
        // A snapshot may already include part of this batch, or a gap may need the next snapshot.
        if (overlap < 0 || overlap >= event.delta.length) return previous
        const next = base + event.delta.slice(overlap)
        return {
          conversationId: id, messageId: event.messageId, text: next,
          reveal: addRevealBatch(message?.reveal ?? [], { start: base.length, end: next.length, receivedAt }),
        }
      })
    })
  }, [id, cache])

  const data = useMemo<LiveMessage[] | null | undefined>(() => {
    if (!live || live.conversationId !== id) return query.data
    return query.data?.map(message => message.id === live.messageId && message.status === 'running' && live.text.length >= message.text.length
      ? { ...message, text: live.text, reveal: live.reveal }
      : message)
  }, [id, query.data, live])

  return { data, isError: query.isError, isPending: query.isPending, refetch: query.refetch }
}
