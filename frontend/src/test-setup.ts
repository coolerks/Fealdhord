import '@testing-library/jest-dom/vitest'
import { cleanup } from '@testing-library/react'
import { afterEach } from 'vitest'
afterEach(cleanup)

import { vi } from 'vitest'
Object.defineProperty(window, 'matchMedia', { writable: true, value: vi.fn((query: string) => ({ matches: false, media: query, onchange: null, addListener() {}, removeListener() {}, addEventListener() {}, removeEventListener() {}, dispatchEvent() { return true } })) })
if (!globalThis.ResizeObserver) vi.stubGlobal('ResizeObserver', class { observe() {} unobserve() {} disconnect() {} })
if (!CSS.escape) CSS.escape = (value: string) => value.replace(/[^a-zA-Z0-9_-]/g, char => `\\${char}`)
Object.defineProperty(HTMLElement.prototype, 'scrollTo', { configurable: true, value: function(this: HTMLElement, options: ScrollToOptions) { this.scrollTop = options?.top ?? 0; queueMicrotask(() => this.dispatchEvent(new Event('scroll'))) } })
Object.defineProperty(HTMLElement.prototype, 'scrollIntoView', { configurable: true, value() {} })
Object.defineProperty(HTMLElement.prototype, 'hasPointerCapture', { configurable: true, value() { return false } })
Object.defineProperty(HTMLElement.prototype, 'setPointerCapture', { configurable: true, value() {} })
Object.defineProperty(HTMLElement.prototype, 'releasePointerCapture', { configurable: true, value() {} })
