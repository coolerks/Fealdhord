import * as Service from '../../bindings/github.com/coolerks/Fealdhord/internal/settings/service'
import type { Preferences, Snapshot } from '../../bindings/github.com/coolerks/Fealdhord/internal/settings/models'
import { unwrap } from './api'
export type { Preferences, Snapshot }

export const preferencesAPI = {
  async get(): Promise<Snapshot> { return unwrap(await Service.Get()) },
  async update(preferences: Preferences, revision: number): Promise<Snapshot> {
    return unwrap(await Service.Update({ preferences, revision }))
  },
}
