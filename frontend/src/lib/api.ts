import { Events } from '@wailsio/runtime'
import '../../bindings/events'
import * as Service from '../../bindings/github.com/coolerks/Fealdhord/internal/app/bootstrapservice'
import type { AppError, Info, OperationSnapshot } from '../../bindings/github.com/coolerks/Fealdhord/internal/app/models'

export type { Info, OperationSnapshot }

export class APIError extends Error {
  readonly code: string
  constructor(error: AppError) {
    super(error.message)
    this.name = 'APIError'
    this.code = error.code
  }
}

export function unwrap<T>(result: { ok: boolean; data?: T | null; error?: AppError | null }): T {
  if (!result.ok) {
    throw new APIError(result.error ?? { code: 'TransportError', message: '无法连接桌面服务。', retryable: true })
  }
  if (result.data == null) throw new Error('桌面服务返回了不完整的数据。')
  return result.data
}

export const desktopAPI = {
  async info(): Promise<Info> { return unwrap(await Service.GetInfo()) },
  async start(): Promise<OperationSnapshot> { return unwrap(await Service.StartProbe()) },
  async snapshot(operationID: string): Promise<OperationSnapshot> {
    return unwrap(await Service.GetOperation({ operation_id: operationID }))
  },
  async cancel(operationID: string): Promise<OperationSnapshot> {
    return unwrap(await Service.Cancel({ operation_id: operationID }))
  },
  subscribe(listener: (snapshot: OperationSnapshot) => void): () => void {
    return Events.On('fealdhord:event', event => {
      if (event.data.schema_version === 1) listener(event.data.payload)
    })
  },
}
