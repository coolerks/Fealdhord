import type { Preferences } from '@/lib/preferences'
import type { Article } from '@/lib/library'

export function preferencesFixture(patch: Partial<Preferences> = {}): Preferences {
  return { reader_open_mode:'feed',reader_auto_read:true,reader_font_family:'sans',selected_scope_id:'',remote_images:'subscriptions',media_cache_enabled:true,web_image_fallback:false,theme: 'system', article_view_mode: 'list', list_density: 'compact', list_thumbnail: 'none', masonry_image_mode: 'original_ratio', animation: 'full', follow_system_reduce_motion: true, sidebar_collapsed: false, sidebar_width: 248, list_width: 360, reader_pinned: true, reader_open: true, selected_scope: 'all', selected_article_id: '', scroll_anchors: {}, reader_font_size: 17, reader_line_height: 1.7, reader_width: 720, ...patch }
}
// 只供测试使用，生产模块不得导入。
export function articleFixtures(count: number): Article[] {
  return Array.from({ length: count }, (_, index) => ({ id: `a-${index}`, title: `测试文章 ${index}`, feedTitle: '测试订阅', publishedAt: new Date(Date.UTC(2026, 10, 4) - index * 60000).toISOString(), summary: '用于验证图文、阅读顺序、筛选和虚拟化的测试摘要。', contentText: '这是一段用于组件测试的正文。\n\n第二段保留段落顺序。', isRead: index % 2 === 0, isFavorite: index % 3 === 0, ...(index % 3 === 0 ? { cover: { url: `/media/test-${index}`, width: 600, height: index % 2 ? 700 : 400, alt: '测试首图' } } : {}) }))
}
