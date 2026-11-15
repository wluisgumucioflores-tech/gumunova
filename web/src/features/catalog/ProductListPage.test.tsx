import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { ApiError } from '../../api/client'
import * as api from '../../api/products'
import { es } from '../../i18n/es'
import { listOf, pvcProduct, tableclothProduct } from '../../test/fixtures'
import { ProductListPage } from './ProductListPage'

vi.mock('../../api/products')

const listProducts = vi.mocked(api.listProducts)
const deactivateProduct = vi.mocked(api.deactivateProduct)
const activateProduct = vi.mocked(api.activateProduct)

function renderPage() {
  return render(
    <MemoryRouter>
      <ProductListPage />
    </MemoryRouter>,
  )
}

beforeEach(() => {
  vi.resetAllMocks()
})

describe('ProductListPage', () => {
  it('shows the products with Spanish prices and type labels', async () => {
    listProducts.mockResolvedValue(listOf([pvcProduct(), tableclothProduct()]))

    renderPage()

    expect(await screen.findByText('PVC-FLORAL-20')).toBeInTheDocument()
    expect(screen.getByText('Bs 15,50 / metro')).toBeInTheDocument()
    expect(screen.getByText('Bs 1.280,00 / rollo')).toBeInTheDocument()
    expect(screen.getByText('Bs 60,00 / unidad')).toBeInTheDocument()
    expect(screen.getAllByText(es.catalog.status.active)).toHaveLength(2)
    expect(screen.getByText(es.catalog.resultsSummary(2, 2))).toBeInTheDocument()
  })

  it('shows a loading state first', () => {
    listProducts.mockReturnValue(new Promise(() => {}))

    renderPage()

    expect(screen.getByRole('status')).toHaveTextContent(es.common.loading)
  })

  it('invites to create the first product when the catalog is empty', async () => {
    listProducts.mockResolvedValue(listOf([]))

    renderPage()

    expect(await screen.findByRole('heading', { name: es.catalog.emptyTitle })).toBeInTheDocument()
    expect(screen.getAllByRole('link', { name: es.catalog.newProduct }).length).toBeGreaterThan(0)
  })

  it('shows a Spanish error with a retry button', async () => {
    const user = userEvent.setup()
    listProducts.mockRejectedValueOnce(new ApiError(0, 'network')).mockResolvedValueOnce(listOf([pvcProduct()]))

    renderPage()

    expect(await screen.findByRole('alert')).toHaveTextContent(es.apiErrors.network)
    await user.click(screen.getByRole('button', { name: es.common.retry }))

    expect(await screen.findByText('PVC-FLORAL-20')).toBeInTheDocument()
    expect(listProducts).toHaveBeenCalledTimes(2)
  })

  it('filters by type and status and explains when nothing matches', async () => {
    const user = userEvent.setup()
    listProducts.mockResolvedValueOnce(listOf([pvcProduct()])).mockResolvedValue(listOf([]))

    renderPage()
    await screen.findByText('PVC-FLORAL-20')

    await user.selectOptions(screen.getByLabelText(es.catalog.typeFilterLabel), 'tablecloth')
    await user.selectOptions(screen.getByLabelText(es.catalog.statusFilterLabel), 'inactive')

    expect(await screen.findByRole('heading', { name: es.catalog.noResultsTitle })).toBeInTheDocument()
    const lastFilters = listProducts.mock.calls.at(-1)?.[0]
    expect(lastFilters).toMatchObject({ type: 'tablecloth', isActive: false, offset: 0 })
  })

  it('searches after the user stops typing', async () => {
    const user = userEvent.setup()
    listProducts.mockResolvedValue(listOf([pvcProduct()]))

    renderPage()
    await screen.findByText('PVC-FLORAL-20')
    await user.type(screen.getByLabelText(es.catalog.searchLabel), 'floral')

    await waitFor(() => expect(listProducts.mock.calls.at(-1)?.[0]).toMatchObject({ query: 'floral' }))
  })

  it('asks for confirmation before deactivating and then reloads', async () => {
    const user = userEvent.setup()
    listProducts.mockResolvedValue(listOf([pvcProduct()]))
    deactivateProduct.mockResolvedValue(pvcProduct({ is_active: false }))

    renderPage()
    await screen.findByText('PVC-FLORAL-20')

    await user.click(screen.getByRole('button', { name: new RegExp(es.catalog.deactivate) }))
    expect(deactivateProduct).not.toHaveBeenCalled()
    const group = screen.getByRole('group')
    expect(group).toHaveTextContent('PVC floral 20 m')

    await user.click(within(group).getByRole('button', { name: es.catalog.confirmDeactivateYes }))

    await waitFor(() => expect(deactivateProduct).toHaveBeenCalledWith('pvc-1'))
    await waitFor(() => expect(listProducts).toHaveBeenCalledTimes(2))
  })

  it('lets the user back out of a deactivation', async () => {
    const user = userEvent.setup()
    listProducts.mockResolvedValue(listOf([pvcProduct()]))

    renderPage()
    await screen.findByText('PVC-FLORAL-20')
    await user.click(screen.getByRole('button', { name: new RegExp(es.catalog.deactivate) }))
    await user.click(within(screen.getByRole('group')).getByRole('button', { name: es.common.cancel }))

    expect(deactivateProduct).not.toHaveBeenCalled()
    expect(screen.queryByRole('group')).not.toBeInTheDocument()
  })

  it('activates an inactive product without confirmation', async () => {
    const user = userEvent.setup()
    listProducts.mockResolvedValue(listOf([pvcProduct({ is_active: false })]))
    activateProduct.mockResolvedValue(pvcProduct())

    renderPage()
    await screen.findByText(es.catalog.status.inactive)
    await user.click(screen.getByRole('button', { name: new RegExp(es.catalog.activate) }))

    await waitFor(() => expect(activateProduct).toHaveBeenCalledWith('pvc-1'))
  })

  it('reports a failed status change in Spanish', async () => {
    const user = userEvent.setup()
    listProducts.mockResolvedValue(listOf([pvcProduct()]))
    deactivateProduct.mockRejectedValue(new ApiError(404, 'product_not_found'))

    renderPage()
    await screen.findByText('PVC-FLORAL-20')
    await user.click(screen.getByRole('button', { name: new RegExp(es.catalog.deactivate) }))
    await user.click(screen.getByRole('button', { name: es.catalog.confirmDeactivateYes }))

    expect(await screen.findByRole('alert')).toHaveTextContent(es.apiErrors.product_not_found)
  })

  it('pages through results', async () => {
    const user = userEvent.setup()
    listProducts.mockResolvedValue({ ...listOf([pvcProduct()], 40), limit: 25 })

    renderPage()
    await screen.findByText('PVC-FLORAL-20')
    expect(screen.getByRole('button', { name: es.common.previous })).toBeDisabled()

    await user.click(screen.getByRole('button', { name: es.common.next }))

    await waitFor(() => expect(listProducts.mock.calls.at(-1)?.[0]).toMatchObject({ offset: 25, limit: 25 }))
  })
})

