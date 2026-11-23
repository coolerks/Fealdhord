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

it('内置与外部工具均提供进行中摘要，且下划线与点号命名一致识别为内置工具', () => {
  expect(getToolMeta('article.list').runningSummary).toBe('获取文章列表中')
  expect(getToolMeta('article_list').runningSummary).toBe('获取文章列表中')
  expect(getToolMeta('article.search').runningSummary).toBe('搜索文章中')
  expect(getToolMeta('article.get').runningSummary).toBe('读取文章中')
  expect(getToolMeta('article.get_metadata').runningSummary).toBe('读取文章中')
  expect(getToolMeta('notification.send').runningSummary).toBe('发送通知中')
  expect(getToolMeta('github.create_issue').runningSummary).toBe('调用 github.create_issue 工具中')

  for (const name of [
    'feed_list', 'feed_get', 'feed_refresh', 'feed_refresh_status',
    'category_list', 'view_get_counts', 'opml_import', 'app_get_settings',
    'schedule_create', 'notification_channel_list', 'notification_send',
    'article_mark_read', 'article_favorite', 'article_ai_evaluate',
  ]) {
    const meta = getToolMeta(name)
    expect(meta.builtin, name).toBe(true)
    expect(meta.runningSummary, name).toBeTruthy()
  }

  expect(summarizeToolActivity(['feed_list', 'article_list', 'feed_refresh'])).toEqual({
    text: '查看了文章列表，查看了订阅，刷新了订阅',
    icon: 'layers',
  })
})
