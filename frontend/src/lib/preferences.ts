import * as Service from '../../bindings/github.com/coolerks/Fealdhord/internal/settings/service'
import type { Preferences as NativePreferences, Snapshot as NativeSnapshot } from '../../bindings/github.com/coolerks/Fealdhord/internal/settings/models'
import { unwrap } from './api'
export type Preferences = Omit<NativePreferences, 'reader_toolbar_pinned'> & { reader_toolbar_pinned: string[] }
export type Snapshot = Omit<NativeSnapshot, 'preferences'> & { preferences: Preferences }

function normalizeSnapshot(snapshot: NativeSnapshot): Snapshot {
  const preferences = snapshot.preferences
  return {
    ...snapshot,
    preferences: { ...preferences, reader_toolbar_pinned: preferences.reader_toolbar_pinned ?? ['refresh', 'favorite', 'translate', 'ai'] },
  }
}

export const preferencesAPI = {
  async get(): Promise<Snapshot> { return normalizeSnapshot(unwrap(await Service.Get())) },
  async update(preferences: Preferences, revision: number): Promise<Snapshot> {
    return normalizeSnapshot(unwrap(await Service.Update({ preferences, revision })))
  },
}
