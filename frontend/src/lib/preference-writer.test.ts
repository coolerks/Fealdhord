import { describe, expect, it, vi } from 'vitest'
import { PreferenceWriter } from './preference-writer'
import type { Snapshot } from './preferences'
import { preferencesFixture } from '@/test/fixtures'
import { effectiveMotion } from './environment'
import { filterArticles } from './library'
import { articleFixtures } from '@/test/fixtures'

function deferred<T>() { let resolve!: (value: T) => void; let reject!: (error: Error) => void; const promise = new Promise<T>((yes, no) => { resolve = yes; reject = no }); return { promise, resolve, reject } }

describe('偏好与共享数据规则', () => {
  it('串行保存并合并连续输入，使用上次真实 revision', async () => {
    const first = deferred<Snapshot>(); const saved = vi.fn(); const failed = vi.fn()
    const update = vi.fn().mockReturnValueOnce(first.promise).mockImplementation(async (preferences, revision) => ({ preferences, revision: revision + 1 }))
    const writer = new PreferenceWriter(5, update, saved, failed)
    writer.enqueue(preferencesFixture({ theme: 'light' }), 1)
    writer.enqueue(preferencesFixture({ theme: 'dark' }), 2)
    writer.enqueue(preferencesFixture({ theme: 'dark', article_view_mode: 'masonry' }), 3)
    expect(update).toHaveBeenCalledTimes(1)
    first.resolve({ preferences: preferencesFixture({ theme: 'light' }), revision: 6 })
    await first.promise; await Promise.resolve(); await Promise.resolve()
    expect(update).toHaveBeenCalledTimes(2)
    expect(update.mock.calls[1][0].article_view_mode).toBe('masonry')
    expect(update.mock.calls[1][1]).toBe(6)
    expect(saved).toHaveBeenLastCalledWith(expect.objectContaining({ revision: 7 }), 3)
    expect(failed).not.toHaveBeenCalled()
  })

  it('失败不假装保存成功，后续重试仍可工作', async () => {
    const failed = vi.fn(); const saved = vi.fn()
    const update = vi.fn().mockRejectedValueOnce(new Error('数据库忙')).mockResolvedValue({ preferences: preferencesFixture(), revision: 1 })
    const writer = new PreferenceWriter(0, update, saved, failed)
    writer.enqueue(preferencesFixture(), 1)
    await Promise.resolve(); await Promise.resolve()
    expect(failed).toHaveBeenCalledOnce(); expect(saved).not.toHaveBeenCalled()
    writer.enqueue(preferencesFixture(), 1)
    await Promise.resolve(); await Promise.resolve()
    expect(saved).toHaveBeenCalledOnce()
  })

  it('Off 优先，系统减少动态与显式 Subtle 使用同一规则', () => {
    expect(effectiveMotion(preferencesFixture({ animation: 'off' }), false)).toBe('off')
    expect(effectiveMotion(preferencesFixture(), true)).toBe('subtle')
    expect(effectiveMotion(preferencesFixture({ follow_system_reduce_motion: false }), true)).toBe('full')
    expect(effectiveMotion(preferencesFixture({ animation: 'subtle' }), false)).toBe('subtle')
  })

  it('两种投影前共用相同过滤与稳定排序，不修改源数据', () => {
    const articles = articleFixtures(6)
    expect(filterArticles(articles, 'unread', '', 'newest').map(article => article.id)).toEqual(['a-1', 'a-3', 'a-5'])
    expect(filterArticles(articles, 'favorites', '', 'oldest').map(article => article.id)).toEqual(['a-3', 'a-0'])
    expect(filterArticles(articles, 'all', '文章 4', 'newest')[0].id).toBe('a-4')
    expect(filterArticles(articles, 'featured', '', 'newest')).toEqual([])
    expect(articles[0].id).toBe('a-0')
    const precision=articleFixtures(2).map((article,index)=>({...article,publishedAt:index?'2026-11-04T00:00:00.001Z':'2026-11-04T00:00:00Z'}))
    expect(filterArticles(precision,'all','','newest')[0].id).toBe('a-1')
  })
})
