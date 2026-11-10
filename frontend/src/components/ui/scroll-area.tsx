import type { ComponentProps, ReactNode } from 'react'
import { ScrollArea as ScrollAreaPrimitive } from 'radix-ui'

export function ScrollArea({ children, viewportProps }: {
  children: ReactNode
  viewportProps: ComponentProps<typeof ScrollAreaPrimitive.Viewport> & { 'data-density'?: string }
}) {
  return (
    <ScrollAreaPrimitive.Root className="scroll-area" type="auto">
      <ScrollAreaPrimitive.Viewport {...viewportProps}>
        {children}
      </ScrollAreaPrimitive.Viewport>
      <ScrollAreaPrimitive.Scrollbar className="scroll-area-bar" orientation="vertical">
        <ScrollAreaPrimitive.Thumb className="scroll-area-thumb" />
      </ScrollAreaPrimitive.Scrollbar>
    </ScrollAreaPrimitive.Root>
  )
}
