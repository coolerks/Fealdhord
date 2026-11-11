import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, expect, it, vi } from 'vitest'
import { useRefresh } from '@/stores/refresh'
const api=vi.hoisted(()=>({cancel:vi.fn()}))
vi.mock('@/lib/rss-api',()=>({rssAPI:api,feedsQueryKey:['rss','feeds']}))
import { RefreshStatus } from './RefreshStatus'
const running={generation:1,sequence:1,operation_id:'one',status:'running',completed:1,total:2,results:[]}
beforeEach(()=>{vi.resetAllMocks();useRefresh.setState({snapshot:running,error:null,dismissedGeneration:-1,pending:false})})
it('取消当前操作并展示取消中的真实快照',async()=>{
  api.cancel.mockResolvedValue({...running,status:'cancelling',sequence:2})
  render(<RefreshStatus/>);await userEvent.click(screen.getByRole('button',{name:'取消刷新'}))
  await waitFor(()=>expect(api.cancel).toHaveBeenCalledWith('one'))
  expect(screen.getByRole('button',{name:'取消刷新'})).toBeDisabled()
  expect(screen.getByRole('status')).toHaveTextContent('正在取消')
})
it('终态退出行内进度条',()=>{
  useRefresh.setState({snapshot:{...running,status:'completed',completed:2}})
  render(<RefreshStatus/>)
  expect(screen.queryByRole('status')).not.toBeInTheDocument()
})
