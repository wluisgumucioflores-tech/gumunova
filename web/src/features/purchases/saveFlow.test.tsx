import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { App } from '../../App'
import { ApiError } from '../../api/client'
import * as fetchAll from '../../api/fetchAll'
import * as api from '../../api/purchases'
import { es } from '../../i18n/es'
import { purchase, purchaseListOf, purchaseSummary, pvcProduct, supplier, tableclothProduct } from '../../test/fixtures'

vi.mock('../../api/purchases')
vi.mock('../../api/fetchAll')

const t = es.purchases.form

function renderAt(path: string) {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <App />
    </MemoryRouter>,
  )
}

function mockFormData({ suppliers = [supplier()], pvc = [pvcProduct()], cloth = [tableclothProduct()] } = {}) {
  vi.mocked(fetchAll.listAllActiveSuppliers).mockResolvedValue(suppliers)
  vi.mocked(fetchAll.listAllActiveProducts).mockImplementation(async (type) => (type === 'pvc' ? pvc : cloth))
  vi.mocked(fetchAll.listAllSuppliers).mockResolvedValue(suppliers)
}

async function fillAndRegister(user: ReturnType<typeof userEvent.setup>) {
  await user.selectOptions(await screen.findByLabelText(t.fields.supplier), 'sup-1')
  await user.click(screen.getByRole('button', { name: t.addTablecloth }))
  await user.selectOptions(screen.getByLabelText(t.productLabel), 'cloth-1')
  await user.type(screen.getByLabelText(t.quantity), '10')
  await user.type(screen.getByLabelText(t.unitCost), '30')
  await user.click(screen.getByRole('button', { name: t.submit }))
}

beforeEach(() => {
  vi.resetAllMocks()
})

describe('registering a purchase', () => {
  it('goes to the purchase list with a notification once the server confirms', async () => {
    const user = userEvent.setup()
    mockFormData()
    vi.mocked(api.registerPurchase).mockResolvedValue({ purchase: purchase(), replayed: false })
    vi.mocked(api.listPurchases).mockResolvedValue(purchaseListOf([purchaseSummary()]))
    renderAt('/compras/nueva')

    await fillAndRegister(user)

    expect(await screen.findByText('FAC-001')).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: es.purchases.title })).toBeInTheDocument()
    expect(screen.getByText(es.purchases.createdNotice)).toBeInTheDocument()
    expect(api.registerPurchase).toHaveBeenCalledTimes(1)
    const [input, key] = vi.mocked(api.registerPurchase).mock.calls[0]!
    expect(input.lines).toEqual([{ product_id: 'cloth-1', quantity: 10, unit_cost: '30.00' }])
    expect(key).toMatch(/^[A-Za-z0-9._:-]{8,100}$/)
  })

  it('treats a replayed registration as success: the purchase already existed', async () => {
    const user = userEvent.setup()
    mockFormData()
    vi.mocked(api.registerPurchase).mockResolvedValue({ purchase: purchase(), replayed: true })
    vi.mocked(api.listPurchases).mockResolvedValue(purchaseListOf([purchaseSummary()]))
    renderAt('/compras/nueva')

    await fillAndRegister(user)

    expect(await screen.findByText(es.purchases.createdNotice)).toBeInTheDocument()
  })

  it('stays on the form with the data when the registration fails', async () => {
    const user = userEvent.setup()
    mockFormData()
    vi.mocked(api.registerPurchase).mockRejectedValue(new ApiError(0, 'network'))
    renderAt('/compras/nueva')

    await fillAndRegister(user)

    expect(await screen.findByRole('alert')).toHaveTextContent(es.apiErrors.network)
    expect(screen.getByLabelText(t.quantity)).toHaveValue('10')
    expect(api.listPurchases).not.toHaveBeenCalled()
  })

  it('starts with an empty form and a fresh key the next time', async () => {
    const user = userEvent.setup()
    mockFormData()
    vi.mocked(api.registerPurchase).mockResolvedValue({ purchase: purchase(), replayed: false })
    vi.mocked(api.listPurchases).mockResolvedValue(purchaseListOf([purchaseSummary()]))
    renderAt('/compras/nueva')
    await fillAndRegister(user)
    await screen.findByText('FAC-001')

    await user.click(screen.getAllByRole('link', { name: es.purchases.newPurchase })[0]!)
    await fillAndRegister(user)
    await waitFor(() => expect(api.registerPurchase).toHaveBeenCalledTimes(2))

    expect(vi.mocked(api.registerPurchase).mock.calls[1]![1]).not.toBe(vi.mocked(api.registerPurchase).mock.calls[0]![1])
  })
})

