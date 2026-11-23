import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, expect, it, vi } from 'vitest'
import { preferencesFixture } from '@/test/fixtures'
import { useWorkspace } from '@/stores/workspace'

const api = vi.hoisted(() => ({
  cacheSize: vi.fn<() => Promise<number>>(),
  clearCache: vi.fn<() => Promise<void>>(),
}))
vi.mock('@/lib/rss-api', () => ({ rssAPI: api }))
vi.mock('@/lib/notifications', () => ({ notify: vi.fn() }))

import { MediaPreferences } from './MediaPreferences'

function setup() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } })
  return render(
    <QueryClientProvider client={client}>
      <MediaPreferences />
    </QueryClientProvider>,
  )
}

beforeEach(() => {
  vi.resetAllMocks()
  useWorkspace.setState({ preferences: preferencesFixture(), patch: vi.fn() })
})

it('异步计算已缓存图片大小：未完成时显示计算中，完成后展示大小并在清理后更新', async () => {
  let resolveSize!: (value: number) => void
  api.cacheSize.mockImplementationOnce(() => new Promise<number>(resolve => { resolveSize = resolve }))
  api.clearCache.mockResolvedValue(undefined)

  setup()
  expect(screen.getByText('计算中')).toBeInTheDocument()

  resolveSize(3 * 1024 * 1024 + 512 * 1024)
  await waitFor(() => expect(screen.getByText('3.5 MB')).toBeInTheDocument())

  api.cacheSize.mockResolvedValueOnce(0)
  await userEvent.click(screen.getByRole('button', { name: '清理图片缓存' }))
  await waitFor(() => expect(screen.getByText('0 B')).toBeInTheDocument())
})
