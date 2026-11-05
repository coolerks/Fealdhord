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
it('全部失败的 partial 也不能显示成功',()=>{
  useRefresh.setState({snapshot:{...running,status:'partial',completed:2,results:[{feed_id:'a',status:'failed',message:'连接失败'},{feed_id:'b',status:'failed',message:'格式错误'}]}})
  render(<RefreshStatus/>);expect(screen.getByRole('status')).toHaveTextContent('2 个订阅失败')
  expect(screen.queryByText('刷新完成')).not.toBeInTheDocument()
})

it('截断的事件明细不掩盖较早失败的累计数',()=>{
 useRefresh.setState({error:null,snapshot:{...running,status:'completed',completed:200,total:200,results:[],failed_count:5,success_count:195,cancelled_count:0,skipped_count:0,results_truncated:true}})
 render(<RefreshStatus/>);expect(screen.getByRole('status')).toHaveTextContent('5 个订阅失败')
})
