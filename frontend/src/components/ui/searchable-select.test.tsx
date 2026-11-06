import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { expect, it, vi } from 'vitest'
import { Dialog, DialogContent, DialogTitle } from '@/components/ui/dialog'
import { SearchableSelect } from './searchable-select'

it('filters labels and aliases and selects with the keyboard inside a dialog', async () => {
  const choose = vi.fn()
  const user = userEvent.setup()
  render(<Dialog defaultOpen>
    <DialogContent aria-describedby={undefined}>
      <DialogTitle>设置</DialogTitle>
      <SearchableSelect label="模型" value="" onValueChange={choose} options={[
        { value: 'first', label: 'GPT Mini', keywords: ['gpt-4.1-mini'] },
        { value: 'second', label: 'Claude Sonnet' },
      ]} />
    </DialogContent>
  </Dialog>)
  const trigger = screen.getByRole('button', { name: '模型' })
  await user.click(trigger)
  const input = screen.getByRole('combobox', { name: '搜索模型' })
  expect(input).toHaveFocus()
  await user.type(input, '41mini')
  expect(screen.getByRole('option', { name: 'GPT Mini' })).toBeVisible()
  expect(screen.queryByRole('option', { name: 'Claude Sonnet' })).toBeNull()
  await user.keyboard('{ArrowDown}{Enter}')
  expect(choose).toHaveBeenCalledExactlyOnceWith('first')
  await waitFor(() => expect(trigger).toHaveFocus())
  await user.click(trigger)
  await user.keyboard('{Escape}')
  expect(screen.queryByRole('combobox')).toBeNull()
  expect(screen.getByRole('dialog', { name: '设置' })).toBeVisible()
})
