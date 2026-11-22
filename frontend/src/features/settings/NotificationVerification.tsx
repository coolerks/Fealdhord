import { useEffect, useState } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import { Button } from '@/components/ui/button'
import { DialogFooter } from '@/components/ui/dialog'
import { FieldError, FieldLabel } from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { StatusBadge } from './settings-primitives'
import * as api from '../../../bindings/github.com/coolerks/Fealdhord/internal/notification/api'
import type { Channel } from '../../../bindings/github.com/coolerks/Fealdhord/internal/notification/models'

type Attempt = { state: 'sending' | 'sent' | 'failed' | 'verifying' | 'verified'; error?: string }

function errorMessage(error: unknown) {
  return error instanceof Error ? error.message : String(error)
}

export function NotificationVerification({ channel, pairingID, label, onClose }: {
  channel: Channel
  pairingID: string
  label(target: string): string
  onClose(): void
}) {
  const client = useQueryClient()
  const [attempts, setAttempts] = useState<Record<string, Attempt>>({})
  const [codes, setCodes] = useState<Record<string, string>>({})
  const recipients = channel.recipients ?? []
  const complete = recipients.every(recipient => recipient.verified || attempts[recipient.id]?.state === 'verified')

  useEffect(() => {
    let stopped = false
    let cancel: (() => void) | undefined
    void Promise.resolve().then(async () => {
      for (const recipient of channel.recipients ?? []) {
        if (stopped) return
        if (recipient.verified) continue
        setAttempts(previous => ({ ...previous, [recipient.id]: { state: 'sending' } }))
        const request = api.SendRecipientVerification(channel.id, channel.revision, recipient.id, pairingID, false)
        cancel = () => { void request.cancel?.() }
        let attempt: Attempt
        try {
          await request
          attempt = { state: 'sent' }
        } catch (error) {
          attempt = { state: 'failed', error: errorMessage(error) }
        }
        if (!stopped) setAttempts(previous => ({ ...previous, [recipient.id]: attempt }))
        cancel = undefined
      }
    })
    return () => { stopped = true; cancel?.() }
  }, [channel, pairingID])

  async function resend(id: string) {
    const previous = attempts[id]
    setAttempts(items => ({ ...items, [id]: { state: 'sending' } }))
    try {
      await api.SendRecipientVerification(channel.id, channel.revision, id, pairingID, true)
      setCodes(items => ({ ...items, [id]: '' }))
      setAttempts(items => ({ ...items, [id]: { state: 'sent' } }))
    } catch (error) {
      setAttempts(items => ({ ...items, [id]: { state: previous.state, error: errorMessage(error) } }))
    }
  }

  async function confirm(id: string) {
    setAttempts(items => ({ ...items, [id]: { state: 'verifying' } }))
    try {
      await api.ConfirmRecipientVerification(channel.id, channel.revision, id, codes[id])
      setCodes(items => ({ ...items, [id]: '' }))
      setAttempts(items => ({ ...items, [id]: { state: 'verified' } }))
      void client.invalidateQueries({ queryKey: ['notifications', 'channels'] })
    } catch (error) {
      setAttempts(items => ({ ...items, [id]: { state: 'sent', error: errorMessage(error) } }))
    }
  }

  return <>
    {!complete && <p className="text-sm text-muted-foreground">填写各收件目标收到的四位校验码。</p>}
    {recipients.map(recipient => {
      const attempt = attempts[recipient.id]
      const verified = recipient.verified || attempt?.state === 'verified'
      const canEnter = attempt?.state === 'sent' || attempt?.state === 'verifying'
      return (
        <div key={recipient.id} className="space-y-2 rounded-lg border border-border p-3">
          <div className="flex items-center justify-between gap-2">
            <span className="min-w-0 break-all text-sm font-medium">{label(recipient.target)}</span>
            {(verified || !attempt || attempt.state === 'sending') && (
              <StatusBadge tone={verified ? 'active' : 'muted'}>{verified ? '已验证' : attempt ? '正在发送' : '等待发送'}</StatusBadge>
            )}
          </div>
          {canEnter && (
            <form className="flex items-end gap-2" onSubmit={event => {
              event.preventDefault()
              if (attempt.state === 'sent' && /^\d{4}$/.test(codes[recipient.id] ?? '')) void confirm(recipient.id)
            }}>
              <div className="min-w-0 flex-1 space-y-1">
                <FieldLabel htmlFor={`recipient-code-${recipient.id}`}>四位校验码</FieldLabel>
                <Input id={`recipient-code-${recipient.id}`} type="text" inputMode="numeric" autoComplete="one-time-code"
                  maxLength={4} disabled={attempt.state === 'verifying'} value={codes[recipient.id] ?? ''}
                  onChange={event => {
                    const code = event.target.value.replace(/[^0-9]/gu, '')
                    setCodes(items => ({ ...items, [recipient.id]: code }))
                  }} />
              </div>
              <Button type="submit" size="sm" disabled={attempt.state === 'verifying' || !/^\d{4}$/.test(codes[recipient.id] ?? '')}>验证</Button>
            </form>
          )}
          {attempt?.error && <FieldError>{attempt.error}</FieldError>}
          {!verified && (attempt?.state === 'sent' || attempt?.state === 'failed') && (
            <Button variant="ghost" size="sm" onClick={() => void resend(recipient.id)}>重发校验码</Button>
          )}
        </div>
      )
    })}
    <DialogFooter>
      <Button size="sm" variant={complete ? 'default' : 'outline'} onClick={onClose}>{complete ? '完成' : '稍后验证'}</Button>
    </DialogFooter>
  </>
}
