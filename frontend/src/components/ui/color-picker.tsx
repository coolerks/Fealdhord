import { useRef, useState } from 'react'
import { Popover } from 'radix-ui'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { cn } from '@/lib/utils'

export const hexColorPattern = /^#([a-f0-9]{6}|[a-f0-9]{8})$/i
const rgbaPattern = /^rgba?\(\s*(\d{1,3})\s*,\s*(\d{1,3})\s*,\s*(\d{1,3})\s*(?:,\s*([0-9]*\.?[0-9]+%?)\s*)?\)$/i

export type ColorPreset = { value: string; label: string }

export const defaultColorPresets: readonly ColorPreset[] = [
  { value: '#64748b', label: '石板灰' },
  { value: '#b77d4d', label: '暖琥珀' },
  { value: '#c2594b', label: '陶土红' },
  { value: '#4e8a63', label: '松针绿' },
  { value: '#3b8299', label: '湖水蓝' },
  { value: '#4c73b8', label: '钴蓝' },
  { value: '#7c5db5', label: '暮紫' },
  { value: '#b25b88', label: '梅红' },
] as const

const checkerboardBg =
  'repeating-conic-gradient(from 0deg, rgba(128,128,128,0.22) 0% 25%, transparent 0% 50%) 0 0 / 8px 8px'

function clamp(value: number, min: number, max: number) {
  return Math.min(max, Math.max(min, value))
}

function rgbToHsv(r: number, g: number, b: number): [number, number, number] {
  const rn = r / 255
  const gn = g / 255
  const bn = b / 255
  const max = Math.max(rn, gn, bn)
  const min = Math.min(rn, gn, bn)
  const delta = max - min
  let h = 0
  if (delta !== 0) {
    if (max === rn) h = 60 * (((gn - bn) / delta) % 6)
    else if (max === gn) h = 60 * ((bn - rn) / delta + 2)
    else h = 60 * ((rn - gn) / delta + 4)
  }
  if (h < 0) h += 360
  const s = max === 0 ? 0 : delta / max
  return [h, s, max]
}

function hsvToRgb(h: number, s: number, v: number): [number, number, number] {
  const c = v * s
  const x = c * (1 - Math.abs(((h / 60) % 2) - 1))
  const m = v - c
  let r = 0
  let g = 0
  let b = 0
  if (h < 60) [r, g, b] = [c, x, 0]
  else if (h < 120) [r, g, b] = [x, c, 0]
  else if (h < 180) [r, g, b] = [0, c, x]
  else if (h < 240) [r, g, b] = [0, x, c]
  else if (h < 300) [r, g, b] = [x, 0, c]
  else [r, g, b] = [c, 0, x]
  return [Math.round((r + m) * 255), Math.round((g + m) * 255), Math.round((b + m) * 255)]
}

function hexToHsva(hex: string): [number, number, number, number] {
  if (!hexColorPattern.test(hex)) return [...rgbToHsv(100, 116, 139), 1]
  const clean = hex.slice(1)
  const r = parseInt(clean.slice(0, 2), 16)
  const g = parseInt(clean.slice(2, 4), 16)
  const b = parseInt(clean.slice(4, 6), 16)
  const a = clean.length === 8 ? parseInt(clean.slice(6, 8), 16) / 255 : 1
  return [...rgbToHsv(r, g, b), a]
}

function hsvaToHex(h: number, s: number, v: number, a: number): string {
  const [r, g, b] = hsvToRgb(h, s, v)
  const toHex = (n: number) => clamp(Math.round(n), 0, 255).toString(16).padStart(2, '0')
  const alphaByte = clamp(Math.round(a * 255), 0, 255)
  return alphaByte >= 255
    ? `#${toHex(r)}${toHex(g)}${toHex(b)}`
    : `#${toHex(r)}${toHex(g)}${toHex(b)}${toHex(alphaByte)}`
}

