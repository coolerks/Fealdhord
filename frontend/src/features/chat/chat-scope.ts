import { create } from 'zustand'
import type { Scope } from '../../../bindings/github.com/coolerks/Fealdhord/internal/chat/models'

type ChatTarget = Scope & { label: string; selection?: string }
export const useChatTarget = create<{ target: ChatTarget | null }>(() => ({ target: null }))

export function openChat(target: ChatTarget) {
  useChatTarget.setState({ target })
}
