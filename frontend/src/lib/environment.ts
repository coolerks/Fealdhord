import { useCallback, useSyncExternalStore } from 'react'
import type { Preferences } from './preferences'

export function effectiveMotion(preferences: Preferences, systemReduced: boolean): 'full' | 'subtle' | 'off' {
  if (preferences.animation === 'off') return 'off'
  if (preferences.animation === 'subtle' || (preferences.follow_system_reduce_motion && systemReduced)) return 'subtle'
  return 'full'
}

export function useMediaQuery(query: string) {
  const subscribe = useCallback((notify: () => void) => {
    const media = window.matchMedia?.(query)
    media?.addEventListener('change', notify)
    return () => media?.removeEventListener('change', notify)
  }, [query])
  const snapshot = useCallback(() => window.matchMedia?.(query).matches ?? false, [query])
  return useSyncExternalStore(subscribe, snapshot, () => false)
}
