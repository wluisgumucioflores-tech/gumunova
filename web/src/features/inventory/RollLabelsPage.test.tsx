import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter, Route, Routes } from 'react-router'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import * as fetchAll from '../../api/fetchAll'
import { es } from '../../i18n/es'
import { roll } from '../../test/fixtures'
import { RollLabelsPage } from './RollLabelsPage'

vi.mock('../../api/fetchAll')

const t = es.inventory.rolls.labels

function renderAt(search: string) {
  return render(
    <MemoryRouter initialEntries={[`/inventario/rollos/etiquetas${search}`]}>
      <Routes>
        <Route path="/inventario/rollos/etiquetas" element={<RollLabelsPage />} />
      </Routes>
    </MemoryRouter>,
  )
}

const first = roll()
const second = roll({ id: 'roll-2', code: 'ROL-000002', label_code: 'R-222222222222' })

beforeEach(() => {
  vi.resetAllMocks()
  vi.stubGlobal('print', vi.fn())
  vi.mocked(fetchAll.listAllRolls).mockResolvedValue([first, second])
})

describe('RollLabelsPage', () => {
  it('prints a label for each chosen roll with its own code, even a depleted one', async () => {
    renderAt('?ids=roll-2')

    const labels = within(await screen.findByRole('list', { name: t.title })).getAllByRole('listitem')
    expect(labels).toHaveLength(1)
    expect(labels[0]).toHaveTextContent('ROL-000002')
    expect(labels[0]).toHaveTextContent('R-222222222222')
    expect(labels[0]).toHaveTextContent('PVC-FLORAL-20 — PVC floral 20 m')
    expect(fetchAll.listAllRolls).toHaveBeenCalledWith('all', expect.anything())
  })

  it('"todos" prints only the rolls that still have meters', async () => {
    const user = userEvent.setup()
    renderAt('?todos=1')

    await user.click(await screen.findByRole('button', { name: es.labels.print }))

    expect(fetchAll.listAllRolls).toHaveBeenCalledWith('available', expect.anything())
    expect(within(screen.getByRole('list', { name: t.title })).getAllByRole('listitem')).toHaveLength(2)
  })

  it('asks to choose rolls first', async () => {
    renderAt('')

    expect(await screen.findByText(t.nothingSelected)).toBeInTheDocument()
    expect(fetchAll.listAllRolls).toHaveBeenCalledTimes(0)
  })
})
