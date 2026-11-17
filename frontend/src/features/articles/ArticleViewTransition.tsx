import { Component, createRef } from 'react'
import type { ReactNode, RefObject } from 'react'

type Props = {
  view: string
  motion: 'full' | 'subtle' | 'off'
  host: RefObject<HTMLDivElement | null>
  children: ReactNode
}

type Snapshot = {
  content: HTMLDivElement
  scrollTop: number
}

export class ArticleViewTransition extends Component<Props> {
  private live = createRef<HTMLDivElement>()
  private outgoing: HTMLDivElement | null = null

  getSnapshotBeforeUpdate(previous: Props): Snapshot | null {
    const live = this.live.current
    const host = this.props.host.current
    if (previous.view === this.props.view || this.props.motion === 'off' || !live || !host || live.closest('[inert]')) return null

    const rect = live.getBoundingClientRect()
    const bounds = host.getBoundingClientRect()
    if (!rect.width || !rect.height) return null
    const content = live.cloneNode(true) as HTMLDivElement
    delete content.dataset.viewEntering
    content.inert = true
    content.classList.add('article-view-outgoing')
    Object.assign(content.style, {
      left: `${rect.left - bounds.left}px`,
      top: `${rect.top - bounds.top}px`,
      width: `${rect.width}px`,
      height: `${rect.height}px`,
    })
    // 旧画面只用来绘制，不参与焦点、文章命令或快捷键匹配。
    for (const element of content.querySelectorAll('[id], [data-article-id], [data-article-trigger]')) {
      element.removeAttribute('id')
      element.removeAttribute('data-article-id')
      element.removeAttribute('data-article-trigger')
    }
    return { content, scrollTop: live.querySelector('.article-scroll')?.scrollTop ?? 0 }
  }

  componentDidUpdate(previous: Props, _state: unknown, snapshot: Snapshot | null) {
    if (previous.view === this.props.view && previous.motion === this.props.motion) return
    this.finish()
    if (previous.view === this.props.view || this.props.motion === 'off') return
    const live = this.live.current
    const host = this.props.host.current
    if (!live || !host) return
    if (!live.closest('[inert]')) {
      live.dataset.viewEntering = this.props.motion
      live.addEventListener('animationend', this.onEnter)
      live.addEventListener('animationcancel', this.onEnter)
    }
    if (!snapshot) return
    const { content } = snapshot
    content.dataset.viewLeaving = this.props.motion
    host.append(content)
    const viewport = content.querySelector('.article-scroll')
    if (viewport) viewport.scrollTop = snapshot.scrollTop
    this.outgoing = content
    content.addEventListener('animationend', this.onExit)
    content.addEventListener('animationcancel', this.onExit)
  }

  componentWillUnmount() {
    this.finish()
  }

  private onExit = (event: AnimationEvent) => {
    if (event.target === this.outgoing) this.finish()
  }

  private onEnter = (event: AnimationEvent) => {
    if (event.target === this.live.current) this.finishEnter()
  }

  private finishEnter() {
    const live = this.live.current
    if (!live) return
    live.removeEventListener('animationend', this.onEnter)
    live.removeEventListener('animationcancel', this.onEnter)
    delete live.dataset.viewEntering
  }

  private finish() {
    const outgoing = this.outgoing
    if (outgoing) {
      outgoing.removeEventListener('animationend', this.onExit)
      outgoing.removeEventListener('animationcancel', this.onExit)
      outgoing.remove()
      this.outgoing = null
    }
    this.finishEnter()
  }

  render() {
    return <div className="article-view-live" ref={this.live}>{this.props.children}</div>
  }
}
