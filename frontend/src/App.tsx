import { useEffect, useRef, useState } from 'react'
import { desktopAPI } from './lib/api'
import type { Info, OperationSnapshot } from './lib/api'
import { mergeSnapshot } from './lib/operation-state'

const statusLabels: Record<string, string> = { running: '正在检查', cancelled: '已取消', succeeded: '检查完成' }

export default function App() {
  const [info, setInfo] = useState<Info | null>(null)
  const [operation, setOperation] = useState<OperationSnapshot | null>(null)
  const [error, setError] = useState('')
  const [pending, setPending] = useState(false)
  const mounted = useRef(false)
  const currentID = useRef<string | null>(null)
  const starting = useRef(false)
  const earlyEvents = useRef(new Map<string, OperationSnapshot>())
  const busy = pending || operation?.status === 'running'

  useEffect(() => {
    mounted.current = true
    const unsubscribe = desktopAPI.subscribe(snapshot => {
      if (starting.current) {
        earlyEvents.current.set(snapshot.operation_id, mergeSnapshot(earlyEvents.current.get(snapshot.operation_id) ?? null, snapshot))
      } else if (currentID.current === snapshot.operation_id) {
        setOperation(current => mergeSnapshot(current, snapshot))
      }
    })
    desktopAPI.info().then(value => { if (mounted.current) setInfo(value) }).catch(() => {
      if (mounted.current) setError('桌面服务连接失败，请重新打开应用。')
    })
    return () => { mounted.current = false; unsubscribe() }
  }, [])

  // 快照恢复防止事件丢失令 UI 永远停留在工作中。
  useEffect(() => {
    if (operation?.status !== 'running') return
    let disposed = false
    const id = operation.operation_id
    const timer = window.setInterval(() => {
      desktopAPI.snapshot(id).then(snapshot => {
        if (!disposed && currentID.current === id) setOperation(current => mergeSnapshot(current, snapshot))
      }).catch(() => { if (!disposed) setError('检查状态暂时无法同步，请重试。') })
    }, 1000)
    return () => { disposed = true; window.clearInterval(timer) }
  }, [operation?.operation_id, operation?.status])

  async function start() {
    if (starting.current || busy) return
    starting.current = true
    earlyEvents.current.clear()
    setPending(true)
    setError('')
    try {
      const snapshot = await desktopAPI.start()
      const early = earlyEvents.current.get(snapshot.operation_id)
      currentID.current = snapshot.operation_id
      if (mounted.current) setOperation(mergeSnapshot(snapshot, early ?? snapshot))
    } catch (cause) {
      if (mounted.current) setError(cause instanceof Error ? cause.message : '检查启动失败。')
    } finally {
      starting.current = false
      earlyEvents.current.clear()
      if (mounted.current) setPending(false)
    }
  }

  async function cancel() {
    if (!operation || pending) return
    setPending(true)
    setError('')
    try {
      const snapshot = await desktopAPI.cancel(operation.operation_id)
      if (mounted.current) setOperation(current => mergeSnapshot(current, snapshot))
    } catch (cause) {
      if (mounted.current) setError(cause instanceof Error ? cause.message : '取消失败，请重试。')
    } finally {
      if (mounted.current) setPending(false)
    }
  }

  return (
    <main className="workspace">
      <header className="titlebar"><span className="brand-mark" aria-hidden="true">折</span><div><strong>折栈</strong><span className="wordmark">Fealdhord</span></div><span className="phase-label">工程初始化</span></header>
      <section className="welcome" aria-labelledby="welcome-title">
        <p className="eyebrow">本地优先 · 为专注阅读而生</p>
        <h1 id="welcome-title">给信息一个安静的位置。</h1>
        <p className="description">桌面工程已建立。订阅、文章和阅读界面将在接下来的阶段接入。</p>
        <p className="empty-note">当前没有模拟文章，也没有配置任何 AI 服务。</p>
      </section>
      <section className="diagnostics" aria-labelledby="diagnostics-title">
        <div className="section-heading"><div><h2 id="diagnostics-title">开发连接检查</h2><p>验证桌面调用、事件和取消，不访问网络。</p></div><span className={`connection ${info ? 'connected' : ''}`}>{info ? '桌面服务已连接' : '连接中'}</span></div>
        {info && <dl className="system-info"><div><dt>版本</dt><dd>{info.version}</dd></div><div><dt>运行平台</dt><dd>{info.platform} / {info.architecture}</dd></div><div><dt>桌面框架</dt><dd>Wails {info.wails_version}</dd></div></dl>}
        <div className="probe"><div className="progress-label"><span role="status">{operation ? statusLabels[operation.status] ?? operation.status : '尚未开始检查'}</span><span>{operation ? `${operation.completed} / ${operation.total}` : '约 5 秒'}</span></div><progress aria-label="连接检查进度" max={operation?.total ?? 20} value={operation?.completed ?? 0}/><div className="actions"><button className="primary" onClick={() => { void start() }} disabled={!info || busy}>开始检查</button><button onClick={() => { void cancel() }} disabled={operation?.status !== 'running' || pending}>取消检查</button></div></div>
        {error && <p role="alert" className="error">{error}</p>}
      </section>
      <footer>Phase 0 · 基础工程验证<span>数据留在本机</span></footer>
    </main>
  )
}
