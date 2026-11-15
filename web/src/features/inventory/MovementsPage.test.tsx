import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import * as api from '../../api/inventory'
import { es } from '../../i18n/es'
import { movement, pageOf } from '../../test/fixtures'
import { MovementsPage } from './MovementsPage'

vi.mock('../../api/inventory')

const listMovements = vi.mocked(api.listMovements)
const m = es.inventory.movements

function renderPage(path = '/inventario/movimientos') {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <MovementsPage />
    </MemoryRouter>,
  )
}

beforeEach(() => {
  vi.resetAllMocks()
})

describe('MovementsPage', () => {
  it('shows a roll entry in meters and a tablecloth entry in units, both with their sign', async () => {
    listMovements.mockResolvedValue(
      pageOf([
        movement(),
        movement({
          id: 'mov-2',
          roll: null,
          product: { id: 'cloth-1', sku: 'MAN-ROJO-01', name: 'Mantel rojo' },
          quantity_delta: '10.00',
          unit: 'unit',
        }),
      ]),
    )

    renderPage()

    const rollRow = (await screen.findByText('ROL-000001')).closest('tr')!
    expect(within(rollRow).getByText('+19,80 m')).toBeInTheDocument()
    expect(within(rollRow).getByText(m.types.purchase_in!)).toBeInTheDocument()

    const clothRow = screen.getByText('MAN-ROJO-01 — Mantel rojo').closest('tr')!
    expect(within(clothRow).getByText('+10 u.')).toBeInTheDocument()
    expect(within(clothRow).getByText(es.common.noValue, { selector: 'td' })).toBeInTheDocument()
  })

  it('shows an outflow with a minus sign', async () => {
    listMovements.mockResolvedValue(pageOf([movement({ quantity_delta: '-5.50' })]))

    renderPage()

    expect(await screen.findByText('−5,50 m')).toBeInTheDocument()
  })

  it('links a purchase movement to its purchase and names unknown types neutrally', async () => {
    listMovements.mockResolvedValue(
      pageOf([movement(), movement({ id: 'mov-3', movement_type: 'future_type' as never, reference_type: 'adjustment' })]),
    )

    renderPage()

    expect(await screen.findByRole('link', { name: m.referencePurchase })).toHaveAttribute('href', '/compras/pur-1')
    expect(screen.getByText(m.unknownType)).toBeInTheDocument()
  })

  it('filters by product and roll from the address, and removes both filters together', async () => {
    listMovements.mockResolvedValue(pageOf([movement()]))
    const user = userEvent.setup()
    renderPage('/inventario/movimientos?producto=pvc-1&rollo=roll-1')

    expect(await screen.findByText(new RegExp(m.filteredByRoll))).toBeInTheDocument()
    expect(listMovements).toHaveBeenLastCalledWith(
      { productId: 'pvc-1', rollId: 'roll-1', limit: 25, offset: 0 },
      expect.any(AbortSignal),
    )

    await user.click(screen.getByRole('button', { name: m.clearFilters }))

    await vi.waitFor(() =>
      expect(listMovements).toHaveBeenLastCalledWith({ productId: undefined, rollId: undefined, limit: 25, offset: 0 }, expect.any(AbortSignal)),
    )
  })

  it('explains an empty ledger', async () => {
    listMovements.mockResolvedValue(pageOf([]))

    renderPage()

    expect(await screen.findByText(m.emptyTitle)).toBeInTheDocument()
  })
})
