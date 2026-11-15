import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter, Route, Routes } from 'react-router'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { ApiError } from '../../api/client'
import * as api from '../../api/sales'
import { es } from '../../i18n/es'
import { sale } from '../../test/fixtures'
import { SaleDetailPage } from './SaleDetailPage'

vi.mock('../../api/sales')

const getSale = vi.mocked(api.getSale)
const t = es.sales.detail

function renderPage() {
  return render(
    <MemoryRouter initialEntries={['/ventas/sale-1']}>
      <Routes>
        <Route path="/ventas/:id" element={<SaleDetailPage />} />
      </Routes>
    </MemoryRouter>,
  )
}

beforeEach(() => {
  vi.resetAllMocks()
})

describe('SaleDetailPage', () => {
  it('shows the totals, cost and gross profit of the sale', async () => {
    getSale.mockResolvedValue(sale())

    renderPage()

    const header = await screen.findByRole('region', { name: t.title })
    expect(header).toHaveTextContent('10/11/2026')
    expect(header).toHaveTextContent('Bs 305,00')
    expect(header).toHaveTextContent('Bs 161,65')
    expect(header).toHaveTextContent('Bs 143,35')
    expect(header).toHaveTextContent(t.status.confirmed)
    expect(header).toHaveTextContent('Cliente de paso')
  })

  it('shows a roll line with its roll code, meters and the price charged against the reference', async () => {
    getSale.mockResolvedValue(sale())

    renderPage()

    const line = await screen.findByRole('region', { name: t.lineHeading(1, 'PVC floral 20 m') })
    expect(line).toHaveTextContent(es.sales.kinds.roll_meters)
    expect(line).toHaveTextContent('ROL-000001')
    expect(within(line).getByText('2,50 m')).toBeInTheDocument()
    expect(within(line).getByText('Bs 18,00')).toBeInTheDocument() // charged
    expect(within(line).getByText('Bs 15,50')).toBeInTheDocument() // reference
    expect(within(line).getByText('Bs 8,44')).toBeInTheDocument() // profit
  })

  it('shows tablecloth units without a roll and a whole roll as one roll', async () => {
    const whole = sale().lines[0]!
    getSale.mockResolvedValue(
      sale({ lines: [{ ...sale().lines[1]!, number: 1 }, { ...whole, id: 'line-3', number: 2, kind: 'full_roll', quantity: '1.00', unit: 'roll' }] }),
    )

    renderPage()

    const cloth = await screen.findByRole('region', { name: t.lineHeading(1, 'Mantel rojo') })
    expect(within(cloth).getByText('4 u.')).toBeInTheDocument()
    expect(cloth).not.toHaveTextContent(t.roll)
    const roll = screen.getByRole('region', { name: t.lineHeading(2, 'PVC floral 20 m') })
    expect(within(roll).getByText('1 rollo')).toBeInTheDocument()
  })

  it('has no edit or delete actions: a registered sale is final', async () => {
    getSale.mockResolvedValue(sale())

    renderPage()

    await screen.findByRole('region', { name: t.title })
    expect(screen.queryByRole('button', { name: /editar|eliminar|anular/i })).not.toBeInTheDocument()
    expect(screen.getByText(t.immutableNote)).toBeInTheDocument()
  })

  it('shows an error and retries', async () => {
    getSale.mockRejectedValueOnce(new ApiError(404, 'sale_not_found')).mockResolvedValue(sale())
    const user = userEvent.setup()
    renderPage()

    expect(await screen.findByRole('alert')).toHaveTextContent(es.apiErrors.sale_not_found)
    await user.click(screen.getByRole('button', { name: es.common.retry }))

    expect(await screen.findByRole('region', { name: t.title })).toBeInTheDocument()
  })
})
