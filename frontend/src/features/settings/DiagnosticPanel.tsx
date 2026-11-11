import { Button } from '@/components/ui/button'
import { useEffect, useRef, useState } from 'react'
import { desktopAPI } from '../../lib/api'
import type { Info, OperationSnapshot } from '../../lib/api'
import { mergeSnapshot } from '../../lib/operation-state'

const statusLabels: Record<string, string> = { running: '正在检查', cancelled: '已取消', succeeded: '检查完成' }

export default function DiagnosticPanel() {
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
      <section className="diagnostics">
        <p className="connection">{info ? '桌面服务已连接' : '连接中'}</p>
        {info && <dl className="system-info"><div><dt>版本</dt><dd>{info.version}</dd></div><div><dt>运行平台</dt><dd>{info.platform} / {info.architecture}</dd></div><div><dt>桌面框架</dt><dd>Wails {info.wails_version}</dd></div></dl>}
        <div className="probe"><div className="progress-label"><span role="status">{operation ? statusLabels[operation.status] ?? operation.status : '尚未开始检查'}</span><span>{operation ? `${operation.completed} / ${operation.total}` : '约 5 秒'}</span></div><progress aria-label="连接检查进度" max={operation?.total ?? 20} value={operation?.completed ?? 0}/><div className="actions"><Button onClick={() => { void start() }} disabled={!info || busy}>开始检查</Button><Button variant="outline" onClick={() => { void cancel() }} disabled={operation?.status !== 'running' || pending}>取消检查</Button></div></div>
        {error && <p role="alert" className="error">{error}</p>}
      </section>

  )
}
