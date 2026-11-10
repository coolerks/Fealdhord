import type { ComponentProps } from 'react'
import { ToggleGroup } from '@/components/ui/toggle-group'
import { cn } from '@/lib/utils'

type Props = Omit<Extract<ComponentProps<typeof ToggleGroup>, { type: 'single' }>, 'type' | 'variant'>

export function SegmentedControl({ className, size = 'sm', ...props }: Props) {
  return <ToggleGroup {...props} type="single" size={size} data-slot="segmented-control"
    className={cn('w-fit! max-w-full flex-wrap self-start bg-muted p-[3px]', className)} spacing={1}
  />
}
