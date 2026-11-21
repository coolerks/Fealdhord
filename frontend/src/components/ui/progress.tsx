import * as React from "react"
import { Progress as ProgressPrimitive } from "radix-ui"
import { cn } from "@/lib/utils"

function Progress({
  className,
  value = 0,
  max = 100,
  ...props
}: React.ComponentProps<typeof ProgressPrimitive.Root>) {
  const safeMax = max > 0 ? max : 100
  const safeValue = Math.min(safeMax, Math.max(0, value ?? 0))
  const percent = (safeValue / safeMax) * 100

  return (
    <ProgressPrimitive.Root
      data-slot="progress"
      value={safeValue}
      max={safeMax}
      className={cn(
        "relative h-1.5 w-full overflow-hidden rounded-full bg-muted",
        className
      )}
      {...props}
    >
      <ProgressPrimitive.Indicator
        data-slot="progress-indicator"
        className="h-full w-full flex-1 rounded-full bg-primary transition-transform duration-200 ease-out"
        style={{ transform: `translateX(-${100 - percent}%)` }}
      />
    </ProgressPrimitive.Root>
  )
}

export { Progress }
