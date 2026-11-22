import type { AnimatedIconName } from '@/components/icon-catalog'

export type BuiltinToolGroup = {
  summary: string
  icon: AnimatedIconName
  priority: number
}

export type ToolMeta = {
  label: string
  summary: string
  icon: AnimatedIconName
  priority: number
  builtin: boolean
}

const groups = {
  askUser: { summary: '向用户确认了需求', icon: 'circle-help', priority: 5 },
  searchArticles: { summary: '搜索了文章', icon: 'search', priority: 10 },
  listArticles: { summary: '查看了文章列表', icon: 'layers', priority: 15 },
  readArticles: { summary: '读取了文章', icon: 'book-text', priority: 20 },
  extractArticles: { summary: '提取了文章正文', icon: 'scan-text', priority: 25 },
  updateReadState: { summary: '更新了阅读状态', icon: 'check-check', priority: 30 },
  updateFavorites: { summary: '更新了文章收藏', icon: 'bookmark', priority: 32 },
  updateArticleCategories: { summary: '调整了文章分类', icon: 'folder-tree', priority: 35 },
  evaluateArticles: { summary: '评估了文章', icon: 'sparkles', priority: 40 },
  viewFeeds: { summary: '查看了订阅', icon: 'radio-tower', priority: 45 },
  manageFeeds: { summary: '更新了订阅', icon: 'radio-tower', priority: 48 },
  refreshFeeds: { summary: '刷新了订阅', icon: 'refresh-cw', priority: 50 },
  viewCategories: { summary: '查看了分类', icon: 'folder-tree', priority: 55 },
  manageCategories: { summary: '更新了分类', icon: 'folder-tree', priority: 58 },
  viewStats: { summary: '查看了视图统计', icon: 'layout-grid', priority: 60 },
  handleOPML: { summary: '处理了 OPML 数据', icon: 'database-backup', priority: 65 },
  viewSettings: { summary: '查看了应用设置', icon: 'sliders-horizontal', priority: 70 },
  updateSettings: { summary: '更新了应用设置', icon: 'sliders-horizontal', priority: 72 },
  viewDiagnostics: { summary: '查看了诊断状态', icon: 'activity', priority: 75 },
  viewSchedules: { summary: '查看了定时任务', icon: 'clock', priority: 80 },
  manageSchedules: { summary: '管理了定时任务', icon: 'clock', priority: 82 },
  findTools: { summary: '查找了可用工具', icon: 'blocks', priority: 85 },
  runBuiltinTool: { summary: '调用了内置工具', icon: 'blocks', priority: 86 },
  viewNotifications: { summary: '查看了通知配置', icon: 'bell', priority: 95 },
  sendNotification: { summary: '发送了通知', icon: 'send', priority: 100 },
} satisfies Record<string, BuiltinToolGroup>