function parseColorInput(raw: string): string | null {
  const trimmed = raw.trim()
  if (trimmed === '') return ''
  if (hexColorPattern.test(trimmed)) return trimmed.toLowerCase()
  const match = rgbaPattern.exec(trimmed)
  if (!match) return null
  const r = clamp(Number(match[1]), 0, 255)
  const g = clamp(Number(match[2]), 0, 255)
  const b = clamp(Number(match[3]), 0, 255)
  let a = 1
  if (match[4] !== undefined) {
    a = match[4].endsWith('%')
      ? clamp(parseFloat(match[4]) / 100, 0, 1)
      : clamp(parseFloat(match[4]), 0, 1)
  }
  const [h, s, v] = rgbToHsv(r, g, b)
  return hsvaToHex(h, s, v, a)
}

export function ColorPicker({
  id,
  value,
  onChange,
  presets = defaultColorPresets,
  disabled = false,
  label = '颜色',
}: {
  id?: string
  value: string
  onChange: (hex: string) => void
  presets?: readonly ColorPreset[]
  disabled?: boolean
  label?: string
}) {
  const [open, setOpen] = useState(false)
  const [hsva, setHsva] = useState<[number, number, number, number]>(() => hexToHsva(value))
  const [textInput, setTextInput] = useState(value)
  const svRef = useRef<HTMLDivElement>(null)
  const hueRef = useRef<HTMLDivElement>(null)
  const alphaRef = useRef<HTMLDivElement>(null)

  const validColor = hexColorPattern.test(value) ? value : ''
  const [h, s, v, a] = hsva
  const [r, g, b] = hsvToRgb(h, s, v)
  const opaqueRgb = `rgb(${r}, ${g}, ${b})`
  const alphaPercent = Math.round(a * 100)

  function syncFromValue(next: string) {
    setTextInput(next)
    if (hexColorPattern.test(next)) setHsva(hexToHsva(next))
  }

  function applyHsva(nextH: number, nextS: number, nextV: number, nextA: number) {
    const clampedH = clamp(nextH, 0, 359.99)
    const clampedS = clamp(nextS, 0, 1)
    const clampedV = clamp(nextV, 0, 1)
    const clampedA = clamp(nextA, 0, 1)
    setHsva([clampedH, clampedS, clampedV, clampedA])
    const hex = hsvaToHex(clampedH, clampedS, clampedV, clampedA)
    setTextInput(hex)
    onChange(hex)
  }

  function pickSV(clientX: number, clientY: number) {
    const rect = svRef.current?.getBoundingClientRect()
    if (!rect || rect.width === 0 || rect.height === 0) return
    const nextS = clamp((clientX - rect.left) / rect.width, 0, 1)
    const nextV = clamp(1 - (clientY - rect.top) / rect.height, 0, 1)
    applyHsva(h, nextS, nextV, a)
  }

  function pickHue(clientX: number) {
    const rect = hueRef.current?.getBoundingClientRect()
    if (!rect || rect.width === 0) return
    const nextH = clamp((clientX - rect.left) / rect.width, 0, 1) * 359.99
    applyHsva(nextH, s === 0 && !validColor ? 0.65 : s, v === 0 && !validColor ? 0.8 : v, a)
  }

  function pickAlpha(clientX: number) {
    const rect = alphaRef.current?.getBoundingClientRect()
    if (!rect || rect.width === 0) return
    const nextA = clamp((clientX - rect.left) / rect.width, 0, 1)
    applyHsva(h, s === 0 && !validColor ? 0.65 : s, v === 0 && !validColor ? 0.8 : v, nextA)
  }

  const matchedPreset = presets.find(p => p.value.toLowerCase() === validColor.toLowerCase())

  return (
    <div className="flex flex-wrap items-center gap-2">
      <Button
        type="button"
        size="xs"
        variant={!validColor ? 'secondary' : 'outline'}
        disabled={disabled}
        onClick={() => { setTextInput(''); onChange('') }}
      >
        默认
      </Button>
      {presets.map(preset => {
        const active = validColor.toLowerCase() === preset.value.toLowerCase()
        return (
          <button
            key={preset.value}
            type="button"
            disabled={disabled}
            aria-label={preset.label}
            aria-pressed={active}
            title={preset.label}
            onClick={() => { syncFromValue(preset.value); onChange(preset.value) }}
            className={cn(
              'size-6 rounded-full border border-border transition-transform focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none disabled:opacity-50',
              active && 'ring-2 ring-ring ring-offset-2 ring-offset-popover',
            )}
            style={{ backgroundColor: preset.value }}
          />
        )
      })}
      <Popover.Root open={open && !disabled} onOpenChange={next => {
        if (next) syncFromValue(value)
        setOpen(next)
      }}>
        <Popover.Trigger asChild>
          <Button
            id={id}
            type="button"
            size="xs"
            variant="outline"
            disabled={disabled}
            aria-label={`选择自定义${label}`}
            className="gap-1.5"
          >
            <span
              className="relative size-3.5 overflow-hidden rounded-full border border-border"
              style={{ background: validColor ? checkerboardBg : undefined }}
            >
              <span
                className="absolute inset-0"
                style={{
                  background: validColor || 'conic-gradient(from 0deg, #ef4444, #eab308, #22c55e, #3b82f6, #a855f7, #ef4444)',
                }}
              />
            </span>
            <span className="font-mono text-xs">{matchedPreset ? matchedPreset.label : (validColor || '自定义')}</span>
          </Button>
        </Popover.Trigger>
        <Popover.Portal>
          <Popover.Content
            align="start"
            sideOffset={6}
            className="z-50 w-60 space-y-3 rounded-xl border bg-popover p-3 text-popover-foreground shadow-md"
          >
            <div
              ref={svRef}
              role="slider"
              tabIndex={0}
              aria-label="饱和度与明度"
              aria-valuetext={validColor || '默认'}
              className="relative h-32 w-full cursor-crosshair overflow-hidden rounded-lg border border-border select-none touch-none"
              style={{ backgroundColor: `hsl(${h}, 100%, 50%)` }}
              onPointerDown={event => {
                event.currentTarget.setPointerCapture(event.pointerId)
                pickSV(event.clientX, event.clientY)
              }}
              onPointerMove={event => {
                if (event.buttons !== 1) return
                pickSV(event.clientX, event.clientY)
              }}
              onKeyDown={event => {
                const step = event.shiftKey ? 0.1 : 0.03
                if (event.key === 'ArrowRight') { event.preventDefault(); applyHsva(h, s + step, v, a) }
                else if (event.key === 'ArrowLeft') { event.preventDefault(); applyHsva(h, s - step, v, a) }
                else if (event.key === 'ArrowUp') { event.preventDefault(); applyHsva(h, s, v + step, a) }
                else if (event.key === 'ArrowDown') { event.preventDefault(); applyHsva(h, s, v - step, a) }
              }}
            >
              <div className="pointer-events-none absolute inset-0 bg-gradient-to-r from-white to-transparent" />
              <div className="pointer-events-none absolute inset-0 bg-gradient-to-t from-black to-transparent" />
              <span
                className="pointer-events-none absolute size-3.5 -translate-x-1/2 -translate-y-1/2 rounded-full border-2 border-white shadow-sm ring-1 ring-black/40"
                style={{ left: `${s * 100}%`, top: `${(1 - v) * 100}%`, backgroundColor: opaqueRgb }}
              />
            </div>

            <div
              ref={hueRef}
              role="slider"
              tabIndex={0}
              aria-label="色相"
              aria-valuemin={0}
              aria-valuemax={360}
              aria-valuenow={Math.round(h)}
              className="relative h-3 w-full cursor-pointer rounded-full border border-border select-none touch-none"
              style={{ background: 'linear-gradient(to right, #ff0000 0%, #ffff00 17%, #00ff00 33%, #00ffff 50%, #0000ff 67%, #ff00ff 83%, #ff0000 100%)' }}
              onPointerDown={event => {
                event.currentTarget.setPointerCapture(event.pointerId)
                pickHue(event.clientX)
              }}
              onPointerMove={event => {
                if (event.buttons !== 1) return
                pickHue(event.clientX)
              }}
              onKeyDown={event => {
                const step = event.shiftKey ? 15 : 5
                if (event.key === 'ArrowRight' || event.key === 'ArrowUp') { event.preventDefault(); applyHsva((h + step) % 360, s, v, a) }
                else if (event.key === 'ArrowLeft' || event.key === 'ArrowDown') { event.preventDefault(); applyHsva((h - step + 360) % 360, s, v, a) }
              }}
            >
              <span
                className="pointer-events-none absolute top-1/2 size-3.5 -translate-x-1/2 -translate-y-1/2 rounded-full border-2 border-white shadow-sm ring-1 ring-black/40"
                style={{ left: `${(h / 360) * 100}%`, backgroundColor: `hsl(${h}, 100%, 50%)` }}
              />
            </div>

            <div className="space-y-1">
              <div className="flex items-center justify-between text-[11px] text-muted-foreground">
                <span>不透明度</span>
                <span className="tabular-nums">{alphaPercent}%</span>
              </div>
              <div
                ref={alphaRef}
                role="slider"
                tabIndex={0}
                aria-label="不透明度"
                aria-valuemin={0}
                aria-valuemax={100}
                aria-valuenow={alphaPercent}
                className="relative h-3 w-full cursor-pointer rounded-full border border-border select-none touch-none"
                style={{
                  background: `linear-gradient(to right, rgba(${r}, ${g}, ${b}, 0) 0%, ${opaqueRgb} 100%), ${checkerboardBg}`,
                }}
                onPointerDown={event => {
                  event.currentTarget.setPointerCapture(event.pointerId)
                  pickAlpha(event.clientX)
                }}
                onPointerMove={event => {
                  if (event.buttons !== 1) return
                  pickAlpha(event.clientX)
                }}
                onKeyDown={event => {
                  const step = event.shiftKey ? 0.1 : 0.02
                  if (event.key === 'ArrowRight' || event.key === 'ArrowUp') { event.preventDefault(); applyHsva(h, s, v, a + step) }
                  else if (event.key === 'ArrowLeft' || event.key === 'ArrowDown') { event.preventDefault(); applyHsva(h, s, v, a - step) }
                }}
              >
                <span
                  className="pointer-events-none absolute top-1/2 size-3.5 -translate-x-1/2 -translate-y-1/2 rounded-full border-2 border-white shadow-sm ring-1 ring-black/40"
                  style={{ left: `${a * 100}%`, backgroundColor: `rgba(${r}, ${g}, ${b}, ${a})` }}
                />
              </div>
            </div>

            <div className="flex items-center gap-2">
              <span
                className="relative size-7 shrink-0 overflow-hidden rounded-md border border-border"
                style={{ background: checkerboardBg }}
              >
                <span className="absolute inset-0" style={{ backgroundColor: validColor || 'transparent' }} />
              </span>
              <Input
                aria-label="颜色色值"
                placeholder="#64748b 或 rgba(...)"
                maxLength={32}
                className="h-7 flex-1 font-mono text-xs"
                value={textInput}
                onChange={event => {
                  const next = event.target.value
                  setTextInput(next)
                  const parsed = parseColorInput(next)
                  if (parsed === '') onChange('')
                  else if (parsed) {
                    setHsva(hexToHsva(parsed))
                    onChange(parsed)
                  }
                }}
              />
            </div>
          </Popover.Content>
        </Popover.Portal>
      </Popover.Root>
    </div>
  )
}
