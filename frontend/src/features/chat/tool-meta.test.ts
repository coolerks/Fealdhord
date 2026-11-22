import { expect, it } from 'vitest'
import { getToolMeta, summarizeToolActivity } from './tool-meta'

it('合并同类内置工具摘要、按优先级排序并取最高优先级图标', () => {
  const result = summarizeToolActivity([
    'notification.send',
    'article.get',
    'article.get_metadata',
    'article.search',
  ])
  expect(result).toEqual({
    text: '搜索了文章，读取了文章，发送了通知',
    icon: 'search',
  })
})

it('单个外部工具展示名称，多个外部工具合并为统一摘要并固定 wrench 图标', () => {
  expect(summarizeToolActivity(['github.create_issue'])).toEqual({
    text: '调用了 github.create_issue 工具',
    icon: 'wrench',
  })
  expect(summarizeToolActivity(['ext.a', 'ext.b', 'ext.a'])).toEqual({
    text: '调用了多个外部工具',
    icon: 'wrench',
  })
})

it('同一摘要的内置工具保持相同动画图标', () => {
  expect(getToolMeta('article.get').icon).toBe(getToolMeta('article.get_metadata').icon)
  expect(getToolMeta('article.mark_read').icon).toBe(getToolMeta('article.mark_unread').icon)
  expect(getToolMeta('schedule.create').icon).toBe(getToolMeta('schedule.delete').icon)
})