const builtinTools: Record<string, { label: string; group: BuiltinToolGroup }> = {
  // 用户澄清
  AskUserQuestion: { label: '向用户提问', group: groups.askUser },
  // 文章查询与阅读
  'article.search': { label: '搜索文章', group: groups.searchArticles },
  article_search: { label: '搜索文章', group: groups.searchArticles },
  library_search: { label: '搜索本地文章', group: groups.searchArticles },
  'article.list': { label: '列出文章', group: groups.listArticles },
  article_list: { label: '列出文章', group: groups.listArticles },
  'article.get': { label: '读取文章正文', group: groups.readArticles },
  article_get: { label: '读取文章正文', group: groups.readArticles },
  article_read: { label: '读取文章正文', group: groups.readArticles },
  read_article: { label: '读取授权文章', group: groups.readArticles },
  'article.get_metadata': { label: '读取文章摘要', group: groups.readArticles },
  article_get_metadata: { label: '读取文章摘要', group: groups.readArticles },
  'article.extract': { label: '提取文章正文', group: groups.extractArticles },
  article_extract: { label: '提取文章正文', group: groups.extractArticles },
  // 文章状态与分类
  'article.mark_read': { label: '标为已读', group: groups.updateReadState },
  'article.mark_unread': { label: '标为未读', group: groups.updateReadState },
  'article.favorite': { label: '收藏文章', group: groups.updateFavorites },
  'article.unfavorite': { label: '取消收藏', group: groups.updateFavorites },
  'article.add_category': { label: '将文章加入分类', group: groups.updateArticleCategories },
  'article.remove_category': { label: '将文章移出分类', group: groups.updateArticleCategories },
  'category.article_memberships': { label: '查看文章分类关系', group: groups.updateArticleCategories },
  // 文章 AI
  'article_ai.evaluate': { label: '评估文章', group: groups.evaluateArticles },
  'article_ai.preview': { label: '试跑文章评估', group: groups.evaluateArticles },
  // 订阅
  'feed.list': { label: '列出订阅', group: groups.viewFeeds },
  'feed.get': { label: '读取订阅详情', group: groups.viewFeeds },
  'feed.discover': { label: '发现订阅源', group: groups.viewFeeds },
  'feed.add': { label: '添加订阅', group: groups.manageFeeds },
  'feed.update': { label: '更新订阅', group: groups.manageFeeds },
  'feed.remove': { label: '归档订阅', group: groups.manageFeeds },
  'feed.restore': { label: '恢复订阅', group: groups.manageFeeds },
  'feed.refresh': { label: '刷新订阅', group: groups.refreshFeeds },
  'feed.refresh_all': { label: '刷新全部订阅', group: groups.refreshFeeds },
  'feed.refresh_status': { label: '查看刷新进度', group: groups.refreshFeeds },
  'feed.cancel_refresh': { label: '取消刷新任务', group: groups.refreshFeeds },
  // 分类
  'category.list': { label: '列出分类', group: groups.viewCategories },
  'category.get': { label: '读取分类详情', group: groups.viewCategories },
  'category.preview_remove': { label: '预览移除分类', group: groups.viewCategories },
  'category.create': { label: '创建分类', group: groups.manageCategories },
  'category.update': { label: '更新分类', group: groups.manageCategories },
  'category.remove': { label: '移除分类', group: groups.manageCategories },
  'category.add_feed': { label: '将订阅加入分类', group: groups.manageCategories },
  'category.remove_feed': { label: '将订阅移出分类', group: groups.manageCategories },
  // 视图与统计
  'view.list': { label: '列出内置视图', group: groups.viewStats },
  'view.get_counts': { label: '获取文章计数', group: groups.viewStats },
  // OPML
  'opml.preview_import': { label: '预览导入 OPML', group: groups.handleOPML },
  'opml.import': { label: '导入 OPML', group: groups.handleOPML },
  'opml.export': { label: '导出 OPML', group: groups.handleOPML },
  // 应用设置与诊断
  'app.get_settings': { label: '查看阅读偏好', group: groups.viewSettings },
  'app.get_refresh_settings': { label: '查看刷新间隔', group: groups.viewSettings },
  'app.get_version': { label: '查看应用版本', group: groups.viewSettings },
  'app.update_reading_settings': { label: '修改阅读偏好', group: groups.updateSettings },
  'app.update_refresh_settings': { label: '修改刷新间隔', group: groups.updateSettings },
  'app.get_diagnostics': { label: '查看刷新诊断', group: groups.viewDiagnostics },
  // 定时任务
  'schedule.list': { label: '列出定时任务', group: groups.viewSchedules },
  'schedule.get': { label: '读取定时任务', group: groups.viewSchedules },
  'schedule.create': { label: '创建定时任务', group: groups.manageSchedules },
  'schedule.update': { label: '更新定时任务', group: groups.manageSchedules },
  'schedule.pause': { label: '暂停定时任务', group: groups.manageSchedules },
  'schedule.resume': { label: '恢复定时任务', group: groups.manageSchedules },
  'schedule.delete': { label: '删除定时任务', group: groups.manageSchedules },
  // 元工具
  find_app_tools: { label: '查找可用工具', group: groups.findTools },
  run_app_tool: { label: '调用内置工具', group: groups.runBuiltinTool },
  // 通知（最低优先级）
  'notification.channel.list': { label: '列出通知渠道', group: groups.viewNotifications },
  'notification.rule.list': { label: '查看推送规则', group: groups.viewNotifications },
  'notification.send': { label: '发送通知', group: groups.sendNotification },
}

const EXTERNAL_PRIORITY = 90

export function getToolMeta(name: string): ToolMeta {
  if (Object.hasOwn(builtinTools, name)) {
    const entry = builtinTools[name]
    return {
      label: entry.label,
      summary: entry.group.summary,
      icon: entry.group.icon,
      priority: entry.group.priority,
      builtin: true,
    }
  }
  return {
    label: name,
    summary: `调用了 ${name} 工具`,
    icon: 'wrench',
    priority: EXTERNAL_PRIORITY,
    builtin: false,
  }
}

export function summarizeToolActivity(toolNames: readonly string[]): { text: string; icon: AnimatedIconName } {
  const builtinBySummary = new Map<string, { summary: string; icon: AnimatedIconName; priority: number }>()
  const externalNames = new Set<string>()

  for (const name of toolNames) {
    const meta = getToolMeta(name)
    if (meta.builtin) {
      if (!builtinBySummary.has(meta.summary)) {
        builtinBySummary.set(meta.summary, {
          summary: meta.summary,
          icon: meta.icon,
          priority: meta.priority,
        })
      }
    } else {
      externalNames.add(name)
    }
  }

  const entries = Array.from(builtinBySummary.values())
  if (externalNames.size === 1) {
    const [single] = externalNames
    entries.push({ summary: `调用了 ${single} 工具`, icon: 'wrench', priority: EXTERNAL_PRIORITY })
  } else if (externalNames.size > 1) {
    entries.push({ summary: '调用了多个外部工具', icon: 'wrench', priority: EXTERNAL_PRIORITY })
  }

  if (entries.length === 0) {
    return { text: '进行了思考', icon: 'sparkles' }
  }

  entries.sort((a, b) => a.priority - b.priority)
  return {
    text: entries.map(item => item.summary).join('，'),
    icon: entries[0].icon,
  }
}
