import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { ApiError } from '../../api/client'
import * as fetchAll from '../../api/fetchAll'
import * as api from '../../api/purchases'
import { es } from '../../i18n/es'
import { purchaseListOf, purchaseSummary, supplier } from '../../test/fixtures'
import { PurchaseListPage } from './PurchaseListPage'

vi.mock('../../api/purchases')
vi.mock('../../api/fetchAll')

const listPurchases = vi.mocked(api.listPurchases)
const t = es.purchases

function renderPage() {
  return render(
    <MemoryRouter>
      <PurchaseListPage />
    </MemoryRouter>,
  )
}

beforeEach(() => {
  vi.resetAllMocks()
  vi.mocked(fetchAll.listAllSuppliers).mockResolvedValue([supplier(), supplier({ id: 'sup-2', name: 'Textiles Norte' })])
})

describe('PurchaseListPage', () => {
  it('shows each purchase with a Spanish date and amount', async () => {
    listPurchases.mockResolvedValue(purchaseListOf([purchaseSummary()]))

    renderPage()

    expect(await screen.findByText('10/11/2026')).toBeInTheDocument()
    expect(screen.getByText('Plásticos del Sur', { selector: 'td' })).toBeInTheDocument()
    expect(screen.getByText('FAC-001')).toBeInTheDocument()
    expect(screen.getByText('Bs 1.234,50')).toBeInTheDocument()
    expect(screen.getByText(t.resultsSummary(1, 1))).toBeInTheDocument()
  })

  it('links to the detail with an icon-only action that still has a name', async () => {
    listPurchases.mockResolvedValue(purchaseListOf([purchaseSummary()]))

    renderPage()

    const link = await screen.findByRole('link', { name: `${t.view} FAC-001` })
    expect(link).toHaveAttribute('href', '/compras/pur-1')
    expect(link).toHaveAttribute('title', `${t.view} FAC-001`)
    expect(link).toHaveTextContent('')
  })

  it('names the action after supplier and date when there is no document reference', async () => {
    listPurchases.mockResolvedValue(purchaseListOf([purchaseSummary({ document_reference: '' })]))

    renderPage()

    expect(await screen.findByRole('link', { name: `${t.view} Plásticos del Sur 10/11/2026` })).toBeInTheDocument()
    expect(screen.getByText(es.common.noValue)).toBeInTheDocument()
  })

  it('shows that a purchase has no supplier', async () => {
    listPurchases.mockResolvedValue(purchaseListOf([purchaseSummary({ supplier: null, document_reference: '' })]))

    renderPage()

    expect(await screen.findByRole('link', { name: `${t.view} ${t.noSupplier} 10/11/2026` })).toBeInTheDocument()
    expect(screen.getByRole('cell', { name: t.noSupplier })).toBeInTheDocument()
  })

  it('shows a loading state first', () => {
    listPurchases.mockReturnValue(new Promise(() => {}))

    renderPage()

    expect(screen.getByRole('status')).toHaveTextContent(es.common.loading)
  })

  it('invites to register the first purchase when there are none', async () => {
    listPurchases.mockResolvedValue(purchaseListOf([]))

    renderPage()

    expect(await screen.findByRole('heading', { name: t.emptyTitle })).toBeInTheDocument()
  })

  it('shows a Spanish error with a retry button', async () => {
    const user = userEvent.setup()
    listPurchases.mockRejectedValueOnce(new ApiError(0, 'network')).mockResolvedValueOnce(purchaseListOf([purchaseSummary()]))

    renderPage()

    expect(await screen.findByRole('alert')).toHaveTextContent(es.apiErrors.network)
    await user.click(screen.getByRole('button', { name: es.common.retry }))
    expect(await screen.findByText('FAC-001')).toBeInTheDocument()
  })

  it('filters by supplier and date range and explains when nothing matches', async () => {
    const user = userEvent.setup()
    listPurchases.mockResolvedValueOnce(purchaseListOf([purchaseSummary()])).mockResolvedValue(purchaseListOf([]))

    renderPage()
    await screen.findByText('FAC-001')
    await screen.findByRole('option', { name: 'Textiles Norte' })
    await user.selectOptions(screen.getByLabelText(t.supplierFilterLabel), 'sup-2')
    await user.type(screen.getByLabelText(t.fromLabel), '2026-11-01')
    await user.type(screen.getByLabelText(t.toLabel), '2026-11-30')

    expect(await screen.findByRole('heading', { name: t.noResultsTitle })).toBeInTheDocument()
    expect(listPurchases.mock.calls.at(-1)?.[0]).toMatchObject({
      supplierId: 'sup-2',
      from: '2026-11-01',
      to: '2026-11-30',
      offset: 0,
    })
  })

  it('searches after the user stops typing', async () => {
    const user = userEvent.setup()
    listPurchases.mockResolvedValue(purchaseListOf([purchaseSummary()]))

    renderPage()
    await screen.findByText('FAC-001')
    await user.type(screen.getByLabelText(t.searchLabel), 'fac')

    await waitFor(() => expect(listPurchases.mock.calls.at(-1)?.[0]).toMatchObject({ query: 'fac' }))
  })

  it('still lists purchases if the supplier filter options cannot be loaded', async () => {
    vi.mocked(fetchAll.listAllSuppliers).mockRejectedValue(new ApiError(0, 'network'))
    listPurchases.mockResolvedValue(purchaseListOf([purchaseSummary()]))

    renderPage()

    expect(await screen.findByText('FAC-001')).toBeInTheDocument()
  })

  it('pages through results', async () => {
    const user = userEvent.setup()
    listPurchases.mockResolvedValue({ ...purchaseListOf([purchaseSummary()], 40), limit: 25 })

    renderPage()
    await screen.findByText('FAC-001')
    expect(screen.getByRole('button', { name: es.common.previous })).toBeDisabled()
    await user.click(screen.getByRole('button', { name: es.common.next }))

    await waitFor(() => expect(listPurchases.mock.calls.at(-1)?.[0]).toMatchObject({ offset: 25, limit: 25 }))
  })
})
