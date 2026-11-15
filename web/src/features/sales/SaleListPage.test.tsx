import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { ApiError } from '../../api/client'
import * as api from '../../api/sales'
import { es } from '../../i18n/es'
import { pageOf, saleSummary } from '../../test/fixtures'
import { SaleListPage } from './SaleListPage'

vi.mock('../../api/sales')

const listSales = vi.mocked(api.listSales)
const t = es.sales

function renderPage() {
  return render(
    <MemoryRouter>
      <SaleListPage />
    </MemoryRouter>,
  )
}

beforeEach(() => {
  vi.resetAllMocks()
})

describe('SaleListPage', () => {
  it('shows each sale with a Spanish date, total and gross profit', async () => {
    listSales.mockResolvedValue(pageOf([saleSummary()]))

    renderPage()

    expect(await screen.findByText('10/11/2026')).toBeInTheDocument()
    expect(screen.getByText('Bs 305,00')).toBeInTheDocument()
    expect(screen.getByText('Bs 143,35')).toBeInTheDocument()
    expect(screen.getByText(t.resultsSummary(1, 1))).toBeInTheDocument()
  })

  it('labels every cell with its column so the table can become cards on a phone', async () => {
    listSales.mockResolvedValue(pageOf([saleSummary()]))

    renderPage()

    const row = (await screen.findByText('10/11/2026')).closest('tr')!
    const headers = screen.getAllByRole('columnheader').map((header) => header.textContent)
    expect(Array.from(row.querySelectorAll('td')).map((cell) => cell.getAttribute('data-label'))).toEqual(headers)
  })

  it('links to the detail with an icon-only action that still has a name', async () => {
    listSales.mockResolvedValue(pageOf([saleSummary()]))

    renderPage()

    const link = await screen.findByRole('link', { name: `${t.view} 10/11/2026` })
    expect(link).toHaveAttribute('href', '/ventas/sale-1')
    expect(link).toHaveTextContent('')
  })

  it('sends the filters to the API', async () => {
    listSales.mockResolvedValue(pageOf([saleSummary()]))
    const user = userEvent.setup()
    renderPage()
    await screen.findByText('10/11/2026')

    await user.type(screen.getByLabelText(t.fromLabel), '2026-11-01')
    await user.type(screen.getByLabelText(t.searchLabel), 'ROL-000001')

    await vi.waitFor(() =>
      expect(listSales).toHaveBeenLastCalledWith(
        { from: '2026-11-01', to: undefined, query: 'ROL-000001', limit: 25, offset: 0 },
        expect.any(AbortSignal),
      ),
    )
  })

  it('explains an empty list and an empty search differently', async () => {
    listSales.mockResolvedValue(pageOf([]))
    const user = userEvent.setup()
    renderPage()

    expect(await screen.findByText(t.emptyTitle)).toBeInTheDocument()
    expect(screen.getAllByRole('link', { name: t.newSale })[0]).toHaveAttribute('href', '/ventas/nueva')

    await user.type(screen.getByLabelText(t.fromLabel), '2026-11-01')
    expect(await screen.findByText(t.noResultsTitle)).toBeInTheDocument()
  })

  it('shows an error and retries', async () => {
    listSales.mockRejectedValueOnce(new ApiError(0, 'network')).mockResolvedValue(pageOf([saleSummary()]))
    const user = userEvent.setup()
    renderPage()

    expect(await screen.findByRole('alert')).toHaveTextContent(t.loadError)
    await user.click(screen.getByRole('button', { name: es.common.retry }))

    expect(await screen.findByText('10/11/2026')).toBeInTheDocument()
  })
})
