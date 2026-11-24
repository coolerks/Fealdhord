import { create } from 'zustand'
import type { Scope } from '../../../bindings/github.com/coolerks/Fealdhord/internal/chat/models'

type ChatTarget = Scope & { label: string; selection?: string }
type PendingPrompt = { id: number; text: string }

export const useChatTarget = create<{
  target: ChatTarget | null
  pendingPrompt: PendingPrompt | null
}>(() => ({ target: null, pendingPrompt: null }))

let nextPromptId = 1

export function openChat(target: ChatTarget) {
  useChatTarget.setState({ target })
}

export function askGlobalAI(text: string) {
  const trimmed = text.trim()
  if (!trimmed) return
  useChatTarget.setState({
    target: { kind: 'global', id: '', label: 'AI 助手' },
    pendingPrompt: { id: nextPromptId++, text: trimmed },
  })
}
