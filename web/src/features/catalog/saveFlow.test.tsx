import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { App } from '../../App'
import { ApiError } from '../../api/client'
import * as api from '../../api/products'
import { es } from '../../i18n/es'
import { listOf, pvcProduct } from '../../test/fixtures'

vi.mock('../../api/products')

const f = es.form.fields

function renderAt(path: string) {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <App />
    </MemoryRouter>,
  )
}

async function fillValidPvc(user: ReturnType<typeof userEvent.setup>) {
  await user.type(screen.getByLabelText(f.sku), 'pvc-floral-20')
  await user.type(screen.getByLabelText(f.name), 'PVC floral 20 m')
  await user.type(screen.getByLabelText(f.widthM), '1,4')
  await user.type(screen.getByLabelText(f.nominalLengthM), '20')
  await user.type(screen.getByLabelText(f.pricePerMeter), '15,5')
  await user.type(screen.getByLabelText(f.pricePerRoll), '280')
}

beforeEach(() => {
  vi.resetAllMocks()
})

describe('saving a new product', () => {
  it('returns to the table with a confirmation, listing the new product', async () => {
    const user = userEvent.setup()
    vi.mocked(api.listProducts).mockResolvedValue(listOf([pvcProduct()]))
    vi.mocked(api.createProduct).mockResolvedValue(pvcProduct())
    renderAt('/catalogo/nuevo')

    await fillValidPvc(user)
    await user.click(screen.getByRole('button', { name: es.form.submit }))

    expect(await screen.findByText('PVC-FLORAL-20')).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: es.catalog.title })).toBeInTheDocument()
    expect(screen.getByRole('status')).toHaveTextContent(es.form.createdNotice)
    expect(screen.queryByLabelText(f.sku)).not.toBeInTheDocument()
  })

  it('uploads the chosen image right after creating the product', async () => {
    const user = userEvent.setup()
    vi.mocked(URL.createObjectURL).mockReturnValue('blob:preview')
    vi.mocked(api.listProducts).mockResolvedValue(listOf([pvcProduct()]))
    vi.mocked(api.createProduct).mockResolvedValue(pvcProduct())
    vi.mocked(api.uploadProductImage).mockResolvedValue(pvcProduct())
    renderAt('/catalogo/nuevo')

    await fillValidPvc(user)
    const photo = new File([new Uint8Array(10)], 'floral.png', { type: 'image/png' })
    await user.upload(screen.getByLabelText(es.image.fileInput), photo)
    await user.click(screen.getByRole('button', { name: es.form.submit }))

    expect(await screen.findByText('PVC-FLORAL-20')).toBeInTheDocument()
    expect(api.uploadProductImage).toHaveBeenCalledWith('pvc-1', photo)
    expect(screen.getByRole('status')).toHaveTextContent(es.form.createdNotice)
  })

  it('does not call the image endpoint when no image was chosen', async () => {
    const user = userEvent.setup()
    vi.mocked(api.listProducts).mockResolvedValue(listOf([pvcProduct()]))
    vi.mocked(api.createProduct).mockResolvedValue(pvcProduct())
    renderAt('/catalogo/nuevo')

    await fillValidPvc(user)
    await user.click(screen.getByRole('button', { name: es.form.submit }))

    await screen.findByText('PVC-FLORAL-20')
    expect(api.uploadProductImage).not.toHaveBeenCalled()
  })

  it('still goes to the table and warns when only the image upload fails', async () => {
    const user = userEvent.setup()
    vi.mocked(URL.createObjectURL).mockReturnValue('blob:preview')
    vi.mocked(api.listProducts).mockResolvedValue(listOf([pvcProduct()]))
    vi.mocked(api.createProduct).mockResolvedValue(pvcProduct())
    vi.mocked(api.uploadProductImage).mockRejectedValue(new ApiError(415, 'image_type_not_supported'))
    renderAt('/catalogo/nuevo')

    await fillValidPvc(user)
    await user.upload(
      screen.getByLabelText(es.image.fileInput),
      new File([new Uint8Array(10)], 'floral.png', { type: 'image/png' }),
    )
    await user.click(screen.getByRole('button', { name: es.form.submit }))

    expect(await screen.findByText('PVC-FLORAL-20')).toBeInTheDocument()
    const notice = screen.getByRole('alert')
    expect(notice).toHaveTextContent(es.form.createdImageFailed(es.apiErrors.image_type_not_supported))
    expect(notice).toHaveClass('toast-warning')
    expect(api.createProduct).toHaveBeenCalledTimes(1)
  })

  it('starts with an empty form the next time', async () => {
    const user = userEvent.setup()
    vi.mocked(api.listProducts).mockResolvedValue(listOf([pvcProduct()]))
    vi.mocked(api.createProduct).mockResolvedValue(pvcProduct())
    renderAt('/catalogo/nuevo')
    await fillValidPvc(user)
    await user.click(screen.getByRole('button', { name: es.form.submit }))
    await screen.findByText('PVC-FLORAL-20')

    await user.click(screen.getAllByRole('link', { name: es.catalog.newProduct })[0]!)

    expect(await screen.findByLabelText(f.sku)).toHaveValue('')
    expect(screen.getByLabelText(f.name)).toHaveValue('')
    expect(screen.getByLabelText(f.widthM)).toHaveValue('')
  })

  it('stays on the form when saving fails', async () => {
    const user = userEvent.setup()
    vi.mocked(api.createProduct).mockRejectedValue(new ApiError(409, 'sku_already_exists'))
    renderAt('/catalogo/nuevo')

    await fillValidPvc(user)
    await user.click(screen.getByRole('button', { name: es.form.submit }))

    expect(await screen.findByText(es.apiErrors.sku_already_exists)).toBeInTheDocument()
    expect(screen.getByLabelText(f.sku)).toHaveValue('pvc-floral-20')
    expect(api.listProducts).not.toHaveBeenCalled()
  })
})

describe('saving an edited product', () => {
  it('returns to the table with a confirmation', async () => {
    const user = userEvent.setup()
    vi.mocked(api.getProduct).mockResolvedValue(pvcProduct())
    vi.mocked(api.updateProduct).mockResolvedValue(pvcProduct({ name: 'Renombrado' }))
    vi.mocked(api.listProducts).mockResolvedValue(listOf([pvcProduct({ name: 'Renombrado' })]))
    renderAt('/catalogo/pvc-1/editar')

    const name = await screen.findByLabelText(f.name)
    await user.clear(name)
    await user.type(name, 'Renombrado')
    await user.click(screen.getByRole('button', { name: es.form.submit }))

    expect(await screen.findByRole('cell', { name: 'Renombrado' })).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: es.catalog.title })).toBeInTheDocument()
    expect(screen.getByRole('status')).toHaveTextContent(es.form.savedNotice)
    expect(api.updateProduct).toHaveBeenCalledWith('pvc-1', expect.objectContaining({ name: 'Renombrado' }))
  })
})
