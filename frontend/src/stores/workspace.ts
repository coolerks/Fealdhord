import { create } from 'zustand'
import { preferencesAPI } from '@/lib/preferences'
import type { Preferences, Snapshot } from '@/lib/preferences'
import { PreferenceWriter } from '@/lib/preference-writer'

export type SettingsTab = 'appearance' | 'reader' | 'feeds' | 'ai' | 'about'
type Workspace = {
  preferences: Preferences | null
  revision: number
  version: number
  savedVersion: number
  saveError: string | null
  saveStatus: 'saved' | 'saving' | 'error'
  search: string
  sort: 'newest' | 'oldest'
  settingsTab: SettingsTab
  settingsOpen: boolean
  paletteOpen: boolean
  searchOpen: boolean
  aiOpen: boolean
  patch: (patch: Partial<Preferences>) => void
  initialize: (snapshot: Snapshot) => void
  retrySave: () => void
  showSettings: (tab?: SettingsTab) => void
}
let writer: PreferenceWriter | null = null
let settingsFocusReturn: HTMLElement | null = null
export function restoreSettingsFocus() {
  const target = settingsFocusReturn?.isConnected ? settingsFocusReturn : document.getElementById('open-settings')
  target?.focus()
  settingsFocusReturn = null
}

export const useWorkspace = create<Workspace>((set, get) => ({
  preferences: null, revision: 0, version: 0, savedVersion: 0, saveError: null, saveStatus: 'saved',
  search: '', sort: 'newest', settingsTab: 'appearance', settingsOpen: false, paletteOpen: false, searchOpen: false, aiOpen: false,
  initialize(snapshot) {
    writer = new PreferenceWriter(snapshot.revision, preferencesAPI.update,
      (saved, version) => set(state => ({ revision: saved.revision, savedVersion: version, saveStatus: state.version > version ? 'saving' : 'saved' })),
      error => set({ saveError: error instanceof Error ? error.message : '设置保存失败，请重试。', saveStatus: 'error' }),
    )
    set({ preferences: snapshot.preferences, revision: snapshot.revision, version: 0, savedVersion: 0, saveError: null, saveStatus: 'saved' })
  },
  patch(patch) {
    const state = get()
    if (!state.preferences) return
    const preferences = { ...state.preferences, ...patch }
    const version = state.version + 1
    set({ preferences, version, saveError: null, saveStatus: 'saving' })
    writer?.enqueue(preferences, version)
  },
  retrySave() {
    const { preferences, version } = get()
    if (preferences) { set({ saveError: null, saveStatus: 'saving' }); writer?.enqueue(preferences, version) }
  },
  showSettings(tab = 'appearance') { settingsFocusReturn = document.activeElement instanceof HTMLElement ? document.activeElement : null; set({ settingsOpen: true, settingsTab: tab }) },
}))
