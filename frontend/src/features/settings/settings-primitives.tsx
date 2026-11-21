import type { ReactNode } from 'react'
import { Button } from '@/components/ui/button'
import {
  Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle,
} from '@/components/ui/dialog'
import { FieldError } from '@/components/ui/field'
import { cn } from '@/lib/utils'

export function SettingSection({
  title,
  actions,
  ariaLabel,
  className,
  children,
}: {
  title?: string
  actions?: ReactNode
  ariaLabel?: string
  className?: string
  children: ReactNode
}) {
  return (
    <section className={cn('space-y-2.5', className)} aria-label={ariaLabel ?? title}>
      {(title || actions) && (
        <div className="flex min-h-7 flex-wrap items-center justify-between gap-2">
          {title && <h2 className="text-[13px] font-semibold tracking-[-0.01em] text-foreground">{title}</h2>}
          {actions && <div className="ml-auto flex flex-wrap items-center gap-1.5">{actions}</div>}
        </div>
      )}
      {children}
    </section>
  )
}

export function SettingCard({
  className,
  divided = true,
  children,
}: {
  className?: string
  divided?: boolean
  children: ReactNode
}) {
  return (
    <div
      className={cn(
        'rounded-xl border border-border bg-card/50',
        divided && 'divide-y divide-border',
        className,
      )}
    >
      {children}
    </div>
  )
}

export function SettingRow({
  className,
  children,
}: {
  className?: string
  children: ReactNode
}) {
  return (
    <div className={cn('flex flex-wrap items-center justify-between gap-x-4 gap-y-2 px-3.5 py-2.5', className)}>
      {children}
    </div>
  )
}

export function SettingEmpty({
  icon,
  title,
  action,
  className,
}: {
  icon?: ReactNode
  title: string
  action?: ReactNode
  className?: string
}) {
  return (
    <div className={cn('flex flex-col items-center justify-center gap-2.5 px-4 py-8 text-center', className)} data-animated-trigger>
      {icon && (
        <span className="flex size-9 items-center justify-center rounded-lg bg-muted/80 text-muted-foreground [&_.animated-glyph]:size-4 [&_svg]:size-4">
          {icon}
        </span>
      )}
      <p className="text-[13px] text-muted-foreground">{title}</p>
      {action}
    </div>
  )
}

const statusToneStyles = {
  active: 'bg-primary/10 text-foreground',
  muted: 'bg-muted text-muted-foreground',
  warning: 'bg-amber-500/12 text-amber-700 dark:text-amber-300',
  error: 'bg-destructive/12 text-destructive',
} as const

export function StatusBadge({
  tone = 'muted',
  className,
  children,
}: {
  tone?: keyof typeof statusToneStyles
  className?: string
  children: ReactNode
}) {
  return (
    <span
      className={cn(
        'inline-flex shrink-0 items-center rounded-md px-1.5 py-0.5 text-[11px] leading-tight font-medium',
        statusToneStyles[tone],
        className,
      )}
    >
      {children}
    </span>
  )
}

export function ConfirmDialog({
  open,
  title,
  description,
  confirmLabel,
  cancelLabel = '取消',
  destructive = true,
  busy = false,
  error,
  onOpenChange,
  onConfirm,
  onCloseAutoFocus,
}: {
  open: boolean
  title: string
  description?: string
  confirmLabel: string
  cancelLabel?: string
  destructive?: boolean
  busy?: boolean
  error?: string
  onOpenChange: (open: boolean) => void
  onConfirm: () => void
  onCloseAutoFocus?: (event: Event) => void
}) {
  return (
    <Dialog open={open} onOpenChange={next => { if (!next && !busy) onOpenChange(false) }}>
      {open && (
        <DialogContent onCloseAutoFocus={onCloseAutoFocus}>
          <DialogHeader>
            <DialogTitle>{title}</DialogTitle>
            {description && <DialogDescription>{description}</DialogDescription>}
          </DialogHeader>
          {error && <FieldError>{error}</FieldError>}
          <DialogFooter>
            <Button type="button" variant="outline" disabled={busy} onClick={() => onOpenChange(false)}>
              {cancelLabel}
            </Button>
            <Button
              type="button"
              variant={destructive ? 'destructive' : 'default'}
              disabled={busy}
              onClick={onConfirm}
            >
              {confirmLabel}
            </Button>
          </DialogFooter>
        </DialogContent>
      )}
    </Dialog>
  )
}