describe('selecting products for labels', () => {
  it('links to the labels of the ticked products and keeps the count', async () => {
    const user = userEvent.setup()
    listProducts.mockResolvedValue(listOf([pvcProduct(), tableclothProduct()]))
    renderPage()

    expect(screen.queryByRole('group', { name: /seleccionado/ })).not.toBeInTheDocument()
    await user.click(await screen.findByRole('checkbox', { name: es.catalog.labels.selectRow('Mantel rojo') }))

    const bar = screen.getByRole('group', { name: es.catalog.labels.selectedCount(1) })
    expect(within(bar).getByRole('link', { name: es.catalog.labels.open })).toHaveAttribute('href', '/catalogo/etiquetas?ids=cloth-1')

    await user.click(within(bar).getByRole('button', { name: es.catalog.labels.clearSelection }))
    expect(screen.queryByRole('group', { name: /seleccionado/ })).not.toBeInTheDocument()
  })

  it('selects every product of the page at once', async () => {
    const user = userEvent.setup()
    listProducts.mockResolvedValue(listOf([pvcProduct(), tableclothProduct()]))
    renderPage()

    await user.click(await screen.findByRole('checkbox', { name: es.catalog.labels.selectPage }))

    expect(screen.getByRole('link', { name: es.catalog.labels.open })).toHaveAttribute('href', '/catalogo/etiquetas?ids=pvc-1,cloth-1')
    await user.click(screen.getByRole('checkbox', { name: es.catalog.labels.selectPage }))
    expect(screen.queryByRole('link', { name: es.catalog.labels.open })).not.toBeInTheDocument()
  })

  it('always offers the labels of every product', async () => {
    listProducts.mockResolvedValue(listOf([pvcProduct()]))
    renderPage()

    expect(await screen.findByRole('link', { name: es.catalog.labels.openAll })).toHaveAttribute('href', '/catalogo/etiquetas?todos=1')
  })
})

describe('regenerating the label code', () => {
  it('asks first, then regenerates and says to print the label again', async () => {
    const user = userEvent.setup()
    listProducts.mockResolvedValue(listOf([pvcProduct()]))
    vi.mocked(api.resetProductLabelCode).mockResolvedValue(pvcProduct({ label_code: 'P-NEW' }))
    renderPage()

    await user.click(await screen.findByRole('button', { name: `${es.catalog.labels.reset} PVC floral 20 m` }))
    expect(api.resetProductLabelCode).not.toHaveBeenCalled()
    expect(screen.getByText(es.catalog.labels.confirmReset('PVC floral 20 m'))).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: es.catalog.labels.confirmResetYes }))

    expect(api.resetProductLabelCode).toHaveBeenCalledWith('pvc-1')
    expect(await screen.findByText(es.catalog.labels.resetDone('PVC floral 20 m'))).toBeInTheDocument()
  })

  it('keeps the code when the person cancels, and shows an error when it fails', async () => {
    const user = userEvent.setup()
    listProducts.mockResolvedValue(listOf([pvcProduct()]))
    vi.mocked(api.resetProductLabelCode).mockRejectedValue(new ApiError(0, 'network'))
    renderPage()

    await user.click(await screen.findByRole('button', { name: `${es.catalog.labels.reset} PVC floral 20 m` }))
    await user.click(screen.getByRole('button', { name: es.common.cancel }))
    expect(api.resetProductLabelCode).not.toHaveBeenCalled()

    await user.click(screen.getByRole('button', { name: `${es.catalog.labels.reset} PVC floral 20 m` }))
    await user.click(screen.getByRole('button', { name: es.catalog.labels.confirmResetYes }))
    expect(await screen.findByRole('alert')).toHaveTextContent(es.apiErrors.network)
  })
})