describe('the purchase form when there is nothing to buy from or buy', () => {
  it('still offers the form when no supplier exists, because the supplier is optional', async () => {
    mockFormData({ suppliers: [] })
    renderAt('/compras/nueva')

    expect(await screen.findByRole('button', { name: t.submit })).toBeInTheDocument()
    expect(screen.getByLabelText(t.fields.supplier)).toHaveValue('')
  })

  it('asks for catalog products first', async () => {
    mockFormData({ pvc: [], cloth: [] })
    renderAt('/compras/nueva')

    expect(await screen.findByText(t.noProducts)).toBeInTheDocument()
    expect(screen.getByRole('link', { name: es.catalog.newProduct })).toHaveAttribute('href', '/catalogo/nuevo')
  })

  it('shows a Spanish error with a retry when the options cannot be loaded', async () => {
    const user = userEvent.setup()
    vi.mocked(fetchAll.listAllActiveSuppliers).mockRejectedValueOnce(new ApiError(0, 'network'))
    vi.mocked(fetchAll.listAllActiveProducts).mockResolvedValue([])
    renderAt('/compras/nueva')

    expect(await screen.findByRole('alert')).toHaveTextContent(es.apiErrors.network)
    vi.mocked(fetchAll.listAllActiveSuppliers).mockResolvedValue([supplier()])
    vi.mocked(fetchAll.listAllActiveProducts).mockImplementation(async (type) => (type === 'pvc' ? [pvcProduct()] : []))
    await user.click(screen.getByRole('button', { name: es.common.retry }))

    expect(await screen.findByLabelText(t.fields.supplier)).toBeInTheDocument()
  })
})

describe('navigation', () => {
  it('has a Compras section in the menu', async () => {
    vi.mocked(api.listPurchases).mockResolvedValue(purchaseListOf([]))
    vi.mocked(fetchAll.listAllSuppliers).mockResolvedValue([])
    renderAt('/compras')

    const menu = screen.getByRole('navigation', { name: es.app.navigationLabel })
    expect(menu).toHaveTextContent(es.app.nav.purchases)
    expect(await screen.findByRole('heading', { name: es.purchases.emptyTitle })).toBeInTheDocument()
  })
})

describe('editing a purchase', () => {
  it('shows the stored purchase in the form and saves the correction', async () => {
    const user = userEvent.setup()
    mockFormData()
    vi.mocked(api.getPurchase).mockResolvedValue(purchase())
    vi.mocked(api.updatePurchase).mockResolvedValue(purchase())
    vi.mocked(api.listPurchases).mockResolvedValue(purchaseListOf([purchaseSummary()]))
    renderAt('/compras/pur-1/editar')

    expect(await screen.findByRole('heading', { name: t.editTitle })).toBeInTheDocument()
    expect(screen.getByText(t.editNotice)).toBeInTheDocument()
    expect(screen.getByLabelText(t.fields.reference, { exact: false })).toHaveValue('FAC-001')
    expect(screen.getByLabelText(t.rollCount)).toHaveValue('2')
    expect(screen.getByLabelText(t.quantity)).toHaveValue('10')

    await user.clear(screen.getByLabelText(t.quantity))
    await user.type(screen.getByLabelText(t.quantity), '12')
    await user.click(screen.getByRole('button', { name: t.save }))

    expect(await screen.findByText(es.purchases.updatedNotice)).toBeInTheDocument()
    const [id, input] = vi.mocked(api.updatePurchase).mock.calls[0]!
    expect(id).toBe('pur-1')
    expect(input.document_reference).toBe('FAC-001')
    expect(input.lines).toEqual([
      {
        product_id: 'pvc-1',
        rolls: [
          { length_m: '19.80', cost: '280.00' },
          { length_m: '20.00', cost: '300.00' },
        ],
      },
      { product_id: 'cloth-1', quantity: 12, unit_cost: '30.00' },
    ])
    expect(api.registerPurchase).not.toHaveBeenCalled()
  })

  it('only lets the data change once stock from the purchase was sold', async () => {
    mockFormData()
    vi.mocked(api.getPurchase).mockResolvedValue(purchase({ items_editable: false }))
    renderAt('/compras/pur-1/editar')

    expect(await screen.findByText(t.itemsLockedNotice)).toBeInTheDocument()
    expect(screen.getByLabelText(t.fields.reference, { exact: false })).toBeEnabled()
    expect(screen.getByLabelText(t.quantity)).toBeDisabled()
    expect(screen.getByLabelText(t.fields.additionalCosts, { exact: false })).toBeDisabled()
    expect(screen.getByRole('button', { name: t.addTablecloth })).toBeDisabled()
  })

  it('keeps the form and explains when the server says the items are locked', async () => {
    const user = userEvent.setup()
    mockFormData()
    vi.mocked(api.getPurchase).mockResolvedValue(purchase())
    vi.mocked(api.updatePurchase).mockRejectedValue(new ApiError(409, 'purchase_items_locked'))
    renderAt('/compras/pur-1/editar')

    await user.click(await screen.findByRole('button', { name: t.save }))

    expect(await screen.findByRole('alert')).toHaveTextContent(es.apiErrors.purchase_items_locked)
    expect(screen.getByLabelText(t.quantity)).toHaveValue('10')
  })

  it('keeps a product that was deactivated after the purchase selectable', async () => {
    mockFormData({ pvc: [], cloth: [] })
    vi.mocked(api.getPurchase).mockResolvedValue(purchase())
    renderAt('/compras/pur-1/editar')

    expect(await screen.findByLabelText(t.rollCount)).toBeInTheDocument()
    const selects = screen.getAllByLabelText(t.productLabel)
    expect(selects[0]).toHaveValue('pvc-1')
    expect(selects[1]).toHaveValue('cloth-1')
  })
})
