import { useEffect, useState } from 'react'
import { QueryClient, QueryClientProvider, useQuery } from '@tanstack/react-query'
import { AlertCircle } from 'lucide-react'
import { TooltipProvider } from '@/components/ui/tooltip'
import { Skeleton } from '@/components/ui/skeleton'
import { Empty, EmptyContent, EmptyDescription, EmptyHeader, EmptyMedia, EmptyTitle } from '@/components/ui/empty'
import { Button } from '@/components/ui/button'
import { preferencesAPI } from '@/lib/preferences'
import { useWorkspace } from '@/stores/workspace'
import { Shell } from '@/app/Shell'

function PreferencesGate() {
  const preferences = useWorkspace(state => state.preferences)
  const initialize = useWorkspace(state => state.initialize)
  const query = useQuery({ queryKey: ['settings', 'preferences'], queryFn: preferencesAPI.get, staleTime: Infinity, retry: false })
  useEffect(() => { if (query.data && !useWorkspace.getState().preferences) initialize(query.data) }, [query.data, initialize])
  if (query.isError) return <main className="app-loading"><Empty><EmptyHeader><EmptyMedia variant="icon"><AlertCircle /></EmptyMedia><EmptyTitle>暂时无法读取本地设置</EmptyTitle><EmptyDescription>请检查桌面服务和数据目录权限，然后重试。</EmptyDescription></EmptyHeader><EmptyContent><Button variant="outline" onClick={() => { void query.refetch() }}>重新载入</Button></EmptyContent></Empty></main>
  if (!preferences) return <main className="app-loading" aria-label="正在恢复阅读空间"><Skeleton className="size-10" /><Skeleton className="h-4 w-36" /><span>正在恢复阅读空间…</span></main>
  return <Shell />
}

export default function App({ client }: { client?: QueryClient }) {
  const [defaultClient] = useState(() => new QueryClient({ defaultOptions: { queries: { refetchOnWindowFocus: false }, mutations: { retry: false } } }))
  return <QueryClientProvider client={client ?? defaultClient}><TooltipProvider delayDuration={450}><PreferencesGate /></TooltipProvider></QueryClientProvider>
}
