import { CopyInlineImage } from '../../bindings/github.com/coolerks/Fealdhord/internal/media/api'
import { notify } from './notifications'

const inlinePath = /^\/media\/inline\/([a-f0-9]{64})$/

export function readerImageID(image: HTMLImageElement) {
  return inlinePath.exec(image.getAttribute('src') ?? '')?.[1]
}

export async function copyReaderImage(id: string) {
  try {
    const result = await CopyInlineImage(id)
    if (!result.ok) throw new Error(result.error?.message || '无法复制图片，请重试。')
    await notify('图片已复制')
  } catch (error) {
    await notify(error instanceof Error ? error.message : '无法复制图片，请重试。', undefined, true)
  }
}
