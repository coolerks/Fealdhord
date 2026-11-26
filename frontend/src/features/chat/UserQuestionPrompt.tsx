import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Check } from 'lucide-react'
import { AnimatedGlyph } from '@/components/animated-glyph'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Input } from '@/components/ui/input'
import { cn } from '@/lib/utils'
import { AnswerQuestion, PendingQuestions } from '../../../bindings/github.com/coolerks/Fealdhord/internal/appagent/api'
import type { PendingQuestion, UserQuestionAnswer } from '../../../bindings/github.com/coolerks/Fealdhord/internal/appagent/models'

export const pendingQuestionsKey = ['ai-platform', 'questions'] as const

export function useInlineQuestion(conversationId: string, enabled: boolean) {
  const query = useQuery({
    queryKey: pendingQuestionsKey,
    queryFn: () => PendingQuestions(),
    enabled: enabled && Boolean(conversationId),
    refetchInterval: enabled ? 1000 : false,
    retry: false,
  })
  return enabled && conversationId
    ? query.data?.find(item => item.conversationId === conversationId)
    : undefined
}

type DraftAnswer = {
  selected: string[]
  note: string
}

export function UserQuestionPrompt({ pending, onStop, stopping }: {
  pending: PendingQuestion
  onStop?: () => void
  stopping?: boolean
}) {
  const cache = useQueryClient()
  const questions = pending.questions ?? []
  const [step, setStep] = useState(0)
  const [drafts, setDrafts] = useState<DraftAnswer[]>(() => questions.map(() => ({ selected: [], note: '' })))

  const submit = useMutation({
    mutationFn: ({ answers, cancelled }: { answers: UserQuestionAnswer[]; cancelled: boolean }) =>
      Promise.resolve(AnswerQuestion(pending.id, answers, cancelled)),
    onSettled: () => cache.invalidateQueries({ queryKey: pendingQuestionsKey }),
    onSuccess: () => {
      cache.setQueryData<PendingQuestion[]>(pendingQuestionsKey, current => current?.filter(item => item.id !== pending.id))
    },
  })

  if (questions.length === 0) return null

  const currentIndex = Math.min(step, questions.length - 1)
  const current = questions[currentIndex]
  const currentDraft = drafts[currentIndex] ?? { selected: [], note: '' }
  const total = questions.length
  const busy = Boolean(stopping || submit.isPending)

  function updateDraft(index: number, updater: (draft: DraftAnswer) => DraftAnswer) {
    setDrafts(prev => prev.map((item, i) => (i === index ? updater(item) : item)))
  }

  function toggleOption(label: string) {
    updateDraft(currentIndex, draft => {
      if (current.multi_select) {
        const exists = draft.selected.includes(label)
        return { ...draft, selected: exists ? draft.selected.filter(item => item !== label) : [...draft.selected, label] }
      }
      return { ...draft, selected: draft.selected[0] === label ? [] : [label] }
    })
  }

  function hasAnswer(draft?: DraftAnswer) {
    return Boolean(draft && (draft.selected.length > 0 || draft.note.trim().length > 0))
  }

  const anyAnswered = drafts.some(hasAnswer)

  function sendAnswers() {
    const answers: UserQuestionAnswer[] = questions.map((item, index) => {
      const draft = drafts[index] ?? { selected: [], note: '' }
      return {
        question: item.question,
        selected: draft.selected,
        note: draft.note.trim(),
      }
    })
    submit.mutate({ answers, cancelled: false })
  }

  return (
    <div className="flex flex-col gap-3">
      <div className="flex items-center justify-between gap-2">
        <div className="flex min-w-0 items-center gap-1.5">
          <AnimatedGlyph meaning="circle-help" className="chat-activity-glyph shrink-0 text-muted-foreground" />
          <span className="truncate text-sm font-medium">{current.header || '请确认以下问题'}</span>
          {current.multi_select && <span className="shrink-0 rounded-full bg-muted px-2 py-0.5 text-[11px] text-muted-foreground">多选</span>}
        </div>
        {total > 1 && (
          <span className="shrink-0 text-xs text-muted-foreground tabular-nums">{currentIndex + 1} / {total}</span>
        )}
      </div>

      {total > 1 && (
        <div className="flex flex-wrap gap-1" role="tablist" aria-label="问题列表">
          {questions.map((item, index) => {
            const active = index === currentIndex
            const answered = hasAnswer(drafts[index])
            return (
              <button key={item.id || index} type="button" role="tab" aria-selected={active}
                disabled={busy}
                onClick={() => setStep(index)}
                className={cn(
                  'inline-flex items-center gap-1 rounded-full border px-2.5 py-0.5 text-xs transition-colors',
                  active ? 'border-foreground/30 bg-muted font-medium text-foreground' : 'border-transparent text-muted-foreground hover:bg-muted/60',
                )}>
                {answered && <Check className="size-3 shrink-0" aria-hidden="true" />}
                <span>{item.header || `问题 ${index + 1}`}</span>
              </button>
            )
          })}
        </div>
      )}

      <p className="text-sm leading-relaxed text-foreground">{current.question}</p>

      <div className="flex flex-col gap-1.5" role={current.multi_select ? 'group' : 'radiogroup'} aria-label={current.question}>
        {(current.options ?? []).map(option => {
          const selected = currentDraft.selected.includes(option.label)
          if (current.multi_select) {
            const checkboxId = `ask-q-${pending.id}-${currentIndex}-${option.label}`
            return (
              <label key={option.label} htmlFor={checkboxId}
                className={cn(
                  'flex cursor-pointer items-start gap-2.5 rounded-xl border px-3 py-2 text-sm transition-colors',
                  selected ? 'border-foreground/30 bg-muted/70' : 'border-border hover:bg-muted/40',
                  busy && 'pointer-events-none opacity-50',
                )}>
                <Checkbox id={checkboxId} className="mt-0.5" checked={selected} disabled={busy}
                  onCheckedChange={() => toggleOption(option.label)} />
                <span className="min-w-0 flex-1">
                  <span className="block font-medium text-foreground">{option.label}</span>
                  {option.description && <span className="mt-0.5 block text-xs text-muted-foreground">{option.description}</span>}
                </span>
              </label>
            )
          }
          return (
            <button key={option.label} type="button" role="radio" aria-checked={selected} disabled={busy}
              onClick={() => toggleOption(option.label)}
              className={cn(
                'flex w-full items-start justify-between gap-2 rounded-xl border px-3 py-2 text-left text-sm transition-colors',
                selected ? 'border-foreground/30 bg-muted/70' : 'border-border hover:bg-muted/40',
                busy && 'pointer-events-none opacity-50',
              )}>
              <span className="min-w-0 flex-1">
                <span className="block font-medium text-foreground">{option.label}</span>
                {option.description && <span className="mt-0.5 block text-xs text-muted-foreground">{option.description}</span>}
              </span>
              {selected && <Check className="mt-0.5 size-4 shrink-0 text-foreground" aria-hidden="true" />}
            </button>
          )
        })}
      </div>

      <Input
        aria-label="补充说明"
        placeholder="补充说明或直接输入你的回答（可选）…"
        value={currentDraft.note}
        disabled={busy}
        maxLength={1000}
        onChange={event => updateDraft(currentIndex, draft => ({ ...draft, note: event.target.value }))}
        onKeyDown={event => {
          if (event.nativeEvent.isComposing || event.keyCode === 229) return
          if (event.key === 'Enter' && !event.shiftKey) {
            event.preventDefault()
            if (currentIndex < total - 1) setStep(currentIndex + 1)
            else if (anyAnswered && !busy) sendAnswers()
          }
        }}
      />

      {submit.isError && <p role="alert" className="text-sm text-destructive">提交失败，请重试。</p>}

      <div className="flex flex-wrap items-center justify-end gap-2">
        {onStop && (
          <Button type="button" variant="ghost" size="sm" className="mr-auto" disabled={busy} onClick={onStop}>
            停止回复
          </Button>
        )}
        <Button type="button" variant="outline" size="sm" disabled={busy}
          onClick={() => submit.mutate({ answers: [], cancelled: true })}>
          跳过
        </Button>
        {total > 1 && currentIndex > 0 && (
          <Button type="button" variant="outline" size="sm" disabled={busy} onClick={() => setStep(currentIndex - 1)}>
            上一题
          </Button>
        )}
        {currentIndex < total - 1 ? (
          <Button type="button" size="sm" disabled={busy} onClick={() => setStep(currentIndex + 1)}>
            下一题
          </Button>
        ) : (
          <Button type="button" size="sm" disabled={busy || !anyAnswered} onClick={sendAnswers}>
            提交回答
          </Button>
        )}
      </div>
    </div>
  )
}
