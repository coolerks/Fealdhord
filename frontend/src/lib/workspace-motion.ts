import type { effectiveMotion } from './environment'

export function animateWorkspaceEntry(
  surfaces: (Element | null | undefined)[],
  mode: ReturnType<typeof effectiveMotion>,
  offset: number,
) {
  if (mode === 'off') return

  const animations = surfaces.map(surface => surface?.animate?.([
    { opacity: 0, ...(mode === 'full' && { translate: `0 ${offset}px` }) },
    { opacity: 1, ...(mode === 'full' && { translate: '0 0' }) },
  ], { duration: mode === 'subtle' ? 100 : 220, easing: 'cubic-bezier(.22,1,.36,1)' }))

  return () => animations.forEach(animation => animation?.cancel())
}
