import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { ApiError } from '../../api/client'
import * as api from '../../api/suppliers'
import { es } from '../../i18n/es'
import { supplier, supplierListOf } from '../../test/fixtures'
import { SupplierListPage } from './SupplierListPage'

vi.mock('../../api/suppliers')

const listSuppliers = vi.mocked(api.listSuppliers)
const deactivateSupplier = vi.mocked(api.deactivateSupplier)
const activateSupplier = vi.mocked(api.activateSupplier)
const t = es.suppliers

function renderPage() {
  return render(
    <MemoryRouter>
      <SupplierListPage />
    </MemoryRouter>,
  )
}

beforeEach(() => {
  vi.resetAllMocks()
})

describe('SupplierListPage', () => {
  it('shows the suppliers, with a dash for missing phone or email', async () => {
    listSuppliers.mockResolvedValue(
      supplierListOf([supplier(), supplier({ id: 'sup-2', name: 'Sin datos', phone: '', email: '' })]),
    )

    renderPage()

    expect(await screen.findByText('Plásticos del Sur')).toBeInTheDocument()
    expect(screen.getByText('ventas@sur.com')).toBeInTheDocument()
    expect(screen.getAllByText(es.common.noValue)).toHaveLength(2)
    expect(screen.getByText(t.resultsSummary(2, 2))).toBeInTheDocument()
  })

  it('shows a loading state first', () => {
    listSuppliers.mockReturnValue(new Promise(() => {}))

    renderPage()

    expect(screen.getByRole('status')).toHaveTextContent(es.common.loading)
  })

  it('invites to create the first supplier when there are none', async () => {
    listSuppliers.mockResolvedValue(supplierListOf([]))

    renderPage()

    expect(await screen.findByRole('heading', { name: t.emptyTitle })).toBeInTheDocument()
  })

  it('shows a Spanish error with a retry button', async () => {
    const user = userEvent.setup()
    listSuppliers.mockRejectedValueOnce(new ApiError(0, 'network')).mockResolvedValueOnce(supplierListOf([supplier()]))

    renderPage()

    expect(await screen.findByRole('alert')).toHaveTextContent(es.apiErrors.network)
    await user.click(screen.getByRole('button', { name: es.common.retry }))

    expect(await screen.findByText('Plásticos del Sur')).toBeInTheDocument()
  })

  it('filters by status and explains when nothing matches', async () => {
    const user = userEvent.setup()
    listSuppliers.mockResolvedValueOnce(supplierListOf([supplier()])).mockResolvedValue(supplierListOf([]))

    renderPage()
    await screen.findByText('Plásticos del Sur')
    await user.selectOptions(screen.getByLabelText(es.common.statusFilterLabel), 'inactive')

    expect(await screen.findByRole('heading', { name: t.noResultsTitle })).toBeInTheDocument()
    expect(listSuppliers.mock.calls.at(-1)?.[0]).toMatchObject({ isActive: false, offset: 0 })
  })

  it('searches after the user stops typing', async () => {
    const user = userEvent.setup()
    listSuppliers.mockResolvedValue(supplierListOf([supplier()]))

    renderPage()
    await screen.findByText('Plásticos del Sur')
    await user.type(screen.getByLabelText(t.searchLabel), 'sur')

    await waitFor(() => expect(listSuppliers.mock.calls.at(-1)?.[0]).toMatchObject({ query: 'sur' }))
  })

  it('asks for confirmation before deactivating and then reloads', async () => {
    const user = userEvent.setup()
    listSuppliers.mockResolvedValue(supplierListOf([supplier()]))
    deactivateSupplier.mockResolvedValue(supplier({ is_active: false }))

    renderPage()
    await screen.findByText('Plásticos del Sur')

    await user.click(screen.getByRole('button', { name: `${t.deactivate} Plásticos del Sur` }))
    expect(deactivateSupplier).not.toHaveBeenCalled()
    const group = screen.getByRole('group')
    await user.click(within(group).getByRole('button', { name: t.confirmDeactivateYes }))

    await waitFor(() => expect(deactivateSupplier).toHaveBeenCalledWith('sup-1'))
    await waitFor(() => expect(listSuppliers).toHaveBeenCalledTimes(2))
  })

  it('lets the user back out of a deactivation', async () => {
    const user = userEvent.setup()
    listSuppliers.mockResolvedValue(supplierListOf([supplier()]))

    renderPage()
    await screen.findByText('Plásticos del Sur')
    await user.click(screen.getByRole('button', { name: `${t.deactivate} Plásticos del Sur` }))
    await user.click(within(screen.getByRole('group')).getByRole('button', { name: es.common.cancel }))

    expect(deactivateSupplier).not.toHaveBeenCalled()
    expect(screen.queryByRole('group')).not.toBeInTheDocument()
  })

  it('activates an inactive supplier without confirmation', async () => {
    const user = userEvent.setup()
    listSuppliers.mockResolvedValue(supplierListOf([supplier({ is_active: false })]))
    activateSupplier.mockResolvedValue(supplier())

    renderPage()
    await screen.findByText(es.common.statusInactive)
    await user.click(screen.getByRole('button', { name: `${t.activate} Plásticos del Sur` }))

    await waitFor(() => expect(activateSupplier).toHaveBeenCalledWith('sup-1'))
  })

  it('reports a failed status change in Spanish', async () => {
    const user = userEvent.setup()
    listSuppliers.mockResolvedValue(supplierListOf([supplier()]))
    deactivateSupplier.mockRejectedValue(new ApiError(404, 'supplier_not_found'))

    renderPage()
    await screen.findByText('Plásticos del Sur')
    await user.click(screen.getByRole('button', { name: `${t.deactivate} Plásticos del Sur` }))
    await user.click(screen.getByRole('button', { name: t.confirmDeactivateYes }))

    expect(await screen.findByRole('alert')).toHaveTextContent(es.apiErrors.supplier_not_found)
  })

  it('pages through results', async () => {
    const user = userEvent.setup()
    listSuppliers.mockResolvedValue({ ...supplierListOf([supplier()], 40), limit: 25 })

    renderPage()
    await screen.findByText('Plásticos del Sur')
    expect(screen.getByRole('button', { name: es.common.previous })).toBeDisabled()
    await user.click(screen.getByRole('button', { name: es.common.next }))

    await waitFor(() => expect(listSuppliers.mock.calls.at(-1)?.[0]).toMatchObject({ offset: 25, limit: 25 }))
  })
})
