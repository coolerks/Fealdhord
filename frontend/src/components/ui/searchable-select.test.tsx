import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { expect, it, vi } from 'vitest'
import { Dialog, DialogContent, DialogTitle } from '@/components/ui/dialog'
import { TooltipProvider } from '@/components/ui/tooltip'
import { SearchableSelect } from './searchable-select'

const hideSearch = '[&_[data-slot=command-input-wrapper]]:sr-only'

it('reveals search when typing in a short list and filters aliases inside a dialog', async () => {
  const choose = vi.fn()
  const user = userEvent.setup()
  render(<TooltipProvider><Dialog defaultOpen>
    <DialogContent aria-describedby={undefined}>
      <DialogTitle>设置</DialogTitle>
      <SearchableSelect label="模型" value="" onValueChange={choose} options={[
        { value: 'first', label: 'GPT Mini', keywords: ['gpt-4.1-mini'] },
        { value: 'second', label: 'Claude Sonnet' },
      ]} />
    </DialogContent>
  </Dialog></TooltipProvider>)
  const trigger = screen.getByRole('button', { name: '模型' })
  await user.click(trigger)
  const input = screen.getByRole('combobox', { name: '搜索模型' })
  expect(input).toHaveFocus()
  expect(input.closest('[data-slot=command]')).toHaveClass(hideSearch)
  await user.keyboard('41mini')
  expect(input.closest('[data-slot=command]')).not.toHaveClass(hideSearch)
  expect(screen.getByRole('option', { name: 'GPT Mini' })).toBeVisible()
  expect(screen.queryByRole('option', { name: 'Claude Sonnet' })).toBeNull()
  await user.keyboard('{ArrowDown}{Enter}')
  expect(choose).toHaveBeenCalledExactlyOnceWith('first')
  await waitFor(() => expect(trigger).toHaveFocus())
  await user.click(trigger)
  expect(screen.getByRole('combobox').closest('[data-slot=command]')).toHaveClass(hideSearch)
  await user.keyboard('{Escape}')
  expect(screen.queryByRole('combobox')).toBeNull()
  expect(screen.getByRole('dialog', { name: '设置' })).toBeVisible()
})

it.each([9, 10])('shows search only at the threshold and keeps keyboard selection with %i options', async count => {
  const choose = vi.fn()
  const user = userEvent.setup()
  const options = Array.from({ length: count }, (_, i) => ({ label: `选项 ${i + 1}`, value: String(i + 1) }))
  render(<TooltipProvider><SearchableSelect label="分类" value="" options={options} onValueChange={choose} /></TooltipProvider>)
  const trigger = screen.getByRole('button', { name: '分类' })
  await user.tab()
  await user.keyboard('{Enter}')
  const input = screen.getByRole('combobox', { name: '搜索分类' })
  expect(input).toHaveFocus()
  expect(input.closest('[data-slot=command]')?.classList.contains(hideSearch)).toBe(count < 10)
  await user.keyboard('{ArrowDown}{Enter}')
  expect(choose).toHaveBeenCalledExactlyOnceWith('2')
  await waitFor(() => expect(trigger).toHaveFocus())
})

it('shows the full label on hover only when the option text overflows', async () => {
  const user = userEvent.setup()
  const fullLabel = 'DeepSeek V4.1 Flash · commandcode-plan/deepseek-v4.1-flash'
  const { baseElement } = render(<TooltipProvider>
    <SearchableSelect label="模型" value="" options={[{ value: 'short', label: '短名称' }, { value: 'long', label: fullLabel }]}
      onValueChange={() => {}} />
  </TooltipProvider>)
  await user.click(screen.getByRole('button', { name: '模型' }))
  const short = screen.getByText('短名称')
  const long = screen.getByText(fullLabel)
  Object.defineProperties(short, { clientWidth: { value: 120 }, scrollWidth: { value: 120 } })
  Object.defineProperties(long, { clientWidth: { value: 120 }, scrollWidth: { value: 360 } })
  const tooltip = () => baseElement.querySelector('[data-slot=tooltip-content]')
  await user.hover(screen.getByRole('option', { name: '短名称' }))
  expect(tooltip()).toBeNull()
  const longOption = screen.getByRole('option', { name: fullLabel })
  await user.hover(longOption)
  await waitFor(() => expect(tooltip()).toHaveTextContent(fullLabel))
  await user.hover(screen.getByRole('option', { name: '短名称' }))
  await waitFor(() => expect(tooltip()).toBeNull())
})
