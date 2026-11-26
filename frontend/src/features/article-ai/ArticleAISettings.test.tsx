import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { expect, it, vi } from 'vitest'
import { TooltipProvider } from '@/components/ui/tooltip'
import { ArticleAISettings } from './ArticleAISettings'

const api = vi.hoisted(() => ({ settings: vi.fn(), save: vi.fn() }))
vi.mock('../../../bindings/github.com/coolerks/Fealdhord/internal/articleai/api', () => ({
  Evaluate: vi.fn(), Settings: api.settings, Save: api.save,
}))

it('文章摘要设置可独立修改两个阈值，拒绝空值', async () => {
  const initial = {
    classification: false, featured: false, prompt: '', revision: 1,
    requestsPerHour: 100, tokensPerHour: 200000, minimumImportance: 0.7, minimumRelevance: 0.6,
    autoTranslate: false, autoTranslateScope: 'summary', translateOnOpen: false, translationTargetLang: 'zh-CN',
    translationPrompt: '', summary: { onOpen: false, hanLimit: 600, englishLimit: 350 },
  }
  api.settings.mockResolvedValue(initial)
  api.save.mockImplementation(async value => ({ ...value, revision: 2 }))
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  render(<QueryClientProvider client={client}><TooltipProvider><ArticleAISettings /></TooltipProvider></QueryClientProvider>)
  const han = await screen.findByRole('spinbutton', { name: '中文短文阈值（字）' })
  const english = screen.getByRole('spinbutton', { name: '英文短文阈值（词）' })
  const automatic = screen.getByRole('switch', { name: '打开文章时自动生成摘要' })
  const batch = screen.getByRole('spinbutton', { name: '单批正文上限（KiB）' })
  expect(han).toHaveValue(600)
  expect(english).toHaveValue(350)
  expect(automatic).not.toBeChecked()
  expect(batch).toHaveValue(null)
  await userEvent.clear(han)
  for (const button of screen.getAllByRole('button', { name: '保存文章 AI 设置' })) {
    expect(button).toBeDisabled()
  }
  await userEvent.type(han, '900')
  await userEvent.clear(english)
  await userEvent.type(english, '450')
  await userEvent.click(automatic)
  await userEvent.type(batch, '1025')
  expect(screen.getAllByRole('button', { name: '保存文章 AI 设置' })[0]).toBeDisabled()
  await userEvent.clear(batch)
  await userEvent.type(batch, '32')
  await userEvent.click(screen.getAllByRole('button', { name: '保存文章 AI 设置' })[0])
  await waitFor(() => expect(api.save).toHaveBeenCalledExactlyOnceWith({
    ...initial, translationBatchKiB: 32, summary: { onOpen: true, hanLimit: 900, englishLimit: 450 },
  }))
  await waitFor(() => expect(screen.queryByRole('button', { name: '保存文章 AI 设置' })).toBeNull())
  await userEvent.clear(batch)
  await userEvent.click(screen.getAllByRole('button', { name: '保存文章 AI 设置' })[0])
  await waitFor(() => expect(api.save).toHaveBeenLastCalledWith(expect.objectContaining({ translationBatchKiB: 0, revision: 2 })))
})
