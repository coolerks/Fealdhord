// JSDOM 不执行布局；仅在虚拟化组件测试中提供可控的几何测量。
import { vi } from 'vitest'

export function installGeometry() {
  vi.spyOn(HTMLElement.prototype, 'clientWidth', 'get').mockImplementation(function(this: HTMLElement) {
    return this.classList.contains('masonry-scroll') ? 760 : 360
  })
  vi.spyOn(HTMLElement.prototype, 'offsetWidth', 'get').mockImplementation(function(this: HTMLElement) {
    return Number.parseFloat(this.style.width) || 360
  })
  vi.spyOn(HTMLElement.prototype, 'offsetHeight', 'get').mockImplementation(function(this: HTMLElement) {
    if (this.classList.contains('article-scroll')) return 600
    if (this.tagName === 'LI') {
      const cover = this.querySelector<HTMLElement>('.article-cover')
      return this.querySelector('.article-card') ? 150 + (parseFloat(cover?.style.height ?? '') || 0) : Number.parseFloat(this.style.height) || 96
    }
    return 32
  })
  vi.spyOn(HTMLElement.prototype, 'clientHeight', 'get').mockReturnValue(600)
  vi.spyOn(HTMLElement.prototype, 'scrollHeight', 'get').mockImplementation(function(this: HTMLElement) {
    return Number.parseFloat(this.querySelector<HTMLElement>('.virtual-articles')?.style.height ?? '') || 600
  })
  class ControlledResizeObserver implements ResizeObserver {
    targets = new Set<Element>()
    constructor(public callback: ResizeObserverCallback) { observers.add(this) }
    observe(target: Element) {
      this.targets.add(target)
      queueMicrotask(() => { if (this.targets.has(target)) this.notify() })
    }
    unobserve(target: Element) { this.targets.delete(target) }
    disconnect() { this.targets.clear(); observers.delete(this) }
    notify() {
      const entries = Array.from(this.targets).map(target => {
        const element = target as HTMLElement
        const width = element.classList.contains('article-scroll') ? element.clientWidth : element.offsetWidth
        const height = element.offsetHeight
        return { target, contentRect: new DOMRectReadOnly(0, 0, width, height), borderBoxSize: [{ inlineSize: width, blockSize: height }], contentBoxSize: [{ inlineSize: width, blockSize: height }], devicePixelContentBoxSize: [{ inlineSize: width, blockSize: height }] }
      })
      this.callback(entries, this)
    }
  }
  const observers = new Set<ControlledResizeObserver>()
  vi.stubGlobal('ResizeObserver', ControlledResizeObserver)
  return {
    cleanup() { for (const observer of observers) observer.targets.clear(); observers.clear() },
    resize() { for (const observer of observers) observer.notify() },
  }
}
