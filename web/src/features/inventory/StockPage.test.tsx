import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { ApiError } from '../../api/client'
import * as api from '../../api/inventory'
import { es } from '../../i18n/es'
import { clothStockItem, pageOf, stockItem } from '../../test/fixtures'
import { StockPage } from './StockPage'

vi.mock('../../api/inventory')

const listStock = vi.mocked(api.listStock)
const s = es.inventory.stock

function renderPage() {
  return render(
    <MemoryRouter>
      <StockPage />
    </MemoryRouter>,
  )
}

beforeEach(() => {
  vi.resetAllMocks()
})

describe('StockPage', () => {
  it('shows PVC in meters and tablecloths in units, with Spanish numbers', async () => {
    listStock.mockResolvedValue(pageOf([stockItem(), clothStockItem()]))

    renderPage()

    const pvcRow = (await screen.findByText('PVC-FLORAL-20')).closest('tr')!
    expect(within(pvcRow).getByText('39,80 m')).toBeInTheDocument()
    expect(within(pvcRow).getByText('50,00 m')).toBeInTheDocument()
    expect(within(pvcRow).getByText('Bs 599,77')).toBeInTheDocument()

    const clothRow = screen.getByText('MAN-ROJO-01').closest('tr')!
    expect(within(clothRow).getByText('1.234 u.')).toBeInTheDocument()
    expect(within(clothRow).getByText('Bs 31,0230')).toBeInTheDocument()
    expect(screen.getByText(s.resultsSummary(2, 2))).toBeInTheDocument()
  })

  it('labels every cell with its column so the table can become cards on a phone', async () => {
    listStock.mockResolvedValue(pageOf([stockItem()]))

    renderPage()

    const row = (await screen.findByText('PVC-FLORAL-20')).closest('tr')!
    const headers = screen.getAllByRole('columnheader').map((header) => header.textContent)
    const labels = Array.from(row.querySelectorAll('td')).map((cell) => cell.getAttribute('data-label'))
    expect(labels).toEqual(headers)
    expect(screen.getByRole('table')).toHaveClass('table-cards')
  })

  it('flags low stock, shows healthy stock and says when there is no alert configured', async () => {
    listStock.mockResolvedValue(
      pageOf([stockItem(), stockItem({ product_id: 'p2', sku: 'PVC-OK', is_low: false, quantity: '90.00' }), clothStockItem()]),
    )

    renderPage()

    const lowRow = (await screen.findByText('PVC-FLORAL-20')).closest('tr')!
    expect(within(lowRow).getByText(s.lowBadge)).toBeInTheDocument()
    expect(within(screen.getByText('PVC-OK').closest('tr')!).getByText(s.okBadge)).toBeInTheDocument()
    expect(within(screen.getByText('MAN-ROJO-01').closest('tr')!).getByText(s.noAlert)).toBeInTheDocument()
  })

  it('offers rolls only for PVC, and movements for both, as icon-only links with a name', async () => {
    listStock.mockResolvedValue(pageOf([stockItem(), clothStockItem()]))

    renderPage()

    const rolls = await screen.findByRole('link', { name: `${s.viewRolls} PVC-FLORAL-20` })
    expect(rolls).toHaveAttribute('href', '/inventario/rollos?producto=pvc-1')
    expect(rolls).toHaveTextContent('')
    expect(screen.queryByRole('link', { name: `${s.viewRolls} MAN-ROJO-01` })).not.toBeInTheDocument()
    expect(screen.getByRole('link', { name: `${s.viewMovements} MAN-ROJO-01` })).toHaveAttribute(
      'href',
      '/inventario/movimientos?producto=cloth-1',
    )
  })

  it('sends the filters to the API and returns to the first page', async () => {
    listStock.mockResolvedValue(pageOf([stockItem()]))
    const user = userEvent.setup()
    renderPage()
    await screen.findByText('PVC-FLORAL-20')

    await user.selectOptions(screen.getByLabelText(s.typeLabel), 'pvc')
    await user.click(screen.getByLabelText(s.onlyLowLabel))
    await user.type(screen.getByLabelText(s.searchLabel), 'floral')

    await vi.waitFor(() =>
      expect(listStock).toHaveBeenLastCalledWith(
        { type: 'pvc', onlyLow: true, query: 'floral', limit: 25, offset: 0 },
        expect.any(AbortSignal),
      ),
    )
  })

  it('explains an empty inventory and an empty search differently', async () => {
    listStock.mockResolvedValue(pageOf([]))
    const user = userEvent.setup()
    renderPage()

    expect(await screen.findByText(s.emptyTitle)).toBeInTheDocument()
    expect(screen.getByRole('link', { name: es.purchases.newPurchase })).toHaveAttribute('href', '/compras/nueva')

    await user.click(screen.getByLabelText(s.onlyLowLabel))
    expect(await screen.findByText(s.noResultsTitle)).toBeInTheDocument()
  })

  it('shows an error and retries', async () => {
    listStock.mockRejectedValueOnce(new ApiError(0, 'network')).mockResolvedValue(pageOf([stockItem()]))
    const user = userEvent.setup()
    renderPage()

    expect(await screen.findByRole('alert')).toHaveTextContent(es.inventory.loadError)
    await user.click(screen.getByRole('button', { name: es.common.retry }))

    expect(await screen.findByText('PVC-FLORAL-20')).toBeInTheDocument()
  })
})
