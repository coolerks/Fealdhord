import { useRef, useState } from 'react'
import { Popover } from 'radix-ui'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { cn } from '@/lib/utils'

const hexPattern = /^#[a-f0-9]{6}$/i

export type ColorPreset = { value: string; label: string }

function clamp(value: number, min: number, max: number) {
  return Math.min(max, Math.max(min, value))
}

function hexToHsv(hex: string): [number, number, number] {
  const clean = hexPattern.test(hex) ? hex.slice(1) : '64748b'
  const r = parseInt(clean.slice(0, 2), 16) / 255
  const g = parseInt(clean.slice(2, 4), 16) / 255
  const b = parseInt(clean.slice(4, 6), 16) / 255
  const max = Math.max(r, g, b)
  const min = Math.min(r, g, b)
  const delta = max - min
  let h = 0
  if (delta !== 0) {
    if (max === r) h = 60 * (((g - b) / delta) % 6)
    else if (max === g) h = 60 * ((b - r) / delta + 2)
    else h = 60 * ((r - g) / delta + 4)
  }
  if (h < 0) h += 360
  const s = max === 0 ? 0 : delta / max
  return [h, s, max]
}

function hsvToHex(h: number, s: number, v: number): string {
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
  const toHex = (n: number) => Math.round((n + m) * 255).toString(16).padStart(2, '0')
  return `#${toHex(r)}${toHex(g)}${toHex(b)}`
}

export function ColorPicker({
  id,
  value,
  onChange,
  presets = [],
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
  const [hsv, setHsv] = useState<[number, number, number]>(() => hexToHsv(value))
  const [hexInput, setHexInput] = useState(value)
  const svRef = useRef<HTMLDivElement>(null)
  const hueRef = useRef<HTMLDivElement>(null)

  const validColor = hexPattern.test(value) ? value : ''
  const [h, s, v] = hsv

  function syncFromValue(next: string) {
    setHexInput(next)
    if (hexPattern.test(next)) setHsv(hexToHsv(next))
  }

  function applyHsv(nextH: number, nextS: number, nextV: number) {
    const clampedH = clamp(nextH, 0, 359.99)
    const clampedS = clamp(nextS, 0, 1)
    const clampedV = clamp(nextV, 0, 1)
    setHsv([clampedH, clampedS, clampedV])
    const hex = hsvToHex(clampedH, clampedS, clampedV)
    setHexInput(hex)
    onChange(hex)
  }

  function pickSV(clientX: number, clientY: number) {
    const rect = svRef.current?.getBoundingClientRect()
    if (!rect || rect.width === 0 || rect.height === 0) return
    const nextS = clamp((clientX - rect.left) / rect.width, 0, 1)
    const nextV = clamp(1 - (clientY - rect.top) / rect.height, 0, 1)
    applyHsv(h, nextS, nextV)
  }

  function pickHue(clientX: number) {
    const rect = hueRef.current?.getBoundingClientRect()
    if (!rect || rect.width === 0) return
    const nextH = clamp((clientX - rect.left) / rect.width, 0, 1) * 359.99
    applyHsv(nextH, s === 0 && !validColor ? 0.65 : s, v === 0 && !validColor ? 0.8 : v)
  }

  const matchedPreset = presets.find(p => p.value.toLowerCase() === validColor.toLowerCase())

  return (
    <div className="flex flex-wrap items-center gap-2">
      <Button
        type="button"
        size="xs"
        variant={!validColor ? 'secondary' : 'outline'}
        disabled={disabled}
        onClick={() => { setHexInput(''); onChange('') }}
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
              className="size-3.5 rounded-full border border-border"
              style={{
                background: validColor || 'conic-gradient(from 0deg, #ef4444, #eab308, #22c55e, #3b82f6, #a855f7, #ef4444)',
              }}
            />
            <span className="font-mono text-xs">{matchedPreset ? matchedPreset.label : (validColor || '自定义')}</span>
          </Button>
        </Popover.Trigger>
        <Popover.Portal>
          <Popover.Content
            align="start"
            sideOffset={6}
            className="z-50 w-56 space-y-3 rounded-xl border bg-popover p-3 text-popover-foreground shadow-md"
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
                if (event.key === 'ArrowRight') { event.preventDefault(); applyHsv(h, s + step, v) }
                else if (event.key === 'ArrowLeft') { event.preventDefault(); applyHsv(h, s - step, v) }
                else if (event.key === 'ArrowUp') { event.preventDefault(); applyHsv(h, s, v + step) }
                else if (event.key === 'ArrowDown') { event.preventDefault(); applyHsv(h, s, v - step) }
              }}
            >
              <div className="pointer-events-none absolute inset-0 bg-gradient-to-r from-white to-transparent" />
              <div className="pointer-events-none absolute inset-0 bg-gradient-to-t from-black to-transparent" />
              <span
                className="pointer-events-none absolute size-3.5 -translate-x-1/2 -translate-y-1/2 rounded-full border-2 border-white shadow-sm ring-1 ring-black/40"
                style={{ left: `${s * 100}%`, top: `${(1 - v) * 100}%`, backgroundColor: hsvToHex(h, s, v) }}
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
                if (event.key === 'ArrowRight' || event.key === 'ArrowUp') { event.preventDefault(); applyHsv((h + step) % 360, s, v) }
                else if (event.key === 'ArrowLeft' || event.key === 'ArrowDown') { event.preventDefault(); applyHsv((h - step + 360) % 360, s, v) }
              }}
            >
              <span
                className="pointer-events-none absolute top-1/2 size-3.5 -translate-x-1/2 -translate-y-1/2 rounded-full border-2 border-white shadow-sm ring-1 ring-black/40"
                style={{ left: `${(h / 360) * 100}%`, backgroundColor: `hsl(${h}, 100%, 50%)` }}
              />
            </div>

            <div className="flex items-center gap-2">
              <span
                className="size-7 shrink-0 rounded-md border border-border"
                style={{ backgroundColor: validColor || 'transparent' }}
              />
              <Input
                aria-label="颜色色值"
                placeholder="#64748b"
                maxLength={7}
                className="h-7 flex-1 font-mono text-xs"
                value={hexInput}
                onChange={event => {
                  const next = event.target.value
                  setHexInput(next)
                  if (next === '') onChange('')
                  else if (hexPattern.test(next)) {
                    setHsv(hexToHsv(next))
                    onChange(next)
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
