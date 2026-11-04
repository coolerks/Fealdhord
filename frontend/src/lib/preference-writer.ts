import type { Preferences, Snapshot } from './preferences'

// 串行写入最新状态；前一请求未结束时只保留最后一次用户编辑。
export class PreferenceWriter {
  private latest: { preferences: Preferences; version: number } | null = null
  private running = false
  private revision: number
  constructor(
    revision: number,
    private readonly update: (preferences: Preferences, revision: number) => Promise<Snapshot>,
    private readonly saved: (snapshot: Snapshot, version: number) => void,
    private readonly failed: (error: unknown) => void,
  ) { this.revision = revision }

  enqueue(preferences: Preferences, version: number) {
    this.latest = { preferences, version }
    if (!this.running) void this.drain()
  }

  private async drain() {
    this.running = true
    try {
      while (this.latest) {
        const next = this.latest
        this.latest = null
        const snapshot = await this.update(next.preferences, this.revision)
        this.revision = snapshot.revision
        this.saved(snapshot, next.version)
      }
    } catch (error) {
      this.latest = null
      this.failed(error)
    } finally { this.running = false }
  }
}
