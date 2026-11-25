const pad = (value: number) => String(value).padStart(2, '0')

/**
 * 格式化聊天消息时间戳：
 * - 无效或旧消息（<= 0）返回空字符串
 * - 今天：HH:mm
 * - 今年非今天：MM-DD HH:mm
 * - 跨年：YYYY-MM-DD HH:mm
 */
export function formatMessageTime(timestampMs: number | undefined | null, now = new Date()): string {
  if (!timestampMs || timestampMs <= 0) return ''
  const date = new Date(timestampMs)
  if (Number.isNaN(date.getTime())) return ''
  const time = `${pad(date.getHours())}:${pad(date.getMinutes())}`
  const sameYear = date.getFullYear() === now.getFullYear()
  const sameMonth = date.getMonth() === now.getMonth()
  const sameDay = date.getDate() === now.getDate()
  if (sameYear && sameMonth && sameDay) return time
  const monthDay = `${pad(date.getMonth() + 1)}-${pad(date.getDate())}`
  if (sameYear) return `${monthDay} ${time}`
  return `${date.getFullYear()}-${monthDay} ${time}`
}

export function formatMessageTimeTitle(timestampMs: number | undefined | null): string | undefined {
  if (!timestampMs || timestampMs <= 0) return undefined
  const date = new Date(timestampMs)
  if (Number.isNaN(date.getTime())) return undefined
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}:${pad(date.getSeconds())}`
}
