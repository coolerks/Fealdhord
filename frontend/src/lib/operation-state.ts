import type { OperationSnapshot } from './api'

// 事件可在 Start 返回之前到达；同一操作只接受更新的快照。
export function mergeSnapshot(current: OperationSnapshot | null, next: OperationSnapshot): OperationSnapshot {
  if (current?.operation_id === next.operation_id && current.sequence >= next.sequence) return current
  return next
}
