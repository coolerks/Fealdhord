import type { ComponentProps } from 'react'
import { ToggleGroup } from '@/components/ui/toggle-group'
import { cn } from '@/lib/utils'

type Props = Omit<Extract<ComponentProps<typeof ToggleGroup>, { type: 'single' }>, 'type' | 'variant'>

export function SegmentedControl({ className, ...props }: Props) {
  return <ToggleGroup {...props} type="single" data-slot="segmented-control"
    className={cn('max-w-full flex-wrap rounded-lg bg-muted p-0.5', className)} spacing={1}
  />
}
