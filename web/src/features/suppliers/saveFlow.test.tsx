import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { App } from '../../App'
import { ApiError } from '../../api/client'
import * as api from '../../api/suppliers'
import { es } from '../../i18n/es'
import { supplier, supplierListOf } from '../../test/fixtures'

vi.mock('../../api/suppliers')

const f = es.suppliers.fields

function renderAt(path: string) {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <App />
    </MemoryRouter>,
  )
}

beforeEach(() => {
  vi.resetAllMocks()
})

describe('saving a supplier', () => {
  it('returns to the table with a notification after creating', async () => {
    const user = userEvent.setup()
    vi.mocked(api.listSuppliers).mockResolvedValue(supplierListOf([supplier()]))
    vi.mocked(api.createSupplier).mockResolvedValue(supplier())
    renderAt('/proveedores/nuevo')

    await user.type(screen.getByLabelText(f.name), 'Plásticos del Sur')
    await user.click(screen.getByRole('button', { name: es.common.save }))

    expect(await screen.findByText('ventas@sur.com')).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: es.suppliers.title })).toBeInTheDocument()
    expect(screen.getByText(es.suppliers.createdNotice)).toBeInTheDocument()
    expect(api.createSupplier).toHaveBeenCalledWith({ name: 'Plásticos del Sur', phone: '', email: '', notes: '' })
  })

  it('starts with an empty form the next time', async () => {
    const user = userEvent.setup()
    vi.mocked(api.listSuppliers).mockResolvedValue(supplierListOf([supplier()]))
    vi.mocked(api.createSupplier).mockResolvedValue(supplier())
    renderAt('/proveedores/nuevo')
    await user.type(screen.getByLabelText(f.name), 'Plásticos del Sur')
    await user.click(screen.getByRole('button', { name: es.common.save }))
    await screen.findByText('ventas@sur.com')

    await user.click(screen.getAllByRole('link', { name: es.suppliers.newSupplier })[0]!)

    expect(await screen.findByLabelText(f.name)).toHaveValue('')
  })

  it('stays on the form when saving fails', async () => {
    const user = userEvent.setup()
    vi.mocked(api.createSupplier).mockRejectedValue(new ApiError(0, 'network'))
    renderAt('/proveedores/nuevo')

    await user.type(screen.getByLabelText(f.name), 'Plásticos del Sur')
    await user.click(screen.getByRole('button', { name: es.common.save }))

    expect(await screen.findByRole('alert')).toHaveTextContent(es.apiErrors.network)
    expect(screen.getByLabelText(f.name)).toHaveValue('Plásticos del Sur')
    expect(api.listSuppliers).not.toHaveBeenCalled()
  })

  it('returns to the table with a notification after editing', async () => {
    const user = userEvent.setup()
    vi.mocked(api.getSupplier).mockResolvedValue(supplier())
    vi.mocked(api.updateSupplier).mockResolvedValue(supplier({ name: 'Renombrado' }))
    vi.mocked(api.listSuppliers).mockResolvedValue(supplierListOf([supplier({ name: 'Renombrado' })]))
    renderAt('/proveedores/sup-1/editar')

    const name = await screen.findByLabelText(f.name)
    await user.clear(name)
    await user.type(name, 'Renombrado')
    await user.click(screen.getByRole('button', { name: es.common.save }))

    expect(await screen.findByRole('cell', { name: 'Renombrado' })).toBeInTheDocument()
    expect(screen.getByText(es.suppliers.savedNotice)).toBeInTheDocument()
    expect(api.updateSupplier).toHaveBeenCalledWith('sup-1', expect.objectContaining({ name: 'Renombrado' }))
  })

  it('shows a Spanish error and a retry when the supplier cannot be loaded', async () => {
    vi.mocked(api.getSupplier).mockRejectedValue(new ApiError(404, 'supplier_not_found'))
    renderAt('/proveedores/missing/editar')

    expect(await screen.findByRole('alert')).toHaveTextContent(es.apiErrors.supplier_not_found)
    expect(screen.getByRole('button', { name: es.common.retry })).toBeInTheDocument()
  })
})

describe('navigation', () => {
  it('links to both sections from the menu', () => {
    vi.mocked(api.listSuppliers).mockResolvedValue(supplierListOf([]))
    renderAt('/proveedores')

    const menu = screen.getByRole('navigation', { name: es.app.navigationLabel })
    expect(menu).toHaveTextContent(es.app.nav.catalog)
    expect(menu).toHaveTextContent(es.app.nav.suppliers)
  })
})
