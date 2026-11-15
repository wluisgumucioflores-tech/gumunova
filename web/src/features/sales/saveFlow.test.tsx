import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { App } from '../../App'
import { ApiError } from '../../api/client'
import * as fetchAll from '../../api/fetchAll'
import * as api from '../../api/sales'
import { es } from '../../i18n/es'
import { clothStockItem, pageOf, pvcProduct, roll, sale, saleSummary, tableclothProduct } from '../../test/fixtures'

vi.mock('../../api/sales')
vi.mock('../../api/fetchAll')

const t = es.sales.form

function renderAt(path: string) {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <App />
    </MemoryRouter>,
  )
}

function mockFormData({ rolls = [roll()], cloths = [clothStockItem({ quantity: '10.00' })] } = {}) {
  vi.mocked(fetchAll.listAllAvailableRolls).mockResolvedValue(rolls)
  vi.mocked(fetchAll.listAllTableclothStock).mockResolvedValue(cloths)
  vi.mocked(fetchAll.listAllActiveProducts).mockImplementation(async (type) => (type === 'pvc' ? [pvcProduct()] : [tableclothProduct()]))
}

async function fillAndRegister(user: ReturnType<typeof userEvent.setup>) {
  await user.click(await screen.findByRole('button', { name: t.addTablecloth }))
  await user.selectOptions(screen.getByLabelText(t.productLabel), 'cloth-1')
  await user.type(screen.getByLabelText(t.quantity), '4')
  await user.click(screen.getByRole('button', { name: t.submit }))
}

beforeEach(() => {
  vi.resetAllMocks()
})

describe('registering a sale', () => {
  it('goes to the sale list with a notification once the server confirms', async () => {
    const user = userEvent.setup()
    mockFormData()
    vi.mocked(api.registerSale).mockResolvedValue({ sale: sale(), replayed: false })
    vi.mocked(api.listSales).mockResolvedValue(pageOf([saleSummary()]))
    renderAt('/ventas/nueva')

    await fillAndRegister(user)

    expect(await screen.findByText('10/11/2026')).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: es.sales.title })).toBeInTheDocument()
    expect(screen.getByText(es.sales.createdNotice)).toBeInTheDocument()
    expect(api.registerSale).toHaveBeenCalledTimes(1)
    const [input, key] = vi.mocked(api.registerSale).mock.calls[0]!
    expect(input.lines).toEqual([{ kind: 'tablecloth_units', product_id: 'cloth-1', quantity: 4, unit_price: '60.00' }])
    expect(key).toMatch(/^[A-Za-z0-9._:-]{8,100}$/)
  })

  it('stays on the form with the error and does not confirm when the server refuses', async () => {
    const user = userEvent.setup()
    mockFormData()
    vi.mocked(api.registerSale).mockRejectedValue(
      new ApiError(422, 'validation_failed', [{ field: 'lines[0].quantity', code: 'insufficient_stock' }]),
    )
    renderAt('/ventas/nueva')

    await fillAndRegister(user)

    expect(await screen.findByText(es.fieldErrors.insufficient_stock)).toBeInTheDocument()
    expect(screen.queryByText(es.sales.createdNotice)).not.toBeInTheDocument()
    expect(screen.getByRole('heading', { name: t.title })).toBeInTheDocument()
  })

  it('says there is nothing to sell instead of showing an unusable form', async () => {
    mockFormData({ rolls: [], cloths: [clothStockItem({ quantity: '0.00' })] })
    renderAt('/ventas/nueva')

    expect(await screen.findByText(t.nothingToSell)).toBeInTheDocument()
    expect(screen.getByRole('link', { name: es.purchases.newPurchase })).toHaveAttribute('href', '/compras/nueva')
    expect(screen.queryByRole('button', { name: t.submit })).not.toBeInTheDocument()
  })

  it('shows a loading error and retries', async () => {
    const user = userEvent.setup()
    mockFormData()
    vi.mocked(fetchAll.listAllAvailableRolls).mockRejectedValueOnce(new ApiError(0, 'network'))
    renderAt('/ventas/nueva')

    expect(await screen.findByRole('alert')).toHaveTextContent(t.loadError)
    vi.mocked(fetchAll.listAllAvailableRolls).mockResolvedValue([roll()])
    await user.click(screen.getByRole('button', { name: es.common.retry }))

    expect(await screen.findByRole('button', { name: t.submit })).toBeInTheDocument()
  })

  it('is reachable from the navigation', async () => {
    vi.mocked(api.listSales).mockResolvedValue(pageOf([]))
    const user = userEvent.setup()
    renderAt('/catalogo')

    await waitFor(() => expect(screen.getByRole('link', { name: es.app.nav.sales })).toBeInTheDocument())
    await user.click(screen.getByRole('link', { name: es.app.nav.sales }))

    expect(await screen.findByRole('heading', { name: es.sales.title })).toBeInTheDocument()
  })
})
